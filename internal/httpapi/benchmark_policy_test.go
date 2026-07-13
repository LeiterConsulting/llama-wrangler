package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
)

func TestSummarizeBenchmarkPolicySeparatesManagedAndPassiveNodes(t *testing.T) {
	now := time.Now().UTC()
	status := summarizeBenchmarkPolicy(map[string]appstate.Node{
		"managed-approved": {
			NodeID:          "managed-approved",
			ControlLevel:    appstate.ControlLevelManaged,
			TrustLevel:      appstate.TrustLevelLocal,
			Enabled:         true,
			Approved:        true,
			ApprovalState:   appstate.ApprovalStateApproved,
			Status:          "healthy",
			BenchmarkSource: appstate.BenchmarkSourceSubscriberReported,
			Observed: map[string]interface{}{
				"benchmark_results": []map[string]interface{}{{
					"source":            appstate.BenchmarkSourceSubscriberReported,
					"status":            "completed",
					"completed_at":      now,
					"tokens_per_second": 42.0,
				}},
			},
		},
		"managed-pending": {
			NodeID:        "managed-pending",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      false,
			ApprovalState: appstate.ApprovalStatePending,
			Status:        "healthy",
		},
		"passive-approved": {
			NodeID:          "passive-approved",
			ControlLevel:    appstate.ControlLevelPassive,
			TrustLevel:      appstate.TrustLevelLANTrusted,
			Enabled:         true,
			Approved:        true,
			ApprovalState:   appstate.ApprovalStateApproved,
			Status:          "healthy",
			BenchmarkSource: appstate.BenchmarkSourceNone,
		},
	})

	if status.Window != "current_node_metadata" {
		t.Fatalf("window = %q", status.Window)
	}
	if status.Summary["eligible"] != 1 || status.Summary["ineligible"] != 2 {
		t.Fatalf("summary = %#v", status.Summary)
	}
	if status.Summary["subscriber_reported"] != 2 || status.Summary["marshal_observed_probe_only"] != 1 {
		t.Fatalf("mode summary = %#v", status.Summary)
	}
	if status.Summary["placement_eligible"] != 1 || status.Summary["placement_limited"] != 2 {
		t.Fatalf("placement summary = %#v", status.Summary)
	}
	assertBenchmarkPolicyNode(t, status, "managed-approved", true, "subscriber_reported", appstate.BenchmarkSourceSubscriberReported, "managed_subscriber_benchmark_allowed", "benchmark_placement_applied")
	assertBenchmarkPolicyNode(t, status, "managed-pending", false, "subscriber_reported", appstate.BenchmarkSourceSubscriberReported, "node_not_approved", "benchmark_placement_summary_missing")
	assertBenchmarkPolicyNode(t, status, "passive-approved", false, "marshal_observed_probe_only", appstate.BenchmarkSourceNone, "passive_no_local_benchmark_control", "benchmark_placement_passive_probe_ignored")

	rendered, _ := json.Marshal(status)
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_admin_", "lw_client_", "lw_enroll_", "token_hash"} {
		if strings.Contains(string(rendered), forbidden) {
			t.Fatalf("benchmark policy status leaked forbidden marker %q: %s", forbidden, string(rendered))
		}
	}
}

func TestSummarizeBenchmarkPolicyMarksStalePlacementLimited(t *testing.T) {
	stale := time.Now().UTC().Add(-(24*time.Hour + time.Minute))
	status := summarizeBenchmarkPolicy(map[string]appstate.Node{
		"managed-stale": {
			NodeID:          "managed-stale",
			ControlLevel:    appstate.ControlLevelManaged,
			TrustLevel:      appstate.TrustLevelLANTrusted,
			Enabled:         true,
			Approved:        true,
			ApprovalState:   appstate.ApprovalStateApproved,
			Status:          "healthy",
			BenchmarkSource: appstate.BenchmarkSourceSubscriberReported,
			Observed: map[string]interface{}{
				"benchmark_results": []map[string]interface{}{{
					"source":            appstate.BenchmarkSourceSubscriberReported,
					"status":            "completed",
					"completed_at":      stale,
					"tokens_per_second": 100.0,
				}},
			},
		},
	})

	if status.Summary["placement_eligible"] != 0 || status.Summary["benchmark_placement_summary_stale"] != 1 {
		t.Fatalf("stale placement summary = %#v", status.Summary)
	}
	assertBenchmarkPolicyNode(t, status, "managed-stale", true, "subscriber_reported", appstate.BenchmarkSourceSubscriberReported, "managed_subscriber_benchmark_allowed", "benchmark_placement_summary_stale")
}

