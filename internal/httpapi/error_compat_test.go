package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/appstate"
)

func TestOpenAIClientAuthErrorShape(t *testing.T) {
	server := newIsolatedTestServer(t)
	completeSetupForErrorTest(t, server)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	server.requireClientAPIKey(server.openAIModels)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Error struct {
			Message string      `json:"message"`
			Type    string      `json:"type"`
			Param   interface{} `json:"param"`
			Code    string      `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != errorCodeClientKeyRequired || body.Error.Type != "authentication_error" || body.Error.Message == "" {
		t.Fatalf("openai error = %+v", body.Error)
	}
}

func TestOllamaClientAuthErrorShape(t *testing.T) {
	server := newIsolatedTestServer(t)
	completeSetupForErrorTest(t, server)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	server.requireClientAPIKey(server.apiTags)(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] == nil || body["code"] != errorCodeClientKeyRequired || body["type"] != "authentication_error" {
		t.Fatalf("ollama error = %#v", body)
	}
	if _, ok := body["error"].(string); !ok {
		t.Fatalf("ollama error field should remain a string: %#v", body["error"])
	}
}

func TestMarshalProxyOpenAIErrorDoesNotLeakPayloadOrUpstreamDetails(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)
	server.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp http://primary.test?token=lw_client_SECRET failed")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-fast","messages":[{"role":"user","content":"SECRET_PROMPT"}]}`))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	raw := rr.Body.String()
	for _, leaked := range []string{"SECRET_PROMPT", "primary.test", "lw_client_SECRET"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("error response leaked %q: %s", leaked, raw)
		}
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != errorCodeUpstreamUnavailable || body.Error.Type != "server_error" || body.Error.Message != "Upstream Ollama node is unavailable." {
		t.Fatalf("openai upstream error = %+v", body.Error)
	}
}

func TestNoEligibleNodeErrorShapes(t *testing.T) {
	server := newIsolatedTestServer(t)
	disableAllNodesForErrorTest(t, server)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-fast"}`))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("openai status = %d body = %s", rr.Code, rr.Body.String())
	}
	var openAI struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &openAI); err != nil {
		t.Fatalf("decode openai: %v", err)
	}
	if openAI.Error.Code != errorCodeNoEligibleNode || openAI.Error.Type != "server_error" {
		t.Fatalf("openai error = %+v", openAI.Error)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"model":"local-fast"}`))
	server.marshalProxy("/api/chat", "ollama_chat")(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("ollama status = %d body = %s", rr.Code, rr.Body.String())
	}
	var ollama map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &ollama); err != nil {
		t.Fatalf("decode ollama: %v", err)
	}
	if ollama["code"] != errorCodeNoEligibleNode || ollama["type"] != "server_error" {
		t.Fatalf("ollama error = %#v", ollama)
	}
	if _, ok := ollama["error"].(string); !ok {
		t.Fatalf("ollama error field should remain a string: %#v", ollama["error"])
	}
}

func TestUpstreamClientErrorIsNormalizedBySurface(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)
	server.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return proxyResponse(http.StatusNotFound, `{"error":"raw upstream mentions SECRET_PROMPT and lw_client_SECRET"}`), nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-fast","messages":[{"role":"user","content":"SECRET_PROMPT"}]}`))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("openai status = %d body = %s", rr.Code, rr.Body.String())
	}
	raw := rr.Body.String()
	for _, leaked := range []string{"raw upstream", "SECRET_PROMPT", "lw_client_SECRET"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("openai upstream error leaked %q: %s", leaked, raw)
		}
	}
	var openAI struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &openAI); err != nil {
		t.Fatalf("decode openai: %v", err)
	}
	if openAI.Error.Code != "upstream_not_found" || openAI.Error.Type != "invalid_request_error" {
		t.Fatalf("openai upstream error = %+v", openAI.Error)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"model":"local-fast"}`))
	server.marshalProxy("/api/chat", "ollama_chat")(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("ollama status = %d body = %s", rr.Code, rr.Body.String())
	}
	var ollama map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &ollama); err != nil {
		t.Fatalf("decode ollama: %v", err)
	}
	if ollama["code"] != "upstream_not_found" || ollama["type"] != "invalid_request_error" {
		t.Fatalf("ollama upstream error = %#v", ollama)
	}
	if _, ok := ollama["error"].(string); !ok {
		t.Fatalf("ollama error field should remain string: %#v", ollama["error"])
	}
}

func completeSetupForErrorTest(t *testing.T, server *Server) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/setup/complete", nil)
	server.setupComplete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setupComplete status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func disableAllNodesForErrorTest(t *testing.T, server *Server) {
	t.Helper()
	state := server.store.Snapshot()
	for _, node := range state.Nodes {
		node.Enabled = false
		node.Approved = false
		node.Status = "disabled"
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("disable node: %v", err)
		}
	}
	if len(state.Nodes) == 0 {
		if err := server.store.UpsertNode(appstate.Node{NodeID: "disabled", Status: "disabled", Enabled: false, Approved: false}); err != nil {
			t.Fatalf("insert disabled node: %v", err)
		}
	}
	server.cfg.Subscribers = nil
}
