package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/config"
)

func TestBootstrapIncludesClientPresetsWithoutStoredSecrets(t *testing.T) {
	server := newIsolatedTestServer(t)
	setup := completeSetupForPresetTest(t, server)
	adminToken := setup["admin_token"].(string)
	clientKey := setup["client_api_key"].(string)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://wrangler.local:11435/wrangler/ui/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	bodyText := rr.Body.String()
	if strings.Contains(bodyText, clientKey) || strings.Contains(bodyText, "lw_client_") {
		t.Fatalf("bootstrap leaked client API key material: %s", bodyText)
	}

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(bodyText), &body); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	presets, ok := body["client_presets"].([]interface{})
	if !ok || len(presets) != 4 {
		t.Fatalf("client_presets = %#v, want 4 presets", body["client_presets"])
	}
	names := map[string]bool{}
	for _, raw := range presets {
		preset := raw.(map[string]interface{})
		names[preset["name"].(string)] = true
		if preset["api_key_placeholder"] != clientAPIKeyPlaceholder {
			t.Fatalf("preset placeholder = %#v", preset["api_key_placeholder"])
		}
		if preset["base_url"] != "http://wrangler.local:11435/v1" {
			t.Fatalf("preset base_url = %#v", preset["base_url"])
		}
		snippets := preset["snippets"].([]interface{})
		if len(snippets) == 0 {
			t.Fatalf("preset missing snippets: %#v", preset)
		}
		snippet := snippets[0].(map[string]interface{})
		if !strings.Contains(snippet["body"].(string), clientAPIKeyPlaceholder) {
			t.Fatalf("snippet missing API key placeholder: %#v", snippet)
		}
	}
	for _, name := range []string{"Cline", "Continue", "Open WebUI", "OpenAI SDK"} {
		if !names[name] {
			t.Fatalf("missing preset %q in %#v", name, names)
		}
	}
}

func TestClientPresetsEndpointUsesForwardedHTTPSAndAliasFallback(t *testing.T) {
	cfg := config.Default("marshal")
	delete(cfg.ModelAliases, "local-code")
	server := newIsolatedTestServerWithConfig(t, cfg)
	setup := completeSetupForPresetTest(t, server)
	adminToken := setup["admin_token"].(string)
	clientKey := setup["client_api_key"].(string)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost:11435/wrangler/client-presets", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("X-Forwarded-Proto", "https")
	server.requireAdmin(server.clientPresets)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("client presets status = %d body = %s", rr.Code, rr.Body.String())
	}
	bodyText := rr.Body.String()
	if strings.Contains(bodyText, clientKey) || strings.Contains(bodyText, "lw_client_") {
		t.Fatalf("client presets leaked client API key material: %s", bodyText)
	}
	var presets []clientPreset
	if err := json.Unmarshal([]byte(bodyText), &presets); err != nil {
		t.Fatalf("decode presets: %v", err)
	}
	if len(presets) != 4 {
		t.Fatalf("len(presets) = %d, want 4", len(presets))
	}
	if presets[0].BaseURL != "https://localhost:11435/v1" {
		t.Fatalf("forwarded base URL = %q", presets[0].BaseURL)
	}
	if presets[0].Model != cfg.Routing.DefaultModelAlias {
		t.Fatalf("missing local-code should fall back to default model: got %q want %q", presets[0].Model, cfg.Routing.DefaultModelAlias)
	}
}

func completeSetupForPresetTest(t *testing.T, server *Server) map[string]interface{} {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	if body["admin_token"] == "" || body["client_api_key"] == "" {
		t.Fatalf("setup did not return one-time tokens: %#v", body)
	}
	return body
}
