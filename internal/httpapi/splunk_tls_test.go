package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/config"
)

func TestSplunkTLSWarningMetadataWhenVerificationDisabled(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.Telemetry.SplunkHEC.Enabled = true
	cfg.Telemetry.SplunkHEC.URL = "https://splunk.local:8088/services/collector"
	cfg.Telemetry.SplunkHEC.VerifySSL = false
	server := newIsolatedTestServerWithConfig(t, cfg)
	if err := server.secrets.Set("splunk_hec_token", "super-secret-hec-token"); err != nil {
		t.Fatalf("seed HEC token: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	body := decodeMap(t, rr.Body.Bytes())
	bootstrapHEC := nestedMap(t, body, "telemetry", "splunk_hec")
	assertSplunkTLSWarning(t, bootstrapHEC)

	raw := rr.Body.String()
	for _, forbidden := range []string{"super-secret-hec-token", "Authorization", "lw_admin_", "lw_client_"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("bootstrap leaked forbidden marker %q: %s", forbidden, raw)
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/telemetry/status", nil)
	server.telemetryStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d body = %s", rr.Code, rr.Body.String())
	}
	body = decodeMap(t, rr.Body.Bytes())
	statusHEC := nestedMap(t, body, "splunk_hec")
	assertSplunkTLSWarning(t, statusHEC)
	if strings.Contains(rr.Body.String(), "super-secret-hec-token") {
		t.Fatalf("telemetry status leaked HEC token: %s", rr.Body.String())
	}
}

func TestSplunkTLSWarningMetadataClearWhenVerificationEnabled(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	body := decodeMap(t, rr.Body.Bytes())
	hec := nestedMap(t, body, "telemetry", "splunk_hec")
	if hec["tls_verification_disabled"] != false || hec["tls_warning"] != "" {
		t.Fatalf("default TLS status should be clear: %#v", hec)
	}
}

func assertSplunkTLSWarning(t *testing.T, hec map[string]interface{}) {
	t.Helper()
	if hec["verify_ssl"] != false {
		t.Fatalf("verify_ssl = %#v, want false in %#v", hec["verify_ssl"], hec)
	}
	if hec["tls_verification_disabled"] != true {
		t.Fatalf("tls_verification_disabled = %#v, want true in %#v", hec["tls_verification_disabled"], hec)
	}
	warning, _ := hec["tls_warning"].(string)
	if !strings.Contains(warning, "TLS certificate verification is disabled") || !strings.Contains(warning, "trusted self-signed Splunk lab certificates") {
		t.Fatalf("unexpected tls_warning = %q", warning)
	}
	if hec["has_token"] != true {
		t.Fatalf("has_token = %#v, want true in %#v", hec["has_token"], hec)
	}
}

func decodeMap(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return body
}

func nestedMap(t *testing.T, body map[string]interface{}, path ...string) map[string]interface{} {
	t.Helper()
	var current interface{} = body
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("path %v did not resolve to object at %q: %#v", path, key, current)
		}
		current = m[key]
	}
	result, ok := current.(map[string]interface{})
	if !ok {
		t.Fatalf("path %v did not resolve to object: %#v", path, current)
	}
	return result
}
