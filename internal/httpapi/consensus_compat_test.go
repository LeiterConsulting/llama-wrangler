package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

type consensusRealUpstreamSpec struct {
	status        int
	contentType   string
	body          string
	waitForCancel bool
}

type consensusRealUpstreamObservation struct {
	mu     sync.Mutex
	hits   int
	paths  []string
	bodies []string
}

func (o *consensusRealUpstreamObservation) record(path, body string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.hits++
	o.paths = append(o.paths, path)
	o.bodies = append(o.bodies, body)
}

func (o *consensusRealUpstreamObservation) snapshot() (int, []string, []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hits, append([]string(nil), o.paths...), append([]string(nil), o.bodies...)
}

func TestConsensusRealClientsPreservePartialSuccessAndDeterministicWinner(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		requestBody    string
		winnerBody     string
		peerBody       string
		responseMarker string
		assertBody     func(*testing.T, []byte)
	}{
		{
			name:           "OpenAI chat completion",
			path:           "/v1/chat/completions",
			requestBody:    `{"model":"consensus-test","messages":[{"role":"user","content":"REAL_CLIENT_PROMPT_OPENAI"}],"stream":false}`,
			winnerBody:     `{"id":"openai-winner-a","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"REAL_CLIENT_RESPONSE_OPENAI"},"finish_reason":"stop"}]}`,
			peerBody:       `{"id":"openai-peer-b","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"  real_client_response_openai  "},"finish_reason":"stop"}]}`,
			responseMarker: "REAL_CLIENT_RESPONSE_OPENAI",
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				var response struct {
					ID      string `json:"id"`
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("decode OpenAI winner: %v; body=%s", err, body)
				}
				if response.ID != "openai-winner-a" || len(response.Choices) != 1 || response.Choices[0].Message.Content != "REAL_CLIENT_RESPONSE_OPENAI" {
					t.Fatalf("OpenAI winner shape = %#v", response)
				}
			},
		},
		{
			name:           "Ollama generate",
			path:           "/api/generate",
			requestBody:    `{"model":"consensus-test","prompt":"REAL_CLIENT_PROMPT_OLLAMA","stream":false}`,
			winnerBody:     `{"model":"consensus-model","response":"{\"answer\":42,\"marker\":\"REAL_CLIENT_RESPONSE_OLLAMA\"}","done":true,"done_reason":"stop"}`,
			peerBody:       `{"model":"consensus-model","response":"{\"marker\":\"REAL_CLIENT_RESPONSE_OLLAMA\",\"answer\":42}","done":true,"done_reason":"stop"}`,
			responseMarker: "REAL_CLIENT_RESPONSE_OLLAMA",
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				var response struct {
					Model      string `json:"model"`
					Response   string `json:"response"`
					Done       bool   `json:"done"`
					DoneReason string `json:"done_reason"`
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("decode Ollama winner: %v; body=%s", err, body)
				}
				if response.Model != "consensus-model" || response.Response != `{"answer":42,"marker":"REAL_CLIENT_RESPONSE_OLLAMA"}` || !response.Done || response.DoneReason != "stop" {
					t.Fatalf("Ollama winner shape = %#v", response)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, api, observations := newConsensusRealClientHarness(t, 2, 3, 5, []consensusRealUpstreamSpec{
				{status: http.StatusOK, contentType: "application/json; charset=utf-8", body: tc.winnerBody},
				{status: http.StatusOK, contentType: "application/json", body: tc.peerBody},
				{status: http.StatusServiceUnavailable, contentType: "application/json", body: `{"error":"REAL_CLIENT_UPSTREAM_ERROR"}`},
			})
			status, header, body := doConsensusRealClientRequest(t, api, tc.path, tc.requestBody, 3*time.Second)
			if status != http.StatusOK || header.Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("winner status=%d content-type=%q body=%s", status, header.Get("Content-Type"), body)
			}
			tc.assertBody(t, body)
			if !strings.Contains(string(body), tc.responseMarker) || strings.Contains(string(body), "REAL_CLIENT_UPSTREAM_ERROR") {
				t.Fatalf("unexpected winner body: %s", body)
			}
			for index, observation := range observations {
				hits, paths, bodies := observation.snapshot()
				if hits != 1 || len(paths) != 1 || paths[0] != "/subscriber/proxy"+tc.path || len(bodies) != 1 || !strings.Contains(bodies[0], `"model":"consensus-model"`) {
					t.Fatalf("upstream %d observation hits=%d paths=%#v bodies=%#v", index, hits, paths, bodies)
				}
			}
			assertConsensusRealClientPayloadExclusion(t, server, api, []string{
				"REAL_CLIENT_PROMPT_OPENAI",
				"REAL_CLIENT_PROMPT_OLLAMA",
				"REAL_CLIENT_RESPONSE_OPENAI",
				"REAL_CLIENT_RESPONSE_OLLAMA",
				"REAL_CLIENT_UPSTREAM_ERROR",
			})
		})
	}
}

