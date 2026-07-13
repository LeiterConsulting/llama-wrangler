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

func TestBootstrapIncludesNodeControlTrustMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:       "passive-endpoint",
		DisplayName:  "Passive Endpoint",
		Role:         "passive",
		ControlLevel: appstate.ControlLevelPassive,
		URL:          "http://studio.local:11434",
		Status:       "unknown",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("upsert passive endpoint: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "lw_client_SECRET") {
		t.Fatalf("bootstrap leaked forbidden marker: %s", rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if body["schema_version"].(float64) != appstate.CurrentSchemaVersion {
		t.Fatalf("schema_version = %#v", body["schema_version"])
	}
	nodes := body["nodes"].(map[string]interface{})
	passive := nodes["passive-endpoint"].(map[string]interface{})
	if passive["control_level"] != appstate.ControlLevelPassive {
		t.Fatalf("control_level = %#v", passive["control_level"])
	}
	if passive["trust_level"] != appstate.TrustLevelLANUnverified {
		t.Fatalf("trust_level = %#v", passive["trust_level"])
	}
	if passive["capability_source"] != appstate.CapabilitySourceMarshalObserved {
		t.Fatalf("capability_source = %#v", passive["capability_source"])
	}
	if passive["approval_state"] != appstate.ApprovalStatePending {
		t.Fatalf("approval_state = %#v", passive["approval_state"])
	}
	if passive["management_supported"].(bool) || passive["warm_state_supported"].(bool) {
		t.Fatalf("passive unsupported controls were enabled: %#v", passive)
	}
	if passive["last_observed_at"] == "" {
		t.Fatalf("last_observed_at missing: %#v", passive)
	}
	if _, ok := passive["last_reported_at"]; ok {
		t.Fatalf("passive endpoint should not claim subscriber report freshness: %#v", passive)
	}
}

func TestPassiveAddNodeValidatesTagsAndStoresLimitedMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("passive validation forwarded authorization header")
		}
		if r.ContentLength > 0 {
			t.Fatalf("passive validation sent a request body")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"},{"name":"qwen2.5-coder:14b"}]}`))
	}))
	defer upstream.Close()

	body := `{"display_name":"Studio Endpoint","endpoint_url":"` + upstream.URL + `","trust_level":"lan_trusted"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-add", bytes.NewBufferString(body))
	server.passiveAddNode(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("passive add status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("passive add leaked forbidden marker: %s", rr.Body.String())
	}
	var node appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &node); err != nil {
		t.Fatalf("decode passive node: %v", err)
	}
	if node.ControlLevel != appstate.ControlLevelPassive || node.TrustLevel != appstate.TrustLevelLANTrusted {
		t.Fatalf("control/trust metadata = %#v", node)
	}
	if node.CapabilitySource != appstate.CapabilitySourceMarshalObserved || node.HealthSource != appstate.HealthSourceMarshalObserved {
		t.Fatalf("source metadata = %#v", node)
	}
	if node.ApprovalState != appstate.ApprovalStatePending || node.Approved {
		t.Fatalf("approval metadata = %#v", node)
	}
	if node.ManagementSupported || node.WarmStateSupported || node.TelemetryLevel != appstate.TelemetryLevelMarshalObservedMetadata {
		t.Fatalf("passive support metadata = %#v", node)
	}
	if len(node.Models) != 2 || node.Models[0].Name != "llama3.1:8b" {
		t.Fatalf("models = %#v", node.Models)
	}
	if node.LastObservedAt == nil || node.LastReportedAt != nil {
		t.Fatalf("freshness metadata = %#v", node)
	}
	persisted := server.store.Snapshot().Nodes[node.NodeID]
	if persisted.ControlLevel != appstate.ControlLevelPassive || persisted.Observed["validation"] != "api_tags" {
		t.Fatalf("persisted passive node = %#v", persisted)
	}
}

func TestPassiveAddNodeRejectsUnsafeOrUntrustedInput(t *testing.T) {
	server := newIsolatedTestServer(t)
	for name, body := range map[string]string{
		"missing trust":      `{"endpoint_url":"http://localhost:11434"}`,
		"credentials in URL": `{"endpoint_url":"http://user:pass@localhost:11434","trust_level":"local"}`,
		"unsupported scheme": `{"endpoint_url":"file:///tmp/ollama.sock","trust_level":"local"}`,
		"unsupported trust":  `{"endpoint_url":"http://localhost:11434","trust_level":"trusted-ish"}`,
		"relative URL":       `{"endpoint_url":"localhost:11434","trust_level":"local"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-add", bytes.NewBufferString(body))
			server.passiveAddNode(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManualSubscriberAddCreatesPendingManagedNode(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriber/capabilities" {
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("manual capability probe forwarded authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"node_id":"managed-worker","display_name":"Managed Worker","role":"subscriber","status":"healthy","enabled":true,"approved":true}`))
	}))
	defer upstream.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/manual-add", bytes.NewBufferString(`{"node_id":"managed-worker","url":"`+upstream.URL+`"}`))
	server.manualAddNode(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("manual add status = %d body = %s", rr.Code, rr.Body.String())
	}
	var node appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &node); err != nil {
		t.Fatalf("decode managed node: %v", err)
	}
	if node.ControlLevel != appstate.ControlLevelManaged || node.CapabilitySource != appstate.CapabilitySourceSubscriberReported {
		t.Fatalf("managed source metadata = %#v", node)
	}
	if node.ApprovalState != appstate.ApprovalStatePending || node.Approved {
		t.Fatalf("managed approval metadata = %#v", node)
	}
	if node.Observed["manual_add"] != true || node.Observed["manual_add_pending_approval"] != true {
		t.Fatalf("manual add metadata missing: %#v", node.Observed)
	}
	status := summarizeRoutingPolicy(server.store.Snapshot().Nodes, time.Now().UTC())
	if !routingStatusHasCode(status, "node_not_approved") {
		t.Fatalf("pending manual node missing routing warning: %#v", status)
	}
}

