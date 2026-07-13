package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

func TestMarshalProxyEmitsMetadataOnlyRoutingPolicyReasons(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-consensus"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"llama3.1:8b"},
		ExecutionMode:   "consensus",
		MinParticipants: 2,
	}
	server := newIsolatedTestServerWithConfig(t, cfg)
	nodes := []appstate.Node{
		{
			NodeID:        "managed-local",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		{
			NodeID:        "passive-trusted",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}
	for _, node := range nodes {
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("upsert node: %v", err)
		}
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-consensus","messages":[{"role":"user","content":"SECRET_PROMPT"}]}`))
	req.Header.Set("Content-Type", "application/json")
	server.marshalProxy("/v1/chat/completions", "openai")(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("marshal status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("response leaked prompt: %s", rr.Body.String())
	}

	audit := server.store.Snapshot().Audit
	var found bool
	for _, event := range audit {
		if event.Type != "error" || event.Fields["error_class"] != "no_eligible_node" {
			continue
		}
		found = true
		rendered := stringifyMap(event.Fields)
		if !strings.Contains(rendered, "consensus_min_participants_unmet") {
			t.Fatalf("routing error missing consensus reason: %s", rendered)
		}
		if !strings.Contains(rendered, "passive_consensus_excluded") {
			t.Fatalf("routing error missing passive exclusion: %s", rendered)
		}
		if strings.Contains(rendered, "SECRET_PROMPT") || strings.Contains(rendered, "Authorization") {
			t.Fatalf("routing metadata leaked forbidden marker: %s", rendered)
		}
	}
	if !found {
		t.Fatalf("no routing error audit event found: %#v", audit)
	}
}

func TestSummarizeRoutingPolicyExplainsControlTrustApprovalAndHeartbeatWarnings(t *testing.T) {
	now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * time.Minute)
	status := summarizeRoutingPolicy(map[string]appstate.Node{
		"pending-managed": {
			NodeID:        "pending-managed",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      false,
			ApprovalState: appstate.ApprovalStatePending,
			Status:        "healthy",
		},
		"passive-trusted": {
			NodeID:        "passive-trusted",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
		},
		"managed-unverified": {
			NodeID:        "managed-unverified",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLANUnverified,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
		},
		"external-managed": {
			NodeID:        "external-managed",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelExternal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
		},
		"missing-heartbeat": {
			NodeID:         "missing-heartbeat",
			ControlLevel:   appstate.ControlLevelManaged,
			TrustLevel:     appstate.TrustLevelLocal,
			Enabled:        true,
			Approved:       true,
			ApprovalState:  appstate.ApprovalStateApproved,
			Status:         "healthy",
			LastReportedAt: nil,
			Observed:       map[string]interface{}{"heartbeat_required": true},
		},
		"stale-heartbeat": {
			NodeID:         "stale-heartbeat",
			ControlLevel:   appstate.ControlLevelManaged,
			TrustLevel:     appstate.TrustLevelLocal,
			Enabled:        true,
			Approved:       true,
			ApprovalState:  appstate.ApprovalStateApproved,
			Status:         "healthy",
			LastReportedAt: &stale,
			Observed:       map[string]interface{}{"heartbeat_required": true},
		},
	}, now)

	for _, code := range []string{
		"node_not_approved",
		"passive_consensus_excluded",
		"trust_lan_unverified_deprioritized",
		"trust_lan_unverified_consensus_excluded",
		"trust_external_excluded",
		"heartbeat_missing",
		"heartbeat_stale",
	} {
		if !routingStatusHasCode(status, code) {
			t.Fatalf("missing routing policy warning %q in %#v", code, status.Warnings)
		}
		if status.Summary[code] == 0 {
			t.Fatalf("missing summary count for %q in %#v", code, status.Summary)
		}
	}
	rendered := stringifyMap(map[string]interface{}{"status": status})
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_admin_", "lw_client_", "lw_enroll_", "token_hash"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("routing policy status leaked forbidden marker %q: %s", forbidden, rendered)
		}
	}
}

func TestBootstrapAndMetricsIncludeRoutingPolicyStatus(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "pending-managed",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		Enabled:       true,
		Approved:      false,
		ApprovalState: appstate.ApprovalStatePending,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert pending node: %v", err)
	}

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
	assertRoutingPolicyStatusBody(t, body["routing_policy_status"], "node_not_approved")
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") || strings.Contains(rr.Body.String(), "token_hash") {
		t.Fatalf("bootstrap routing policy status leaked forbidden marker: %s", rr.Body.String())
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
	assertRoutingPolicyStatusBody(t, body["routing_policy_status"], "node_not_approved")
}

func routingStatusHasCode(status RoutingPolicyStatus, code string) bool {
	for _, warning := range status.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func assertRoutingPolicyStatusBody(t *testing.T, raw interface{}, code string) {
	t.Helper()
	status, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("routing_policy_status missing or wrong type: %#v", raw)
	}
	if status["window"] != "current_node_metadata" {
		t.Fatalf("routing policy window = %#v", status["window"])
	}
	warnings, ok := status["warnings"].([]interface{})
	if !ok || len(warnings) == 0 {
		t.Fatalf("routing policy warnings missing: %#v", status)
	}
	for _, warning := range warnings {
		item, ok := warning.(map[string]interface{})
		if !ok {
			continue
		}
		if item["code"] == code && item["message"] != "" && item["node_id"] != "" {
			return
		}
	}
	t.Fatalf("routing policy status missing code %q in %#v", code, status)
}

func stringifyMap(fields map[string]interface{}) string {
	data, _ := json.Marshal(fields)
	return string(data)
}