func TestConsensusRealClientsNormalizeAllUpstream4xx(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requestBody string
		status      int
		code        string
		errorType   string
		openAI      bool
	}{
		{name: "OpenAI rate limit", path: "/v1/chat/completions", requestBody: `{"model":"consensus-test","messages":[]}`, status: http.StatusTooManyRequests, code: "upstream_rate_limited", errorType: "rate_limit_error", openAI: true},
		{name: "Ollama invalid request", path: "/api/generate", requestBody: `{"model":"consensus-test","prompt":"","stream":false}`, status: http.StatusBadRequest, code: "upstream_invalid_request", errorType: "invalid_request_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, api, _ := newConsensusRealClientHarness(t, 2, 2, 5, []consensusRealUpstreamSpec{
				{status: tc.status, contentType: "application/json", body: `{"error":"REAL_CLIENT_4XX_BODY"}`},
				{status: tc.status, contentType: "application/json", body: `{"error":"REAL_CLIENT_4XX_BODY"}`},
			})
			status, _, body := doConsensusRealClientRequest(t, api, tc.path, tc.requestBody, 3*time.Second)
			if status != tc.status || strings.Contains(string(body), "REAL_CLIENT_4XX_BODY") {
				t.Fatalf("4xx compatibility status=%d body=%s", status, body)
			}
			assertConsensusCompatibilityError(t, body, tc.code, tc.errorType, tc.openAI)
			assertConsensusRealClientPayloadExclusion(t, server, api, []string{"REAL_CLIENT_4XX_BODY"})
		})
	}
}

