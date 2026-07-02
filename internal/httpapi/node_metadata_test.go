package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
