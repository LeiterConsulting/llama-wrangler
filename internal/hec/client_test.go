package hec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llama-wrangler/internal/config"
)

func TestSendBuildsHECPayload(t *testing.T) {
	var got Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Splunk test-token" {
			t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(config.SplunkHECConfig{
		Enabled: true,
		URL:     server.URL,
		Token:   "test-token",
		Index:   "llama_wrangler",
		Source:  "llama-wrangler",
		Prefix:  "llama_wrangler",
	})
	if err := client.Send("routing_decision", map[string]interface{}{"event_type": "routing_decision"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got.Sourcetype != "llama_wrangler:routing_decision" {
		t.Fatalf("sourcetype = %q", got.Sourcetype)
	}
	if got.Index != "llama_wrangler" {
		t.Fatalf("index = %q", got.Index)
	}
}

func TestSendDisabledNoops(t *testing.T) {
	client := New(config.SplunkHECConfig{Enabled: false})
	if err := client.Send("request", map[string]interface{}{}); err != nil {
		t.Fatalf("disabled Send() error = %v", err)
	}
}

func TestSendAllowsSelfSignedWhenVerifySSLDisabled(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(config.SplunkHECConfig{
		Enabled:   true,
		URL:       server.URL,
		Token:     "test-token",
		Index:     "llama_wrangler",
		Source:    "llama-wrangler",
		Prefix:    "llama_wrangler",
		VerifySSL: false,
	})
	if err := client.Send("config", map[string]interface{}{"event_type": "config"}); err != nil {
		t.Fatalf("Send() with VerifySSL=false error = %v", err)
	}
}

func TestSendRejectsSelfSignedWhenVerifySSLEnabled(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(config.SplunkHECConfig{
		Enabled:   true,
		URL:       server.URL,
		Token:     "test-token",
		Index:     "llama_wrangler",
		Source:    "llama-wrangler",
		Prefix:    "llama_wrangler",
		VerifySSL: true,
	})
	if err := client.Send("config", map[string]interface{}{"event_type": "config"}); err == nil {
		t.Fatalf("Send() with VerifySSL=true should reject self-signed cert")
	}
}
