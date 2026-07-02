package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
)

func TestSupportBundleIncludesSchemaConfigAndQueueMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:       "passive-endpoint",
		DisplayName:  "Passive Endpoint",
		Role:         "passive",
		ControlLevel: appstate.ControlLevelPassive,
		URL:          "http://studio.local:11434",
		Status:       "unknown",
		Enabled:      true,
		Approved:     false,
	}); err != nil {
		t.Fatalf("upsert passive endpoint: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/support-bundle/export", nil)
	server.exportSupportBundle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	bundleSchema, ok := body["bundle_schema"].(map[string]interface{})
	if !ok {
		t.Fatalf("bundle_schema metadata missing: %#v", body["bundle_schema"])
	}
	if bundleSchema["name"] != supportBundleSchemaName {
		t.Fatalf("bundle_schema name = %#v", bundleSchema["name"])
	}
	if bundleSchema["version"].(float64) != supportBundleSchemaVersion {
		t.Fatalf("bundle_schema version = %#v", bundleSchema["version"])
	}
	if bundleSchema["json_schema"] != supportBundleJSONSchemaPath || bundleSchema["documentation"] != supportBundleDocumentationPath {
		t.Fatalf("bundle_schema paths = %#v", bundleSchema)
	}
	if bundleSchema["compatibility"] != supportBundleCompatibilityPolicy {
		t.Fatalf("bundle_schema compatibility = %#v", bundleSchema["compatibility"])
	}
	service, ok := body["service"].(map[string]interface{})
	if !ok {
		t.Fatalf("service metadata missing: %#v", body["service"])
	}
	if service["schema_version"].(float64) != appstate.CurrentSchemaVersion {
		t.Fatalf("schema_version = %#v", service["schema_version"])
	}
	if service["config_version"].(float64) < 1 {
		t.Fatalf("config_version = %#v", service["config_version"])
	}
	if _, ok := body["config"].(map[string]interface{}); !ok {
		t.Fatalf("config missing: %#v", body["config"])
	}
	nodes, ok := body["nodes"].(map[string]interface{})
	if !ok {
		t.Fatalf("nodes missing: %#v", body["nodes"])
	}
	passive, ok := nodes["passive-endpoint"].(map[string]interface{})
	if !ok {
		t.Fatalf("passive endpoint missing from support bundle: %#v", nodes)
	}
	if passive["control_level"] != appstate.ControlLevelPassive || passive["trust_level"] != appstate.TrustLevelLANUnverified {
		t.Fatalf("passive metadata = %#v", passive)
	}
	if passive["capability_source"] != appstate.CapabilitySourceMarshalObserved || passive["approval_state"] != appstate.ApprovalStatePending {
		t.Fatalf("passive source/approval metadata = %#v", passive)
	}
	queue, ok := body["queue"].(map[string]interface{})
	if !ok {
		t.Fatalf("queue missing: %#v", body["queue"])
	}
	if queue["max_depth"].(float64) <= 0 {
		t.Fatalf("queue max_depth = %#v", queue["max_depth"])
	}
	secretStorage, ok := body["secret_storage"].(map[string]interface{})
	if !ok {
		t.Fatalf("secret_storage missing: %#v", body["secret_storage"])
	}
	requiredFiles, ok := secretStorage["backup_required_files"].([]interface{})
	if !ok || !interfaceSliceContains(requiredFiles, "secrets.enc.json") || !interfaceSliceContains(requiredFiles, "secrets.key") {
		t.Fatalf("backup_required_files = %#v", secretStorage["backup_required_files"])
	}
	if description, _ := secretStorage["backup_description"].(string); !strings.Contains(description, "Back up secrets.enc.json and secrets.key together") {
		t.Fatalf("backup_description = %q", description)
	}
	if description, _ := secretStorage["restore_description"].(string); !strings.Contains(description, "Restore both files together") {
		t.Fatalf("restore_description = %q", description)
	}
	privacy, ok := body["privacy"].(map[string]interface{})
	if !ok {
		t.Fatalf("privacy missing: %#v", body["privacy"])
	}
	if privacy["secrets_included"].(bool) || privacy["prompt_bodies_included"].(bool) || privacy["response_bodies_included"].(bool) {
		t.Fatalf("privacy flags = %#v", privacy)
	}
}