func TestBenchmarkActionEnforcesManagedOnlyPolicy(t *testing.T) {
	server := newIsolatedTestServer(t)
	for _, node := range []appstate.Node{
		{
			NodeID:        "managed-approved",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
		},
		{
			NodeID:          "passive-approved",
			ControlLevel:    appstate.ControlLevelPassive,
			TrustLevel:      appstate.TrustLevelLANTrusted,
			Enabled:         true,
			Approved:        true,
			ApprovalState:   appstate.ApprovalStateApproved,
			Status:          "healthy",
			BenchmarkSource: appstate.BenchmarkSourceNone,
		},
	} {
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("upsert node %q: %v", node.NodeID, err)
		}
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-approved/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("managed benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("managed benchmark response leaked forbidden marker: %s", rr.Body.String())
	}
	managed := server.store.Snapshot().Nodes["managed-approved"]
	if managed.BenchmarkSource != appstate.BenchmarkSourceSubscriberReported {
		t.Fatalf("managed benchmark source = %q", managed.BenchmarkSource)
	}
	if managed.Observed["benchmark_status"] != "queued" || managed.Observed["benchmark_policy"] != "managed_subscriber_reported" {
		t.Fatalf("managed benchmark metadata = %#v", managed.Observed)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-approved/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("passive benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "passive_no_local_benchmark_control") {
		t.Fatalf("passive benchmark response missing policy reason: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("passive benchmark response leaked forbidden marker: %s", rr.Body.String())
	}
	passive := server.store.Snapshot().Nodes["passive-approved"]
	if passive.BenchmarkSource != appstate.BenchmarkSourceNone {
		t.Fatalf("passive benchmark source changed = %q", passive.BenchmarkSource)
	}
	if passive.Observed["benchmark_status"] == "queued" {
		t.Fatalf("passive benchmark was queued: %#v", passive.Observed)
	}
}

func TestBootstrapAndMetricsIncludeBenchmarkPolicyStatus(t *testing.T) {
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

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/wrangler/ui/bootstrap", handler: server.bootstrap},
		{name: "metrics", method: http.MethodGet, path: "/wrangler/metrics", handler: server.metrics},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			tc.handler(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s status = %d body = %s", tc.name, rr.Code, rr.Body.String())
			}
			var body map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}
			assertBenchmarkPolicyStatusBody(t, body["benchmark_policy_status"], "node_not_approved")
			if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") || strings.Contains(rr.Body.String(), "token_hash") {
				t.Fatalf("%s benchmark policy status leaked forbidden marker: %s", tc.name, rr.Body.String())
			}
		})
	}
}

func assertBenchmarkPolicyNode(t *testing.T, status BenchmarkPolicyStatus, nodeID string, eligible bool, mode string, source string, reason string, placementReason string) {
	t.Helper()
	for _, node := range status.Nodes {
		if node.NodeID != nodeID {
			continue
		}
		if node.Eligible != eligible || node.Mode != mode || node.BenchmarkSource != source || !benchmarkPolicyHasReason(node, reason) || !benchmarkPolicyHasPlacementReason(node, placementReason) {
			t.Fatalf("benchmark policy node %q = %#v", nodeID, node)
		}
		return
	}
	t.Fatalf("benchmark policy node %q missing in %#v", nodeID, status.Nodes)
}

func assertBenchmarkPolicyStatusBody(t *testing.T, raw interface{}, reason string) {
	t.Helper()
	status, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("benchmark_policy_status missing or wrong type: %#v", raw)
	}
	if status["window"] != "current_node_metadata" {
		t.Fatalf("benchmark policy window = %#v", status["window"])
	}
	nodes, ok := status["nodes"].([]interface{})
	if !ok || len(nodes) == 0 {
		t.Fatalf("benchmark policy nodes missing: %#v", status)
	}
	for _, rawNode := range nodes {
		node, ok := rawNode.(map[string]interface{})
		if !ok {
			continue
		}
		reasons, _ := node["reason_codes"].([]interface{})
		for _, rawReason := range reasons {
			if rawReason == reason && node["message"] != "" && node["node_id"] != "" {
				return
			}
		}
	}
	t.Fatalf("benchmark policy status missing reason %q in %#v", reason, status)
}

func benchmarkPolicyHasReason(node BenchmarkPolicyNode, reason string) bool {
	for _, code := range node.ReasonCodes {
		if code == reason {
			return true
		}
	}
	return false
}

func benchmarkPolicyHasPlacementReason(node BenchmarkPolicyNode, reason string) bool {
	for _, code := range node.PlacementReasonCodes {
		if code == reason {
			return true
		}
	}
	return false
}
