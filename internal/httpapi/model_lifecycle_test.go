package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
)

func TestManagedHeartbeatUpdatesModelLifecycleMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-lifecycle",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	body := `{"node_id":"managed-lifecycle","status":"healthy","ollama_available":true,"models":[{"name":"llama3.1:8b","state":"warm","keep_warm":true,"tokens_per_second":42.5,"load_time_ms":1200},{"name":"qwen2.5-coder:14b","state":"loading"},{"name":"SECRET_PROMPT","state":"warm"}],"active_jobs":1}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(body))
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, rr.Body.String())
	node := server.store.Snapshot().Nodes["managed-lifecycle"]
	if !node.WarmStateSupported || !node.ManagementSupported || node.ModelInventorySource != appstate.ModelInventorySourceSubscriberReported {
		t.Fatalf("managed lifecycle support metadata = %#v", node)
	}
	if len(node.Models) != 3 || node.Models[0].Name == "SECRET_PROMPT" {
		t.Fatalf("models not sanitized/preserved as expected: %#v", node.Models)
	}
	if node.Observed["model_lifecycle_source"] != modelLifecycleSourceSubscriberReported ||
		node.Observed["model_lifecycle_mode"] != modelLifecycleModeRich ||
		benchmarkJobInt(node.Observed["warm_model_count"], 0) != 2 ||
		benchmarkJobInt(node.Observed["keep_warm_count"], 0) != 1 {
		t.Fatalf("model lifecycle observed metadata = %#v", node.Observed)
	}

	status := server.modelLifecycleStatus()
	if status.Summary["warm_models"] != 2 || status.Summary["keep_warm_models"] != 1 || len(status.Nodes) == 0 {
		t.Fatalf("model lifecycle status = %#v", status)
	}
	rendered := string(mustJSON(t, status)) + string(mustJSON(t, node.Observed))
	assertNoModelLifecycleLeak(t, rendered)

	for _, route := range []string{"/wrangler/ui/bootstrap", "/wrangler/metrics", "/wrangler/models/lifecycle"} {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, route, nil)
		switch route {
		case "/wrangler/ui/bootstrap":
			server.bootstrap(rr, req)
		case "/wrangler/metrics":
			server.metrics(rr, req)
		default:
			server.modelLifecycle(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "model_lifecycle") && route != "/wrangler/models/lifecycle" {
			t.Fatalf("%s missing model lifecycle: %s", route, rr.Body.String())
		}
		assertNoModelLifecycleLeak(t, rr.Body.String())
	}
}

func TestPassiveEndpointModelLifecycleIsInventoryOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"},{"name":"SECRET_RESPONSE"}]}`))
	}))
	defer upstream.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-add", bytes.NewBufferString(`{"display_name":"Passive Lifecycle","endpoint_url":"`+upstream.URL+`","trust_level":"lan_trusted"}`))
	server.passiveAddNode(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("passive add status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, rr.Body.String())
	var node appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &node); err != nil {
		t.Fatalf("decode passive node: %v", err)
	}
	if node.ControlLevel != appstate.ControlLevelPassive || node.WarmStateSupported || node.ManagementSupported {
		t.Fatalf("passive lifecycle support metadata = %#v", node)
	}
	if node.Observed["model_lifecycle_source"] != modelLifecycleSourceMarshalObserved ||
		node.Observed["model_lifecycle_mode"] != modelLifecycleModeInventoryOnly ||
		node.Observed["warm_state_supported"] != false {
		t.Fatalf("passive lifecycle observed metadata = %#v", node.Observed)
	}
	status := server.modelLifecycleStatus()
	if status.Summary["passive_inventory_only_nodes"] != 1 || status.Nodes[0].WarmStateSupported {
		t.Fatalf("passive lifecycle status = %#v", status)
	}
	if !strings.Contains(status.Nodes[0].Message, "warm-state") {
		t.Fatalf("passive lifecycle message should explain warm-state limitation: %#v", status.Nodes[0])
	}
	assertNoModelLifecycleLeak(t, string(mustJSON(t, status)))
}