func interfaceSliceContains(items []interface{}, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestSupportBundleRedactsSecretsPayloadsAndTokenLikeValues(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.secrets.Set("splunk_hec_token", "splunk-token-secret"); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:          "observed-node",
		DisplayName:     "Observed Node",
		Status:          "healthy",
		Enabled:         true,
		Approved:        true,
		OllamaAvailable: true,
		Observed: map[string]interface{}{
			"safe_status":    "ok",
			"authorization":  "Bearer lw_client_SECRET",
			"payload":        "SECRET_PROMPT",
			"response_body":  "SECRET_RESPONSE",
			"nested_visible": map[string]interface{}{"safe": "kept", "token": "hidden-token"},
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	server.store.AddAudit(appstate.AuditEvent{
		Timestamp: time.Now().UTC(),
		Type:      "test_event",
		RequestID: "req_support",
		Message:   "metadata only",
		Fields: map[string]interface{}{
			"safe":          "kept",
			"prompt":        "SECRET_PROMPT",
			"messages":      []interface{}{"SECRET_MESSAGE"},
			"response":      "SECRET_RESPONSE",
			"api_key":       "lw_client_SECRET",
			"authorization": "Bearer abc",
			"nested": map[string]interface{}{
				"safe_nested": "kept",
				"secret":      "hidden",
			},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/support-bundle/export", nil)
	server.exportSupportBundle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	raw := rr.Body.String()
	for _, leaked := range []string{
		"SECRET_PROMPT",
		"SECRET_MESSAGE",
		"SECRET_RESPONSE",
		"lw_client_SECRET",
		"Bearer abc",
		"splunk-token-secret",
		"hidden-token",
	} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("support bundle leaked %q: %s", leaked, raw)
		}
	}
	for _, expected := range []string{"safe_status", "safe_nested", "kept", "encrypted_file", "schema_version", "config_version"} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("support bundle missing expected %q: %s", expected, raw)
		}
	}
}

func TestSupportBundleSchemaArtifactsMatchRuntimeMetadata(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../" + supportBundleJSONSchemaPath)
	if err != nil {
		t.Fatalf("read support bundle JSON schema: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("decode support bundle JSON schema: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	bundleSchema := properties["bundle_schema"].(map[string]interface{})
	bundleSchemaProps := bundleSchema["properties"].(map[string]interface{})
	if bundleSchemaProps["name"].(map[string]interface{})["const"] != supportBundleSchemaName {
		t.Fatalf("schema name const mismatch: %#v", bundleSchemaProps["name"])
	}
	if bundleSchemaProps["version"].(map[string]interface{})["const"].(float64) != supportBundleSchemaVersion {
		t.Fatalf("schema version const mismatch: %#v", bundleSchemaProps["version"])
	}
	if bundleSchemaProps["json_schema"].(map[string]interface{})["const"] != supportBundleJSONSchemaPath {
		t.Fatalf("schema json_schema const mismatch: %#v", bundleSchemaProps["json_schema"])
	}
	if bundleSchemaProps["documentation"].(map[string]interface{})["const"] != supportBundleDocumentationPath {
		t.Fatalf("schema documentation const mismatch: %#v", bundleSchemaProps["documentation"])
	}
	if bundleSchemaProps["compatibility"].(map[string]interface{})["const"] != supportBundleCompatibilityPolicy {
		t.Fatalf("schema compatibility const mismatch: %#v", bundleSchemaProps["compatibility"])
	}
	privacy := properties["privacy"].(map[string]interface{})["properties"].(map[string]interface{})
	for _, field := range []string{"prompt_bodies_included", "response_bodies_included", "secrets_included"} {
		if privacy[field].(map[string]interface{})["const"] != false {
			t.Fatalf("privacy field %s must be const false: %#v", field, privacy[field])
		}
	}

	docBytes, err := os.ReadFile("../../" + supportBundleDocumentationPath)
	if err != nil {
		t.Fatalf("read support bundle documentation: %v", err)
	}
	doc := string(docBytes)
	for _, expected := range []string{
		supportBundleSchemaName,
		`"version": 1`,
		supportBundleJSONSchemaPath,
		supportBundleCompatibilityPolicy,
		"additive-backward-compatible",
		"prompt_bodies_included",
		"secrets_included",
		"not backup or restore artifacts",
		"backup_required_files",
		"docs/11_configuration_storage.md",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("support bundle documentation missing %q", expected)
		}
	}
}