func TestConsensusRealClientsReturnCompatibilityQuorumErrors(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requestBody string
		openAI      bool
	}{
		{name: "OpenAI", path: "/v1/chat/completions", requestBody: `{"model":"consensus-test","messages":[{"role":"user","content":"REAL_CLIENT_QUORUM_PROMPT"}]}`, openAI: true},
		{name: "Ollama", path: "/api/generate", requestBody: `{"model":"consensus-test","prompt":"REAL_CLIENT_QUORUM_PROMPT","stream":false}`},
	}

	for _, tc := range tests {
		t.Run(tc.name+" immediate quorum failure", func(t *testing.T) {
			server, api, _ := newConsensusRealClientHarness(t, 2, 2, 5, []consensusRealUpstreamSpec{
				{status: http.StatusOK, contentType: "application/json", body: consensusSuccessBody(tc.openAI, "only-success", "REAL_CLIENT_PARTIAL_RESPONSE")},
				{status: http.StatusServiceUnavailable, contentType: "application/json", body: `{"error":"REAL_CLIENT_QUORUM_ERROR"}`},
			})
			status, _, body := doConsensusRealClientRequest(t, api, tc.path, tc.requestBody, 3*time.Second)
			if status != http.StatusBadGateway || strings.Contains(string(body), "REAL_CLIENT_PARTIAL_RESPONSE") || strings.Contains(string(body), "REAL_CLIENT_QUORUM_ERROR") {
				t.Fatalf("quorum failure status=%d body=%s", status, body)
			}
			assertConsensusCompatibilityError(t, body, errorCodeConsensusInsufficientSuccesses, "server_error", tc.openAI)
			assertConsensusRealClientPayloadExclusion(t, server, api, []string{"REAL_CLIENT_QUORUM_PROMPT", "REAL_CLIENT_PARTIAL_RESPONSE", "REAL_CLIENT_QUORUM_ERROR"})
		})

		t.Run(tc.name+" timeout quorum failure", func(t *testing.T) {
			server, api, _ := newConsensusRealClientHarness(t, 2, 2, 1, []consensusRealUpstreamSpec{
				{waitForCancel: true},
				{waitForCancel: true},
			})
			started := time.Now()
			status, _, body := doConsensusRealClientRequest(t, api, tc.path, tc.requestBody, 3*time.Second)
			if elapsed := time.Since(started); status != http.StatusGatewayTimeout || elapsed < 900*time.Millisecond || elapsed > 2500*time.Millisecond {
				t.Fatalf("timeout status=%d elapsed=%s body=%s", status, elapsed, body)
			}
			assertConsensusCompatibilityError(t, body, errorCodeConsensusInsufficientSuccesses, "server_error", tc.openAI)
			assertConsensusRealClientPayloadExclusion(t, server, api, []string{"REAL_CLIENT_QUORUM_PROMPT"})
		})
	}
}