func TestManualSubscriberAddRejectsUnsafeURL(t *testing.T) {
	server := newIsolatedTestServer(t)
	for name, body := range map[string]string{
		"missing URL":        `{"node_id":"worker"}`,
		"credentials in URL": `{"node_id":"worker","url":"http://user:pass@localhost:11436"}`,
		"unsupported scheme": `{"node_id":"worker","url":"file:///tmp/subscriber.sock"}`,
		"relative URL":       `{"node_id":"worker","url":"localhost:11436"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/manual-add", bytes.NewBufferString(body))
			server.manualAddNode(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestNodeApproveAndRevokeActionsUpdateRoutingEligibilityMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "pending-passive",
		DisplayName:   "Pending Passive",
		Role:          "passive",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANUnverified,
		URL:           "http://studio.local:11434",
		Status:        "healthy",
		Enabled:       true,
		Approved:      false,
		ApprovalState: appstate.ApprovalStatePending,
	}); err != nil {
		t.Fatalf("upsert pending passive: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/pending-passive/approve", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("approve response leaked forbidden marker: %s", rr.Body.String())
	}
	var approved appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved node: %v", err)
	}
	if approved.ApprovalState != appstate.ApprovalStateApproved || !approved.Approved || !approved.Enabled {
		t.Fatalf("approved node metadata = %#v", approved)
	}
	if approved.ControlLevel != appstate.ControlLevelPassive || approved.TrustLevel != appstate.TrustLevelLANUnverified {
		t.Fatalf("approval changed passive control/trust metadata = %#v", approved)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/pending-passive/revoke", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke status = %d body = %s", rr.Code, rr.Body.String())
	}
	var revoked appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &revoked); err != nil {
		t.Fatalf("decode revoked node: %v", err)
	}
	if revoked.ApprovalState != appstate.ApprovalStateRevoked || revoked.Approved || revoked.Enabled || revoked.Status != "disabled" {
		t.Fatalf("revoked node metadata = %#v", revoked)
	}
}

func TestNodeTrustUpdateControlsExistingNodeTrustMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "trusted-candidate",
		DisplayName:   "Trusted Candidate",
		Role:          "passive",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANUnverified,
		URL:           "http://studio.local:11434",
		Status:        "healthy",
		Enabled:       true,
		Approved:      true,
		ApprovalState: appstate.ApprovalStateApproved,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/trusted-candidate/trust", bytes.NewBufferString(`{"trust_level":"external"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("trust update status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("trust response leaked forbidden marker: %s", rr.Body.String())
	}
	var node appstate.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &node); err != nil {
		t.Fatalf("decode trusted node: %v", err)
	}
	if node.TrustLevel != appstate.TrustLevelExternal {
		t.Fatalf("trust level = %q, want external", node.TrustLevel)
	}
	if node.ControlLevel != appstate.ControlLevelPassive || node.ApprovalState != appstate.ApprovalStateApproved || !node.Approved || !node.Enabled {
		t.Fatalf("trust update changed unrelated metadata = %#v", node)
	}
	if node.Observed["trust_updated_at"] == nil {
		t.Fatalf("trust update metadata missing: %#v", node.Observed)
	}
}

func TestNodeTrustUpdateRejectsInvalidTrustLevel(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "node-1",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		Enabled:       true,
		Approved:      true,
		ApprovalState: appstate.ApprovalStateApproved,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/node-1/trust", bytes.NewBufferString(`{"trust_level":"trusted-ish"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid trust update status = %d body = %s", rr.Code, rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["node-1"]
	if node.TrustLevel != appstate.TrustLevelLocal {
		t.Fatalf("invalid trust update mutated node = %#v", node)
	}
}

func TestManagedEnrollmentTokenRegistersPendingNode(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/enrollment-tokens", bytes.NewBufferString(`{"node_id":"worker-1","subscriber_url":"http://worker.local:11436","trust_level":"lan_trusted","ttl_minutes":5}`))
	server.createEnrollmentToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create enrollment token status = %d body = %s", rr.Code, rr.Body.String())
	}
	var tokenBody map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode token body: %v", err)
	}
	token, _ := tokenBody["token"].(string)
	if token == "" || !strings.HasPrefix(token, "lw_enroll_") {
		t.Fatalf("token = %q", token)
	}
	state := server.store.Snapshot()
	if len(state.EnrollmentQueue) != 1 {
		t.Fatalf("enrollment queue = %#v", state.EnrollmentQueue)
	}
	if state.EnrollmentQueue[0].TokenHash == "" || strings.Contains(state.EnrollmentQueue[0].TokenHash, token) {
		t.Fatalf("token hash not stored safely: %#v", state.EnrollmentQueue[0])
	}
	if state.EnrollmentQueue[0].TrustLevel != appstate.TrustLevelLANTrusted {
		t.Fatalf("trust = %q", state.EnrollmentQueue[0].TrustLevel)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/enroll", bytes.NewBufferString(`{"token":"`+token+`","node_id":"worker-1","display_name":"Worker One","subscriber_url":"http://worker.local:11436","hostname":"worker.local","platform":"linux","arch":"amd64","ollama_available":true,"models":[{"name":"llama3.1:8b","state":"installed"}]}`))
	server.subscriberEnroll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("subscriber enroll status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), token) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("enroll response leaked forbidden marker: %s", rr.Body.String())
	}
	var enrollBody struct {
		Status        string            `json:"status"`
		Node          appstate.Node     `json:"node"`
		HeartbeatAuth map[string]string `json:"heartbeat_auth"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &enrollBody); err != nil {
		t.Fatalf("decode enroll response: %v", err)
	}
	if enrollBody.Status != "pending_approval" {
		t.Fatalf("enroll status = %q", enrollBody.Status)
	}
	node := enrollBody.Node
	if node.ControlLevel != appstate.ControlLevelManaged || node.TrustLevel != appstate.TrustLevelLANTrusted || node.ApprovalState != appstate.ApprovalStatePending || node.Approved {
		t.Fatalf("managed pending metadata = %#v", node)
	}
	if node.CapabilitySource != appstate.CapabilitySourceSubscriberReported || node.ModelInventorySource != appstate.ModelInventorySourceSubscriberReported {
		t.Fatalf("subscriber source metadata = %#v", node)
	}
	heartbeatCredential := deriveSubscriberHeartbeatCredential(token, node.NodeID)
	if enrollBody.HeartbeatAuth["method"] != "shared_secret" || enrollBody.HeartbeatAuth["credential_derivation"] == "" {
		t.Fatalf("heartbeat auth metadata = %#v", enrollBody.HeartbeatAuth)
	}
	if strings.Contains(rr.Body.String(), heartbeatCredential) || server.secrets.Get(subscriberHeartbeatSecretKey(node.NodeID)) != heartbeatCredential {
		t.Fatalf("heartbeat credential handling unsafe; response=%s stored=%q", rr.Body.String(), server.secrets.Get(subscriberHeartbeatSecretKey(node.NodeID)))
	}
	if node.Observed["heartbeat_auth_method"] != "shared_secret" || node.Observed["heartbeat_auth_required"] != true {
		t.Fatalf("heartbeat auth node metadata = %#v", node.Observed)
	}
	state = server.store.Snapshot()
	if state.EnrollmentQueue[0].TokenHash != "" || state.EnrollmentQueue[0].RegisteredAt.IsZero() {
		t.Fatalf("token not consumed into queue metadata: %#v", state.EnrollmentQueue[0])
	}
	for _, route := range []string{"/wrangler/ui/bootstrap", "/wrangler/support-bundle/export"} {
		rr = httptest.NewRecorder()
		method := http.MethodGet
		if strings.Contains(route, "support-bundle") {
			method = http.MethodPost
		}
		req = httptest.NewRequest(method, route, nil)
		if method == http.MethodPost {
			server.exportSupportBundle(rr, req)
		} else {
			server.bootstrap(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), token) || strings.Contains(rr.Body.String(), heartbeatCredential) {
			t.Fatalf("%s leaked enrollment or heartbeat credential: %s", route, rr.Body.String())
		}
	}
}

func TestManagedEnrollmentRejectsInvalidTokenAndSupportBundleExcludesTokenMaterial(t *testing.T) {
	server := newIsolatedTestServer(t)
	rawToken := "lw_enroll_SECRET"
	reqMeta, err := server.store.AddEnrollmentRequest(appstate.EnrollmentRequest{
		NodeID:           "worker-2",
		URL:              "http://worker2.local:11436",
		ControlLevel:     appstate.ControlLevelManaged,
		TrustLevel:       appstate.TrustLevelLANUnverified,
		CapabilitySource: appstate.CapabilitySourceSubscriberReported,
		ApprovalState:    appstate.ApprovalStatePending,
		TokenHash:        enrollmentTokenHash(rawToken),
		TokenHint:        tokenHint(rawToken),
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("add enrollment request: %v", err)
	}
	if reqMeta.TokenHash == "" {
		t.Fatalf("missing token hash")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/enroll", bytes.NewBufferString(`{"token":"wrong-token","node_id":"worker-2","subscriber_url":"http://worker2.local:11436"}`))
	server.subscriberEnroll(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d body = %s", rr.Code, rr.Body.String())
	}
	if _, ok := server.store.Snapshot().Nodes["worker-2"]; ok {
		t.Fatalf("invalid token created node")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), rawToken) || strings.Contains(rr.Body.String(), reqMeta.TokenHash) {
		t.Fatalf("bootstrap leaked token material: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/support-bundle/export", nil)
	server.exportSupportBundle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("support bundle status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), rawToken) || strings.Contains(rr.Body.String(), reqMeta.TokenHash) {
		t.Fatalf("support bundle leaked token material: %s", rr.Body.String())
	}
}

func TestSubscriberHeartbeatUpdatesManagedNodeFreshnessMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	oldReport := time.Now().UTC().Add(-10 * time.Minute)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:               "worker-heartbeat",
		DisplayName:          "Worker Heartbeat",
		URL:                  "http://worker.local:11436",
		Role:                 "subscriber",
		ControlLevel:         appstate.ControlLevelManaged,
		TrustLevel:           appstate.TrustLevelLANTrusted,
		CapabilitySource:     appstate.CapabilitySourceSubscriberReported,
		ApprovalState:        appstate.ApprovalStateApproved,
		HealthSource:         appstate.HealthSourceSubscriberReported,
		ModelInventorySource: appstate.ModelInventorySourceSubscriberReported,
		Status:               "healthy",
		Enabled:              true,
		Approved:             true,
		LastReportedAt:       &oldReport,
		Observed:             map[string]interface{}{"heartbeat_required": true, "heartbeat_state": "stale"},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"worker-heartbeat","subscriber_url":"http://worker.local:11436","hostname":"worker.local","platform":"linux","arch":"amd64","status":"healthy","ollama_available":true,"models":[{"name":"llama3.1:8b","state":"warm"}],"active_jobs":1,"queue_depth":2}`))
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("heartbeat response leaked forbidden marker: %s", rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["worker-heartbeat"]
	if node.ApprovalState != appstate.ApprovalStateApproved || !node.Approved || node.TrustLevel != appstate.TrustLevelLANTrusted {
		t.Fatalf("heartbeat changed approval/trust metadata = %#v", node)
	}
	if node.LastReportedAt == nil || !node.LastReportedAt.After(oldReport) {
		t.Fatalf("last reported not refreshed: %#v", node.LastReportedAt)
	}
	if node.Observed["heartbeat_state"] != "fresh" || node.Observed["heartbeat_required"] != true {
		t.Fatalf("heartbeat metadata = %#v", node.Observed)
	}
	if node.Observed["heartbeat_auth_method"] != "legacy_unverified" || node.Observed["heartbeat_identity_verified"] != false {
		t.Fatalf("legacy heartbeat auth metadata = %#v", node.Observed)
	}
	if len(node.Models) != 1 || node.Models[0].Name != "llama3.1:8b" || node.ActiveJobs != 1 || node.QueueDepth != 2 {
		t.Fatalf("heartbeat reported metadata not stored: %#v", node)
	}
}

