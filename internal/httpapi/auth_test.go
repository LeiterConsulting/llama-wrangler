package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"llama-wrangler/internal/config"
)

func newIsolatedTestServer(t *testing.T) *Server {
	t.Helper()
	return newIsolatedTestServerWithConfig(t, config.Default("marshal"))
}

func newIsolatedTestServerWithConfig(t *testing.T, cfg config.Config) *Server {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cfg.Telemetry.JSONLogs = false
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func TestManagementAuthAfterSetupComplete(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	adminToken, _ := body["admin_token"].(string)
	if adminToken == "" {
		t.Fatalf("setup response did not include admin token")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated config status = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("authenticated config status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestClientAPIKeyRequiredAfterSetupComplete(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	clientKey, _ := body["client_api_key"].(string)
	if clientKey == "" {
		t.Fatalf("setup response did not include client API key")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	server.requireClientAPIKey(server.openAIModels)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated models status = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+clientKey)
	server.requireClientAPIKey(server.openAIModels)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("authenticated models status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestAdminTokenRotationInvalidatesOldToken(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	oldToken, _ := body["admin_token"].(string)
	if oldToken == "" {
		t.Fatalf("setup response did not include admin token")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/auth/admin-token/rotate", nil)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	server.requireAdmin(server.rotateAdminTokenHandler)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rotate status = %d body = %s", rr.Code, rr.Body.String())
	}
	body = map[string]interface{}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	newToken, _ := body["token"].(string)
	if newToken == "" || newToken == oldToken {
		t.Fatalf("new token was not rotated")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("old token status = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("new token status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestClientAPIKeyRevokeInvalidatesKey(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	clientKey, _ := body["client_api_key"].(string)
	keyID, _ := body["client_api_key_id"].(string)
	if clientKey == "" || keyID == "" {
		t.Fatalf("setup response did not include client key and id")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/auth/api-keys/"+keyID+"/revoke", nil)
	server.clientAPIKeyAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+clientKey)
	server.requireClientAPIKey(server.openAIModels)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key status = %d, want 401", rr.Code)
	}
}

func TestSecretRekeyRequiresAdminAndPreservesCredentials(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var setup map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	adminToken := setup["admin_token"].(string)
	clientKey := setup["client_api_key"].(string)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/secrets/rekey", nil)
	server.requireAdmin(server.rekeySecretsHandler)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated rekey status = %d body = %s, want 401", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/secrets/rekey", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	server.requireAdmin(server.rekeySecretsHandler)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("authenticated rekey status = %d body = %s", rr.Code, rr.Body.String())
	}
	raw := rr.Body.String()
	if strings.Contains(raw, "lw_admin_") || strings.Contains(raw, "lw_client_") {
		t.Fatalf("rekey response leaked token material: %s", raw)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode rekey response: %v", err)
	}
	if body["rekeyed"] != true {
		t.Fatalf("rekeyed = %#v", body["rekeyed"])
	}
	status, ok := body["status"].(map[string]interface{})
	if !ok {
		t.Fatalf("status missing: %#v", body["status"])
	}
	if status["backend"] != "encrypted_file" || status["key_source"] != "file" || status["rekey_supported"] != true {
		t.Fatalf("secret status = %#v", status)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin token after rekey status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+clientKey)
	server.requireClientAPIKey(server.openAIModels)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("client key after rekey status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestBootstrapIncludesLANExposureWarning(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.Server.Listen = "0.0.0.0:11435"
	server := newIsolatedTestServerWithConfig(t, cfg)

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
	safeDefaults, ok := body["safe_defaults"].(map[string]interface{})
	if !ok {
		t.Fatalf("safe_defaults missing: %#v", body["safe_defaults"])
	}
	if safeDefaults["lan_access_enabled"] != true {
		t.Fatalf("lan_access_enabled = %#v, want true", safeDefaults["lan_access_enabled"])
	}
	if safeDefaults["lan_access_by_default"] != false || safeDefaults["lan_requires_explicit_enablement"] != true {
		t.Fatalf("lan safe defaults = %#v", safeDefaults)
	}
	if safeDefaults["lan_access_warning"] == "" {
		t.Fatalf("expected LAN warning in safe defaults: %#v", safeDefaults)
	}
}

func TestAdminAuthFailuresAreRateLimited(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	var setup map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &setup); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	adminToken := setup["admin_token"].(string)

	for i := 0; i < defaultAuthFailureLimit-1; i++ {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
		req.RemoteAddr = "198.51.100.10:54321"
		req.Header.Set("Authorization", "Bearer invalid-admin-token")
		server.requireAdmin(server.getConfig)(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d body = %s, want 401", i+1, rr.Code, rr.Body.String())
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.RemoteAddr = "198.51.100.10:54321"
	req.Header.Set("Authorization", "Bearer invalid-admin-token")
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limited status = %d body = %s, want 429", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatalf("Retry-After header missing")
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rate limit body: %v", err)
	}
	if body["error"] != "admin_auth_rate_limited" {
		t.Fatalf("rate limit body = %#v", body)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/config", nil)
	req.RemoteAddr = "198.51.100.10:54321"
	req.Header.Set("Authorization", "Bearer "+adminToken)
	server.requireAdmin(server.getConfig)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid token after failures status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestClientAuthFailuresAreRateLimitedWithCompatibleShapes(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}

	for i := 0; i < defaultAuthFailureLimit; i++ {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.RemoteAddr = "203.0.113.25:43210"
		req.Header.Set("Authorization", "Bearer invalid-client-key")
		server.requireClientAPIKey(server.openAIModels)(rr, req)
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("openai rate limited status = %d body = %s, want 429", rr.Code, rr.Body.String())
	}
	var openAI struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &openAI); err != nil {
		t.Fatalf("decode openai rate limit: %v", err)
	}
	if openAI.Error.Code != errorCodeClientAuthRateLimited || openAI.Error.Type != "rate_limit_error" {
		t.Fatalf("openai rate limit error = %+v", openAI.Error)
	}

	server = newIsolatedTestServer(t)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
	for i := 0; i < defaultAuthFailureLimit; i++ {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/tags", nil)
		req.RemoteAddr = "203.0.113.26:43210"
		req.Header.Set("Authorization", "Bearer invalid-client-key")
		server.requireClientAPIKey(server.apiTags)(rr, req)
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("ollama rate limited status = %d body = %s, want 429", rr.Code, rr.Body.String())
	}
	var ollama map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &ollama); err != nil {
		t.Fatalf("decode ollama rate limit: %v", err)
	}
	if ollama["code"] != errorCodeClientAuthRateLimited || ollama["type"] != "rate_limit_error" {
		t.Fatalf("ollama rate limit error = %#v", ollama)
	}
	if _, ok := ollama["error"].(string); !ok {
		t.Fatalf("ollama error should be string: %#v", ollama["error"])
	}
}