func TestConsensusRealClientsRejectStreamingBeforeFanout(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requestBody string
		openAI      bool
	}{
		{name: "OpenAI", path: "/v1/chat/completions", requestBody: `{"model":"consensus-test","stream":true,"messages":[{"role":"user","content":"REAL_CLIENT_STREAM_PROMPT"}]}`, openAI: true},
		{name: "Ollama", path: "/api/generate", requestBody: `{"model":"consensus-test","stream":true,"prompt":"REAL_CLIENT_STREAM_PROMPT"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, api, observations := newConsensusRealClientHarness(t, 2, 2, 5, []consensusRealUpstreamSpec{{status: http.StatusOK}, {status: http.StatusOK}})
			status, _, body := doConsensusRealClientRequest(t, api, tc.path, tc.requestBody, 3*time.Second)
			if status != http.StatusBadRequest {
				t.Fatalf("streaming rejection status=%d body=%s", status, body)
			}
			assertConsensusCompatibilityError(t, body, errorCodeConsensusStreamingUnsupported, "invalid_request_error", tc.openAI)
			for index, observation := range observations {
				if hits, _, _ := observation.snapshot(); hits != 0 {
					t.Fatalf("streaming rejection reached upstream %d %d times", index, hits)
				}
			}
			assertConsensusRealClientPayloadExclusion(t, server, api, []string{"REAL_CLIENT_STREAM_PROMPT"})
		})
	}
}

func newConsensusRealClientHarness(t *testing.T, minParticipants, maxParticipants, timeoutSeconds int, specs []consensusRealUpstreamSpec) (*Server, *httptest.Server, []*consensusRealUpstreamObservation) {
	t.Helper()
	cfg := config.Default("marshal")
	cfg.Routing.RequestTimeoutSec = timeoutSeconds
	cfg.ModelAliases["consensus-test"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"consensus-model"},
		ExecutionMode:   "consensus",
		MinParticipants: minParticipants,
		MaxParticipants: maxParticipants,
	}
	server := newIsolatedTestServerWithConfig(t, cfg)
	for _, node := range server.store.Snapshot().Nodes {
		node.Enabled = false
		node.Approved = false
		node.ApprovalState = appstate.ApprovalStateRevoked
		node.Status = "disabled"
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("disable existing node: %v", err)
		}
	}

	observations := make([]*consensusRealUpstreamObservation, 0, len(specs))
	now := time.Now().UTC()
	for index, spec := range specs {
		observation := &consensusRealUpstreamObservation{}
		observations = append(observations, observation)
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			observation.record(r.URL.Path, string(body))
			if spec.waitForCancel {
				<-r.Context().Done()
				return
			}
			if spec.contentType != "" {
				w.Header().Set("Content-Type", spec.contentType)
			}
			status := spec.status
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, spec.body)
		}))
		t.Cleanup(upstream.Close)
		nodeID := string(rune('a'+index)) + "-real-client-managed"
		if err := server.store.UpsertNode(appstate.Node{
			NodeID:           nodeID,
			DisplayName:      nodeID,
			URL:              upstream.URL,
			Role:             "subscriber",
			ControlLevel:     appstate.ControlLevelManaged,
			TrustLevel:       appstate.TrustLevelLocal,
			CapabilitySource: appstate.CapabilitySourceSubscriberReported,
			ApprovalState:    appstate.ApprovalStateApproved,
			Enabled:          true,
			Approved:         true,
			Status:           "healthy",
			OllamaAvailable:  true,
			Models:           []appstate.ModelState{{Name: "consensus-model", State: "installed"}},
			LastReportedAt:   &now,
			Observed:         map[string]interface{}{"heartbeat_required": true},
		}); err != nil {
			t.Fatalf("upsert real-client node %s: %v", nodeID, err)
		}
	}
	mux := http.NewServeMux()
	server.routes(mux)
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	return server, api, observations
}

func doConsensusRealClientRequest(t *testing.T, api *httptest.Server, path, requestBody string, timeout time.Duration) (int, http.Header, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.URL+path, strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("create consensus client request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client().Do(req)
	if err != nil {
		t.Fatalf("execute consensus client request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read consensus client response: %v", err)
	}
	return resp.StatusCode, resp.Header.Clone(), body
}

func assertConsensusCompatibilityError(t *testing.T, body []byte, code, errorType string, openAI bool) {
	t.Helper()
	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode compatibility error: %v; body=%s", err, body)
	}
	if openAI {
		errorObject, ok := decoded["error"].(map[string]interface{})
		if !ok || errorObject["code"] != code || errorObject["type"] != errorType || errorObject["message"] == "" {
			t.Fatalf("OpenAI compatibility error = %#v", decoded)
		}
		return
	}
	if decoded["code"] != code || decoded["type"] != errorType || decoded["error"] == "" {
		t.Fatalf("Ollama compatibility error = %#v", decoded)
	}
}

func assertConsensusRealClientPayloadExclusion(t *testing.T, server *Server, api *httptest.Server, forbidden []string) {
	t.Helper()
	artifacts := [][]byte{mustJSON(t, server.store.Snapshot().Audit), mustJSON(t, server.buildSupportBundle())}
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/wrangler/ui/bootstrap"},
		{method: http.MethodGet, path: "/wrangler/metrics"},
		{method: http.MethodPost, path: "/wrangler/support-bundle/export"},
	} {
		req, err := http.NewRequest(endpoint.method, api.URL+endpoint.path, nil)
		if err != nil {
			t.Fatalf("create management request: %v", err)
		}
		resp, err := api.Client().Do(req)
		if err != nil {
			t.Fatalf("request management endpoint %s: %v", endpoint.path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("management endpoint %s status=%d read_err=%v body=%s", endpoint.path, resp.StatusCode, readErr, body)
		}
		artifacts = append(artifacts, body)
	}
	for _, artifact := range artifacts {
		for _, marker := range forbidden {
			if strings.Contains(string(artifact), marker) {
				t.Fatalf("consensus payload marker %q leaked into metadata artifact: %s", marker, artifact)
			}
		}
	}
}

func consensusSuccessBody(openAI bool, id, marker string) string {
	if openAI {
		return `{"id":"` + id + `","choices":[{"message":{"role":"assistant","content":"` + marker + `"}}]}`
	}
	return `{"model":"consensus-model","response":"` + marker + `","done":true}`
}