func TestManagedModelKeepWarmActionClaimAndComplete(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_model_action_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:               "managed-model-action",
		ControlLevel:         appstate.ControlLevelManaged,
		TrustLevel:           appstate.TrustLevelLocal,
		ApprovalState:        appstate.ApprovalStateApproved,
		Approved:             true,
		Enabled:              true,
		Status:               "healthy",
		WarmStateSupported:   true,
		ManagementSupported:  true,
		ModelInventorySource: appstate.ModelInventorySourceSubscriberReported,
		Models:               []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed: map[string]interface{}{
			"heartbeat_auth_method":   "shared_secret",
			"heartbeat_auth_required": true,
			"heartbeat_state":         "fresh",
		},
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-model-action"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-model-action/model-actions/keep-warm", bytes.NewBufferString(`{"model_name":"llama3.1:8b","keep_warm":true,"prompt":"SECRET_PROMPT"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue keep-warm status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, rr.Body.String())
	var queued struct {
		Status string                 `json:"status"`
		Action map[string]interface{} `json:"action"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatalf("decode queued action: %v", err)
	}
	actionID, _ := queued.Action["action_id"].(string)
	if queued.Status != "queued" || actionID == "" || queued.Action["desired_keep_warm"] != true {
		t.Fatalf("queued action = %#v", queued)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/model-actions/claim", bytes.NewBufferString(`{"node_id":"managed-model-action"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberModelActionClaim(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("claim model action status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, rr.Body.String())
	if !strings.Contains(rr.Body.String(), "action_claimed") || !strings.Contains(rr.Body.String(), actionID) {
		t.Fatalf("claim response = %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/model-actions/status", bytes.NewBufferString(`{"node_id":"managed-model-action","action_id":"`+actionID+`","status":"completed","response":"SECRET_RESPONSE"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberModelActionStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("complete model action status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, rr.Body.String())
	node := server.store.Snapshot().Nodes["managed-model-action"]
	if len(node.Models) != 1 || !node.Models[0].KeepWarm || node.Models[0].State != "warm" {
		t.Fatalf("model keep-warm was not applied: %#v", node.Models)
	}
	actions := modelLifecycleActions(node.Observed["model_lifecycle_actions"])
	if len(actions) == 0 || actions[0]["status"] != modelLifecycleActionCompleted {
		t.Fatalf("model lifecycle actions = %#v", node.Observed["model_lifecycle_actions"])
	}
	status := server.modelLifecycleStatus()
	if status.Summary["model_lifecycle_actions"] != 1 || status.Summary["pending_model_lifecycle_actions"] != 0 || status.Summary["keep_warm_models"] != 1 {
		t.Fatalf("model lifecycle status after action = %#v", status)
	}
	assertNoModelLifecycleLeak(t, string(mustJSON(t, node.Observed))+string(mustJSON(t, status)))
}

func TestModelKeepWarmActionRejectsPassiveAndUnsupportedManagedNodes(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-model-action",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "unsupported-model-action",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
	}); err != nil {
		t.Fatalf("upsert unsupported node: %v", err)
	}

	for _, tc := range []struct {
		nodeID string
		reason string
	}{
		{nodeID: "passive-model-action", reason: "passive_no_model_management_control"},
		{nodeID: "unsupported-model-action", reason: "warm_state_not_reported"},
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/"+tc.nodeID+"/model-actions/keep-warm", bytes.NewBufferString(`{"model_name":"llama3.1:8b","keep_warm":true,"authorization":"Bearer SECRET"}`))
		server.nodeAction(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s keep-warm status = %d body = %s", tc.nodeID, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), tc.reason) {
			t.Fatalf("%s response missing reason %q: %s", tc.nodeID, tc.reason, rr.Body.String())
		}
		assertNoModelLifecycleLeak(t, rr.Body.String())
	}
}

func TestModelLifecycleActionPolicyStatusExplainsEligibility(t *testing.T) {
	server := newIsolatedTestServer(t)
	nodes := []appstate.Node{
		{
			NodeID:               "eligible-model-action",
			ControlLevel:         appstate.ControlLevelManaged,
			TrustLevel:           appstate.TrustLevelLocal,
			ApprovalState:        appstate.ApprovalStateApproved,
			Approved:             true,
			Enabled:              true,
			Status:               "healthy",
			WarmStateSupported:   true,
			ManagementSupported:  true,
			ModelInventorySource: appstate.ModelInventorySourceSubscriberReported,
			Models:               []appstate.ModelState{{Name: "llama3.1:8b", State: "warm", KeepWarm: true}},
			Observed: map[string]interface{}{
				"model_lifecycle_actions": []map[string]interface{}{
					{
						"action_id":         "model_action_safe",
						"action_type":       modelLifecycleActionKeepWarm,
						"model":             "llama3.1:8b",
						"desired_keep_warm": true,
						"status":            modelLifecycleActionQueued,
					},
				},
			},
		},
		{
			NodeID:        "passive-policy",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			ApprovalState: appstate.ApprovalStateApproved,
			Approved:      true,
			Enabled:       true,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		{
			NodeID:        "unsupported-policy",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			ApprovalState: appstate.ApprovalStateApproved,
			Approved:      true,
			Enabled:       true,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		{
			NodeID:               "empty-inventory-policy",
			ControlLevel:         appstate.ControlLevelManaged,
			TrustLevel:           appstate.TrustLevelLocal,
			ApprovalState:        appstate.ApprovalStateApproved,
			Approved:             true,
			Enabled:              true,
			Status:               "healthy",
			WarmStateSupported:   true,
			ManagementSupported:  true,
			ModelInventorySource: appstate.ModelInventorySourceSubscriberReported,
		},
	}
	for _, node := range nodes {
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("upsert %s: %v", node.NodeID, err)
		}
	}

	status := server.modelLifecycleActionPolicyStatus()
	if status.Window != "current_model_lifecycle_action_policy" ||
		status.Summary["eligible_nodes"] != 1 ||
		status.Summary["blocked_nodes"] < 3 ||
		status.Summary["pending_model_lifecycle_actions"] != 1 {
		t.Fatalf("model lifecycle action policy status = %#v", status)
	}
	policies := map[string]ModelLifecycleActionPolicyNode{}
	for _, node := range status.Nodes {
		policies[node.NodeID] = node
	}
	if !policies["eligible-model-action"].Eligible ||
		!containsString(policies["eligible-model-action"].SupportedActions, modelLifecycleActionKeepWarm) ||
		!containsString(policies["eligible-model-action"].ReasonCodes, "managed_subscriber_model_action_allowed") {
		t.Fatalf("eligible policy = %#v", policies["eligible-model-action"])
	}
	if policies["passive-policy"].Eligible ||
		!containsString(policies["passive-policy"].ReasonCodes, "passive_no_model_management_control") ||
		policies["passive-policy"].Mode != modelLifecycleModeInventoryOnly {
		t.Fatalf("passive policy = %#v", policies["passive-policy"])
	}
	if policies["unsupported-policy"].Eligible ||
		!containsString(policies["unsupported-policy"].ReasonCodes, "warm_state_not_reported") {
		t.Fatalf("unsupported policy = %#v", policies["unsupported-policy"])
	}
	if policies["empty-inventory-policy"].Eligible ||
		!containsString(policies["empty-inventory-policy"].ReasonCodes, "model_inventory_empty") {
		t.Fatalf("empty inventory policy = %#v", policies["empty-inventory-policy"])
	}

	for _, route := range []string{"/wrangler/models/lifecycle/action-policies", "/wrangler/ui/bootstrap", "/wrangler/metrics"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		switch route {
		case "/wrangler/ui/bootstrap":
			server.bootstrap(rr, req)
		case "/wrangler/metrics":
			server.metrics(rr, req)
		default:
			server.modelLifecycleActionPolicies(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "model_lifecycle_actions") && route != "/wrangler/models/lifecycle/action-policies" {
			t.Fatalf("%s missing model lifecycle action policy: %s", route, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "current_model_lifecycle_action_policy") {
			t.Fatalf("%s missing policy window: %s", route, rr.Body.String())
		}
		assertNoModelLifecycleLeak(t, rr.Body.String())
	}
	assertNoModelLifecycleLeak(t, string(mustJSON(t, status)))
}

func TestModelLifecycleActionHistoryFiltersAndSanitizesMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	now := time.Now().UTC().Truncate(time.Second)
	actions := []map[string]interface{}{
		{
			"action_id":         "model_action_failed",
			"action_type":       modelLifecycleActionKeepWarm,
			"policy":            modelLifecycleActionPolicyManaged,
			"model":             "llama3.1:8b",
			"desired_keep_warm": true,
			"status":            modelLifecycleActionFailed,
			"requested_at":      now.Add(-10 * time.Minute),
			"claimed_at":        now.Add(-9 * time.Minute),
			"updated_at":        now,
			"failed_at":         now,
			"error_code":        "redacted_error_code",
		},
		{
			"action_id":         "model_action_completed",
			"action_type":       modelLifecycleActionKeepWarm,
			"policy":            modelLifecycleActionPolicyManaged,
			"model":             "llama3.1:8b",
			"desired_keep_warm": false,
			"status":            modelLifecycleActionCompleted,
			"requested_at":      now.Add(-8 * time.Minute),
			"claimed_at":        now.Add(-7 * time.Minute),
			"updated_at":        now.Add(-time.Minute),
			"completed_at":      now.Add(-time.Minute),
		},
		{
			"action_id":    "model_action_running",
			"action_type":  modelLifecycleActionKeepWarm,
			"policy":       modelLifecycleActionPolicyManaged,
			"model":        "llama3.1:8b",
			"status":       modelLifecycleActionRunning,
			"requested_at": now.Add(-6 * time.Minute),
			"claimed_at":   now.Add(-5 * time.Minute),
			"updated_at":   now.Add(-2 * time.Minute),
		},
		{
			"action_id":    "model_action_queued",
			"action_type":  modelLifecycleActionKeepWarm,
			"policy":       modelLifecycleActionPolicyManaged,
			"model":        "llama3.1:8b",
			"status":       modelLifecycleActionQueued,
			"requested_at": now.Add(-3 * time.Minute),
			"updated_at":   now.Add(-3 * time.Minute),
		},
		{
			"action_id":    "model_action_cancelled",
			"action_type":  modelLifecycleActionKeepWarm,
			"policy":       modelLifecycleActionPolicyManaged,
			"model":        "llama3.1:8b",
			"status":       modelLifecycleActionCancelled,
			"requested_at": now.Add(-5 * time.Minute),
			"updated_at":   now.Add(-4 * time.Minute),
			"completed_at": now.Add(-4 * time.Minute),
		},
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:               "managed-history",
		ControlLevel:         appstate.ControlLevelManaged,
		TrustLevel:           appstate.TrustLevelLocal,
		ApprovalState:        appstate.ApprovalStateApproved,
		Approved:             true,
		Enabled:              true,
		Status:               "healthy",
		WarmStateSupported:   true,
		ManagementSupported:  true,
		ModelInventorySource: appstate.ModelInventorySourceSubscriberReported,
		Models:               []appstate.ModelState{{Name: "llama3.1:8b", State: "warm"}},
		Observed:             map[string]interface{}{"model_lifecycle_actions": actions},
	}); err != nil {
		t.Fatalf("upsert managed history node: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-history",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed: map[string]interface{}{"model_lifecycle_actions": []map[string]interface{}{{
			"action_id":  "passive_action_must_not_surface",
			"status":     modelLifecycleActionCompleted,
			"updated_at": now.Add(time.Minute),
		}}},
	}); err != nil {
		t.Fatalf("upsert passive history node: %v", err)
	}

	history := server.modelLifecycleActionHistoryStatus(ModelLifecycleActionHistoryFilter{Limit: 20})
	if history.Window != "recent_model_lifecycle_actions" || history.Retention != "bounded_node_state_recent_actions" || history.MaxActionsPerNode != maxModelLifecycleActions {
		t.Fatalf("history metadata = %#v", history)
	}
	if history.TotalMatches != 5 || history.Count != 5 || history.Summary[modelLifecycleActionFailed] != 1 || history.Summary[modelLifecycleActionCompleted] != 1 {
		t.Fatalf("history summary = %#v", history)
	}
	if history.Actions[0].ActionID != "model_action_failed" || history.Actions[0].ErrorCode != "redacted_error_code" {
		t.Fatalf("newest/sanitized history item = %#v", history.Actions[0])
	}
	legacy := modelLifecycleActionHistoryItem(server.store.Snapshot().Nodes["managed-history"], map[string]interface{}{
		"action_id":   "legacy_unsafe_action",
		"action_type": modelLifecycleActionKeepWarm,
		"status":      modelLifecycleActionFailed,
		"error_code":  "SECRET_PROMPT Authorization: Bearer SECRET",
	})
	if legacy.ErrorCode != "redacted_error_code" {
		t.Fatalf("legacy unsafe error code was not redacted: %#v", legacy)
	}
	if strings.Contains(string(mustJSON(t, history)), "passive_action_must_not_surface") {
		t.Fatalf("passive action surfaced in Managed Node history: %#v", history)
	}

	filtered := server.modelLifecycleActionHistoryStatus(ModelLifecycleActionHistoryFilter{Status: modelLifecycleActionCompleted, NodeID: "managed-history", Limit: 1})
	if filtered.TotalMatches != 1 || filtered.Count != 1 || filtered.Actions[0].Status != modelLifecycleActionCompleted || filtered.Filters.NodeID != "managed-history" {
		t.Fatalf("filtered history = %#v", filtered)
	}

	for _, route := range []string{
		"/wrangler/models/lifecycle/action-history?status=failed&node_id=managed-history&limit=1",
		"/wrangler/ui/bootstrap",
		"/wrangler/metrics",
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		switch route {
		case "/wrangler/ui/bootstrap":
			server.bootstrap(rr, req)
		case "/wrangler/metrics":
			server.metrics(rr, req)
		default:
			server.modelLifecycleActionHistory(rr, req)
		}
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "recent_model_lifecycle_actions") {
			t.Fatalf("%s missing action history: status=%d body=%s", route, rr.Code, rr.Body.String())
		}
		assertNoModelLifecycleLeak(t, rr.Body.String())
	}

	rendered := string(mustJSON(t, history)) + string(mustJSON(t, server.buildSupportBundle()))
	assertNoModelLifecycleLeak(t, rendered)
}

func TestModelLifecycleActionHistoryRejectsInvalidFiltersAndBoundsLimit(t *testing.T) {
	server := newIsolatedTestServer(t)
	for _, path := range []string{
		"/wrangler/models/lifecycle/action-history?status=SECRET_PROMPT",
		"/wrangler/models/lifecycle/action-history?limit=0",
		"/wrangler/models/lifecycle/action-history?limit=51",
		"/wrangler/models/lifecycle/action-history?limit=not-a-number",
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		server.modelLifecycleActionHistory(rr, req)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid model lifecycle action history filter") {
			t.Fatalf("invalid filter %s status=%d body=%s", path, rr.Code, rr.Body.String())
		}
		assertNoModelLifecycleLeak(t, rr.Body.String())
	}

	if got := safeModelLifecycleActionErrorCode("temporary_runner_error"); got != "temporary_runner_error" {
		t.Fatalf("safe error code changed: %q", got)
	}
	for _, unsafe := range []string{"SECRET_RESPONSE", "Authorization: Bearer SECRET", "/tmp/Secret Project", strings.Repeat("a", 65)} {
		if got := safeModelLifecycleActionErrorCode(unsafe); got != "redacted_error_code" && got != "invalid_error_code" {
			t.Fatalf("unsafe error code %q normalized to %q", unsafe, got)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertNoModelLifecycleLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"SECRET_PROMPT", "SECRET_RESPONSE", "Authorization", "Bearer SECRET", "lw_hb_", "lw_enroll_", "lw_admin_", "lw_client_", "sk-"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("model lifecycle leaked %q: %s", forbidden, body)
		}
	}
}