func TestSubscriberHeartbeatRequiresStoredCredentialForEnrolledNode(t *testing.T) {
	server := newIsolatedTestServer(t)
	rawToken := "lw_enroll_SECRET"
	now := time.Now().UTC()
	if _, err := server.store.AddEnrollmentRequest(appstate.EnrollmentRequest{
		NodeID:           "worker-secure",
		URL:              "http://worker-secure.local:11436",
		ControlLevel:     appstate.ControlLevelManaged,
		TrustLevel:       appstate.TrustLevelLANTrusted,
		CapabilitySource: appstate.CapabilitySourceSubscriberReported,
		ApprovalState:    appstate.ApprovalStatePending,
		TokenHash:        enrollmentTokenHash(rawToken),
		TokenHint:        tokenHint(rawToken),
		CreatedAt:        now,
		ExpiresAt:        now.Add(15 * time.Minute),
	}); err != nil {
		t.Fatalf("add enrollment request: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/enroll", bytes.NewBufferString(`{"token":"`+rawToken+`","node_id":"worker-secure","subscriber_url":"http://worker-secure.local:11436"}`))
	server.subscriberEnroll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("subscriber enroll status = %d body = %s", rr.Code, rr.Body.String())
	}
	heartbeatCredential := deriveSubscriberHeartbeatCredential(rawToken, "worker-secure")
	if heartbeatCredential == "" || server.secrets.Get(subscriberHeartbeatSecretKey("worker-secure")) != heartbeatCredential {
		t.Fatalf("heartbeat credential not stored in secret backend")
	}
	for name, configure := range map[string]func(*http.Request){
		"missing": func(req *http.Request) {},
		"wrong": func(req *http.Request) {
			req.Header.Set("X-Llama-Wrangler-Subscriber-Token", "wrong-token")
		},
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"worker-secure","status":"healthy"}`))
			configure(req)
			server.subscriberHeartbeat(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("heartbeat status = %d body = %s", rr.Code, rr.Body.String())
			}
			if strings.Contains(rr.Body.String(), heartbeatCredential) || strings.Contains(rr.Body.String(), rawToken) {
				t.Fatalf("heartbeat auth failure leaked secret: %s", rr.Body.String())
			}
		})
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"worker-secure","status":"healthy","active_jobs":1}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", heartbeatCredential)
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), heartbeatCredential) || strings.Contains(rr.Body.String(), rawToken) {
		t.Fatalf("heartbeat response leaked secret: %s", rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["worker-secure"]
	if node.Observed["heartbeat_auth_method"] != "shared_secret" || node.Observed["heartbeat_identity_verified"] != true {
		t.Fatalf("heartbeat auth metadata = %#v", node.Observed)
	}
}

func TestManagedNodeHeartbeatCredentialRotationReprovisionsLegacyNode(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "manual-worker",
		DisplayName:   "Manual Worker",
		URL:           "http://manual-worker.local:11436",
		Role:          "subscriber",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed:      map[string]interface{}{"manual_add": true, "heartbeat_auth_method": "legacy_unverified"},
	}); err != nil {
		t.Fatalf("upsert legacy node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/manual-worker/heartbeat-credential/rotate", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rotate heartbeat credential status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Status            string                          `json:"status"`
		Credential        string                          `json:"credential"`
		Node              appstate.Node                   `json:"node"`
		HeartbeatAuth     map[string]string               `json:"heartbeat_auth"`
		SubscriberInstall subscriberCredentialInstallPlan `json:"subscriber_install"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if body.Status != "rotated" || !strings.HasPrefix(body.Credential, "lw_hb_") {
		t.Fatalf("rotation response = %#v", body)
	}
	if body.HeartbeatAuth["method"] != "shared_secret" || body.HeartbeatAuth["credential_derivation"] != "random_shared_secret_v1" {
		t.Fatalf("heartbeat auth metadata = %#v", body.HeartbeatAuth)
	}
	if body.SubscriberInstall.EnvironmentVariable != subscriberHeartbeatCredentialEnv || body.SubscriberInstall.ConfigKey != "registration.heartbeat_credential_env" {
		t.Fatalf("subscriber install plan missing env/config hook: %#v", body.SubscriberInstall)
	}
	if body.SubscriberInstall.EnvFilePath == "" || !strings.Contains(body.SubscriberInstall.EnvFileTemplate, subscriberHeartbeatCredentialEnv) {
		t.Fatalf("subscriber install plan missing env-file template: %#v", body.SubscriberInstall)
	}
	if !strings.Contains(body.SubscriberInstall.LaunchdDryRunCommand, "service-dry-run") || !strings.Contains(body.SubscriberInstall.HeartbeatCheckCommand, "/subscriber/heartbeat") {
		t.Fatalf("subscriber install commands missing: %#v", body.SubscriberInstall)
	}
	if body.SubscriberInstall.ServiceWrapper.Target != "launchd" || body.SubscriberInstall.ServiceWrapper.Label == "" || body.SubscriberInstall.ServiceWrapper.PlistPath == "" {
		t.Fatalf("subscriber service wrapper plan missing launchd metadata: %#v", body.SubscriberInstall.ServiceWrapper)
	}
	if !strings.Contains(body.SubscriberInstall.ServiceWrapper.LaunchdPlistTemplate, subscriberHeartbeatCredentialEnv) || !strings.Contains(body.SubscriberInstall.ServiceWrapper.LaunchdPlistTemplate, "&lt;credential-from-rotation-response&gt;") {
		t.Fatalf("subscriber launchd plist template missing env placeholder: %s", body.SubscriberInstall.ServiceWrapper.LaunchdPlistTemplate)
	}
	if len(body.SubscriberInstall.ServiceWrapper.InstallCommands) == 0 || len(body.SubscriberInstall.ServiceWrapper.ValidationCommands) == 0 || len(body.SubscriberInstall.ServiceWrapper.UninstallCommands) == 0 {
		t.Fatalf("subscriber service wrapper commands missing: %#v", body.SubscriberInstall.ServiceWrapper)
	}
	installJSON := string(mustJSON(t, body.SubscriberInstall))
	if strings.Contains(installJSON, body.Credential) || strings.Contains(installJSON, "lw_hb_") {
		t.Fatalf("subscriber install plan should use placeholders and not duplicate raw credential: %s", installJSON)
	}
	for _, command := range []string{
		body.SubscriberInstall.ShellExportCommand,
		body.SubscriberInstall.EnvFileTemplate,
		body.SubscriberInstall.LaunchdDryRunCommand,
		body.SubscriberInstall.HeartbeatCheckCommand,
		strings.Join(body.SubscriberInstall.ServiceWrapper.ValidationCommands, "\n"),
	} {
		if !strings.Contains(command, subscriberCredentialPlaceholder) || strings.Contains(command, body.Credential) {
			t.Fatalf("subscriber install command should use placeholder only: %s", command)
		}
	}
	if strings.Contains(body.SubscriberInstall.ServiceWrapper.LaunchdPlistTemplate, body.Credential) {
		t.Fatalf("subscriber launchd plist template leaked raw credential: %s", body.SubscriberInstall.ServiceWrapper.LaunchdPlistTemplate)
	}
	if strings.Contains(string(mustJSON(t, body.Node)), body.Credential) {
		t.Fatalf("node metadata leaked rotated credential: %#v", body.Node)
	}
	if server.secrets.Get(subscriberHeartbeatSecretKey("manual-worker")) != body.Credential {
		t.Fatalf("rotated credential not stored in secret backend")
	}
	node := server.store.Snapshot().Nodes["manual-worker"]
	if node.Observed["heartbeat_auth_method"] != "shared_secret" || node.Observed["heartbeat_auth_required"] != true {
		t.Fatalf("rotation metadata = %#v", node.Observed)
	}
	if node.Observed["heartbeat_credential_derivation"] != "random_shared_secret_v1" || node.Observed["heartbeat_reprovisioning_required"] != true {
		t.Fatalf("rotation provisioning metadata = %#v", node.Observed)
	}

	for _, route := range []string{"/wrangler/ui/bootstrap", "/wrangler/support-bundle/export"} {
		rr = httptest.NewRecorder()
		method := http.MethodGet
		if strings.Contains(route, "support-bundle") {
			method = http.MethodPost
		}
		req = httptest.NewRequest(method, route, nil)
		if method == http.MethodPost {
			server.exportSupportBundle(rr, req)
		} else {
			server.bootstrap(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), body.Credential) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
			t.Fatalf("%s leaked rotated credential or forbidden marker: %s", route, rr.Body.String())
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"manual-worker","status":"healthy"}`))
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing rotated credential heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"manual-worker","status":"healthy","active_jobs":2}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", body.Credential)
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("credential heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), body.Credential) {
		t.Fatalf("heartbeat response leaked rotated credential: %s", rr.Body.String())
	}
	node = server.store.Snapshot().Nodes["manual-worker"]
	if node.Observed["heartbeat_identity_verified"] != true || node.Observed["heartbeat_reprovisioning_required"] != false {
		t.Fatalf("verified heartbeat metadata = %#v", node.Observed)
	}
	if node.ActiveJobs != 2 {
		t.Fatalf("heartbeat metadata not applied after credential auth: %#v", node)
	}
}

func TestManagedNodeHeartbeatCredentialRotationInvalidatesOldCredentialAndRejectsPassive(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "secure-worker",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert secure node: %v", err)
	}
	oldCredential := "lw_hb_old_SECRET"
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("secure-worker"), oldCredential); err != nil {
		t.Fatalf("seed old credential: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-node",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/secure-worker/heartbeat-credential/rotate", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rotate heartbeat credential status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	newCredential, _ := body["credential"].(string)
	if newCredential == "" || newCredential == oldCredential || server.secrets.Get(subscriberHeartbeatSecretKey("secure-worker")) != newCredential {
		t.Fatalf("credential was not rotated safely: old=%q new=%q", oldCredential, newCredential)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"secure-worker","status":"healthy"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", oldCredential)
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("old credential heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), oldCredential) || strings.Contains(rr.Body.String(), newCredential) {
		t.Fatalf("old credential rejection leaked secret: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-node/heartbeat-credential/rotate", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("passive rotation status = %d body = %s", rr.Code, rr.Body.String())
	}
	if server.secrets.Get(subscriberHeartbeatSecretKey("passive-node")) != "" {
		t.Fatalf("passive node received heartbeat credential")
	}
}

func TestSubscriberHeartbeatRejectsUnknownOrPassiveNodes(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-node",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"missing-node"}`))
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/heartbeat", bytes.NewBufferString(`{"node_id":"passive-node"}`))
	server.subscriberHeartbeat(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("passive heartbeat status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
