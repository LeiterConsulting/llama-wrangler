package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
)

func TestSummarizeOperationsCountsRetryPartialAndCancellationEvents(t *testing.T) {
	base := time.Date(2026, 7, 2, 14, 0, 0, 0, time.UTC)
	stats := summarizeOperations([]appstate.AuditEvent{
		{Timestamp: base.Add(1 * time.Minute), Type: "upstream_retry", Fields: map[string]interface{}{"retry_phase": "before_first_token"}},
		{Timestamp: base.Add(2 * time.Minute), Type: "response_partial", Fields: map[string]interface{}{"retry_phase": "after_partial_output"}},
		{Timestamp: base.Add(3 * time.Minute), Type: "request_cancelled", Fields: map[string]interface{}{"reason": "client_cancelled_before_first_token"}},
		{Timestamp: base.Add(4 * time.Minute), Type: "request_cancelled", Fields: map[string]interface{}{"reason": "client_cancelled_after_partial_output"}},
		{Timestamp: base.Add(5 * time.Minute), Type: "request_cancelled", Fields: map[string]interface{}{"reason": "client_disconnect_before_queue"}},
		{Timestamp: base.Add(6 * time.Minute), Type: "response"},
	})

	if stats.Window != "recent_audit_events" || stats.AuditEvents != 6 {
		t.Fatalf("stats window/audit count = %+v", stats)
	}
	if stats.Retries.Total != 1 || stats.Retries.BeforeFirstToken != 1 {
		t.Fatalf("retry stats = %+v", stats.Retries)
	}
	if stats.Partials.Total != 1 || stats.Partials.AfterPartial != 1 {
		t.Fatalf("partial stats = %+v", stats.Partials)
	}
	if stats.Cancellations.Total != 3 || stats.Cancellations.BeforeFirstToken != 1 || stats.Cancellations.AfterPartialOutput != 1 || stats.Cancellations.BeforeQueue != 1 {
		t.Fatalf("cancellation stats = %+v", stats.Cancellations)
	}
	if stats.Cancellations.LastAt == nil || !stats.Cancellations.LastAt.Equal(base.Add(5*time.Minute)) {
		t.Fatalf("cancellation last_at = %v", stats.Cancellations.LastAt)
	}
}

func TestBootstrapAndMetricsIncludeOperationStats(t *testing.T) {
	server := newIsolatedTestServer(t)
	server.store.AddAudit(appstate.AuditEvent{Type: "upstream_retry", RequestID: "req_retry", Fields: map[string]interface{}{"retry_phase": "before_first_token", "reason": "upstream_status_5xx"}})
	server.store.AddAudit(appstate.AuditEvent{Type: "response_partial", RequestID: "req_partial", Fields: map[string]interface{}{"retry_phase": "after_partial_output", "bytes_written": 12}})
	server.store.AddAudit(appstate.AuditEvent{Type: "request_cancelled", RequestID: "req_cancel", Fields: map[string]interface{}{"reason": "client_cancelled_after_partial_output", "partial_output": true}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	assertOperationStatsBody(t, body["operation_stats"])
	raw := rr.Body.String()
	for _, forbidden := range []string{"prompt", "messages", "authorization", "api_key", "lw_client_", "lw_admin_"} {
		if jsonContainsKey(raw, forbidden) {
			t.Fatalf("operation stats response should not expose forbidden marker %q: %s", forbidden, raw)
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/metrics", nil)
	server.metrics(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body = %s", rr.Code, rr.Body.String())
	}
	body = map[string]interface{}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	assertOperationStatsBody(t, body["operation_stats"])
}

func assertOperationStatsBody(t *testing.T, raw interface{}) {
	t.Helper()
	stats, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("operation_stats missing or wrong type: %#v", raw)
	}
	if stats["window"] != "recent_audit_events" || stats["audit_events"].(float64) < 3 {
		t.Fatalf("operation_stats window/audit_events = %#v", stats)
	}
	retries := stats["retries"].(map[string]interface{})
	if retries["total"].(float64) != 1 || retries["before_first_token"].(float64) != 1 {
		t.Fatalf("retry stats = %#v", retries)
	}
	partials := stats["partials"].(map[string]interface{})
	if partials["total"].(float64) != 1 || partials["after_partial"].(float64) != 1 {
		t.Fatalf("partial stats = %#v", partials)
	}
	cancellations := stats["cancellations"].(map[string]interface{})
	if cancellations["total"].(float64) != 1 || cancellations["after_partial_output"].(float64) != 1 {
		t.Fatalf("cancellation stats = %#v", cancellations)
	}
}

func jsonContainsKey(raw, key string) bool {
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return false
	}
	return containsKey(value, key)
}

func containsKey(value interface{}, key string) bool {
	switch v := value.(type) {
	case map[string]interface{}:
		for candidate, item := range v {
			if candidate == key || containsKey(item, key) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if containsKey(item, key) {
				return true
			}
		}
	}
	return false
}
