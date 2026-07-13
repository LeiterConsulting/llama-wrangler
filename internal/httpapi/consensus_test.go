package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
	"llama-wrangler/internal/routing"
)

func TestMarshalConsensusFansOutWithinPolicyAndReturnsDeterministicMajority(t *testing.T) {
	server := newConsensusHTTPTestServer(t, 2, 3)
	var mu sync.Mutex
	hits := map[string]int{}
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		hits[r.URL.Host]++
		mu.Unlock()
		var request map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, fmt.Errorf("decode participant request: %w", err)
		}
		if request["model"] != "consensus-model" {
			return nil, fmt.Errorf("participant model = %#v", request["model"])
		}
		switch r.URL.Host {
		case "a.test":
			return consensusHTTPResponse(http.StatusOK, `{"id":"a","choices":[{"message":{"role":"assistant","content":"CONSENSUS_OUTPUT_SECRET"}}]}`), nil
		case "b.test":
			return consensusHTTPResponse(http.StatusOK, `{"id":"b","choices":[{"message":{"role":"assistant","content":"  consensus_output_secret  "}}]}`), nil
		case "c.test":
			return consensusHTTPResponse(http.StatusServiceUnavailable, `{"error":"unavailable"}`), nil
		default:
			return nil, fmt.Errorf("unexpected consensus host %s", r.URL.Host)
		}
	})}

	requestBody := `{"model":"consensus-test","messages":[{"role":"user","content":"SECRET_PROMPT"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"id":"a"`) || !strings.Contains(rr.Body.String(), "CONSENSUS_OUTPUT_SECRET") {
		t.Fatalf("consensus response status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Llama-Wrangler-Consensus") != "" {
		t.Fatalf("consensus metadata should be debug-only: %#v", rr.Header())
	}
	mu.Lock()
	if hits["a.test"] != 1 || hits["b.test"] != 1 || hits["c.test"] != 1 || hits["d.test"] != 0 || len(hits) != 3 {
		t.Fatalf("consensus fan-out hits = %#v", hits)
	}
	mu.Unlock()

	event := consensusAuditEvent(t, server)
	if benchmarkJobInt(event.Fields["participant_count"], 0) != 3 || benchmarkJobInt(event.Fields["successful_count"], 0) != 2 || benchmarkJobInt(event.Fields["failed_count"], 0) != 1 || event.Fields["winner_node"] != "a-managed" || event.Fields["consensus_reached"] != true {
		t.Fatalf("consensus event = %#v", event.Fields)
	}
	if event.Fields["agreement_score"] != 1.0 || event.Fields["comparison_strategy"] != "exact_normalized" || event.Fields["frontier_used"] != false {
		t.Fatalf("consensus agreement metadata = %#v", event.Fields)
	}
	assertConsensusFailureCount(t, event, consensusFailureUpstream5xx, 1)
	assertConsensusParticipantFailure(t, event, "c-managed", consensusFailureUpstream5xx, http.StatusServiceUnavailable)
	rendered := string(mustJSON(t, server.store.Snapshot().Audit)) + string(mustJSON(t, server.buildSupportBundle()))
	for _, forbidden := range []string{"SECRET_PROMPT", "CONSENSUS_OUTPUT_SECRET", "Authorization", "lw_hb_", "lw_admin_", "lw_client_", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("consensus metadata/support bundle leaked %q: %s", forbidden, rendered)
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	req.Header.Set("X-Llama-Wrangler-Debug", "true")
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("X-Llama-Wrangler-Consensus") != "reached" || rr.Header().Get("X-Llama-Wrangler-Consensus-Winner") != "a-managed" || rr.Header().Get("X-Llama-Wrangler-Consensus-Participants") != "2" {
		t.Fatalf("debug consensus headers/status=%d headers=%#v body=%s", rr.Code, rr.Header(), rr.Body.String())
	}
}

func TestMarshalConsensusRejectsStreamingBeforeFanout(t *testing.T) {
	server := newConsensusHTTPTestServer(t, 2, 3)
	hits := 0
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits++
		return consensusHTTPResponse(http.StatusOK, `{"response":"unexpected"}`), nil
	})}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"consensus-test","stream":true,"messages":[{"role":"user","content":"SECRET_PROMPT"}]}`))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), errorCodeConsensusStreamingUnsupported) {
		t.Fatalf("streaming consensus status=%d body=%s", rr.Code, rr.Body.String())
	}
	if hits != 0 {
		t.Fatalf("streaming consensus reached %d participants", hits)
	}
	if strings.Contains(string(mustJSON(t, server.store.Snapshot().Audit)), "SECRET_PROMPT") {
		t.Fatalf("streaming rejection telemetry leaked prompt")
	}
}

func TestMarshalConsensusPreservesOllamaWinnerShape(t *testing.T) {
	server := newConsensusHTTPTestServer(t, 2, 2)
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "a.test":
			return consensusHTTPResponse(http.StatusOK, `{"model":"consensus-model","response":"{\"answer\":42,\"ok\":true}","done":true}`), nil
		case "b.test":
			return consensusHTTPResponse(http.StatusOK, `{"model":"consensus-model","response":"{\"ok\":true,\"answer\":42}","done":true}`), nil
		default:
			return nil, fmt.Errorf("unexpected Ollama consensus host %s", r.URL.Host)
		}
	})}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"model":"consensus-test","prompt":"SECRET_PROMPT","stream":false}`))
	server.marshalProxy("/api/generate", "ollama_generate")(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "application/json" || !strings.Contains(rr.Body.String(), `"response":"{\"answer\":42,\"ok\":true}"`) {
		t.Fatalf("Ollama consensus status=%d headers=%#v body=%s", rr.Code, rr.Header(), rr.Body.String())
	}
	event := consensusAuditEvent(t, server)
	if event.Fields["comparison_strategy"] != "json_structural" || event.Fields["winner_node"] != "a-managed" || event.Fields["consensus_reached"] != true {
		t.Fatalf("Ollama consensus event = %#v", event.Fields)
	}
	rendered := string(mustJSON(t, server.store.Snapshot().Audit))
	if strings.Contains(rendered, "SECRET_PROMPT") || strings.Contains(rendered, `\"answer\":42`) {
		t.Fatalf("Ollama consensus telemetry leaked content: %s", rendered)
	}
}

func TestMarshalConsensusRequiresEnoughSuccessfulParticipants(t *testing.T) {
	server := newConsensusHTTPTestServer(t, 3, 3)
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "a.test" {
			return consensusHTTPResponse(http.StatusOK, `{"id":"a","choices":[{"message":{"content":"only success"}}]}`), nil
		}
		return consensusHTTPResponse(http.StatusServiceUnavailable, `{"error":"unavailable"}`), nil
	})}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"consensus-test","messages":[{"role":"user","content":"SECRET_PROMPT"}]}`))
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusBadGateway || !strings.Contains(rr.Body.String(), errorCodeConsensusInsufficientSuccesses) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("insufficient consensus status=%d body=%s", rr.Code, rr.Body.String())
	}
	event := consensusAuditEvent(t, server)
	if benchmarkJobInt(event.Fields["successful_count"], 0) != 1 || benchmarkJobInt(event.Fields["failed_count"], 0) != 2 || event.Fields["consensus_reached"] != false {
		t.Fatalf("insufficient consensus event = %#v", event.Fields)
	}
	assertConsensusFailureCount(t, event, consensusFailureUpstream5xx, 2)
}

func TestForwardConsensusHonorsCancellationAndTimeout(t *testing.T) {
	server := newConsensusHTTPTestServer(t, 2, 2)
	decision := routing.Decision{
		ResolvedModel:     "consensus-model",
		CandidateNodes:    []string{"a-managed", "b-managed"},
		ConsensusRequired: 2,
		ConsensusLimit:    2,
	}
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	outcome := server.forwardConsensus(ctx, rr, "/v1/chat/completions", "openai_chat_completions", []byte(`{"model":"consensus-test"}`), decision, false)
	if !outcome.ClientCancelled || outcome.Err == nil || outcome.ResponseCommitted || rr.Body.Len() != 0 {
		t.Fatalf("cancelled consensus outcome = %#v body=%s", outcome, rr.Body.String())
	}
	if outcome.FailureReasonCounts[consensusFailureCancellation] != 2 {
		t.Fatalf("cancelled consensus failure reasons = %#v", outcome.FailureReasonCounts)
	}

	server.cfg.Routing.RequestTimeoutSec = 1
	rr = httptest.NewRecorder()
	started := time.Now()
	outcome = server.forwardConsensus(context.Background(), rr, "/v1/chat/completions", "openai_chat_completions", []byte(`{"model":"consensus-test"}`), decision, false)
	if !outcome.TimedOut || outcome.ClientCancelled || outcome.Err == nil || outcome.ResponseCommitted {
		t.Fatalf("timed-out consensus outcome = %#v", outcome)
	}
	if outcome.FailureReasonCounts[consensusFailureTimeout] != 2 {
		t.Fatalf("timed-out consensus failure reasons = %#v", outcome.FailureReasonCounts)
	}
	if elapsed := time.Since(started); elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("consensus timeout elapsed = %s", elapsed)
	}
}

func TestFetchConsensusParticipantClassifiesSafeFailureReasons(t *testing.T) {
	tests := []struct {
		name       string
		nodeID     string
		wantReason string
		wantStatus int
		context    func() (context.Context, context.CancelFunc)
		transport  roundTripFunc
	}{
		{
			name:       "missing proxy URL",
			nodeID:     "missing-node",
			wantReason: consensusFailureMissingProxyURL,
		},
		{
			name:       "connection error",
			nodeID:     "a-managed",
			wantReason: consensusFailureConnectionError,
			transport: func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("SECRET_RESPONSE connection failed")
			},
		},
		{
			name:       "upstream 4xx",
			nodeID:     "a-managed",
			wantReason: consensusFailureUpstream4xx,
			wantStatus: http.StatusTooManyRequests,
			transport: func(r *http.Request) (*http.Response, error) {
				return consensusHTTPResponse(http.StatusTooManyRequests, `{"error":"SECRET_RESPONSE"}`), nil
			},
		},
		{
			name:       "upstream 5xx",
			nodeID:     "a-managed",
			wantReason: consensusFailureUpstream5xx,
			wantStatus: http.StatusServiceUnavailable,
			transport: func(r *http.Request) (*http.Response, error) {
				return consensusHTTPResponse(http.StatusServiceUnavailable, `{"error":"SECRET_RESPONSE"}`), nil
			},
		},
		{
			name:       "body read failure",
			nodeID:     "a-managed",
			wantReason: consensusFailureBodyRead,
			wantStatus: http.StatusOK,
			transport: func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(errReader{err: errors.New("SECRET_RESPONSE body failed")})}, nil
			},
		},
		{
			name:       "response size limit",
			nodeID:     "a-managed",
			wantReason: consensusFailureResponseSize,
			wantStatus: http.StatusOK,
			transport: func(r *http.Request) (*http.Response, error) {
				return consensusHTTPResponse(http.StatusOK, strings.Repeat("x", int(consensusMaxResponseBytes)+1)), nil
			},
		},
		{
			name:       "timeout",
			nodeID:     "a-managed",
			wantReason: consensusFailureTimeout,
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Millisecond)
			},
			transport: func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			},
		},
		{
			name:       "cancellation",
			nodeID:     "a-managed",
			wantReason: consensusFailureCancellation,
			context: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			transport: func(r *http.Request) (*http.Response, error) {
				return nil, r.Context().Err()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newConsensusHTTPTestServer(t, 2, 2)
			if tc.transport != nil {
				server.client = &http.Client{Transport: tc.transport}
			}
			ctx := context.Background()
			cancel := func() {}
			if tc.context != nil {
				ctx, cancel = tc.context()
			}
			defer cancel()
			outcome := server.fetchConsensusParticipant(ctx, tc.nodeID, "/v1/chat/completions", []byte(`{"model":"consensus-test","messages":[{"content":"SECRET_PROMPT"}]}`), "consensus-model")
			if outcome.Err == nil || outcome.FailureReason != tc.wantReason || outcome.StatusCode != tc.wantStatus || len(outcome.Body) != 0 {
				t.Fatalf("participant outcome = %#v, want reason=%s status=%d", outcome, tc.wantReason, tc.wantStatus)
			}
			failure := consensusParticipantFailure(outcome)
			rendered := string(mustJSON(t, failure))
			if failure.ReasonCode != tc.wantReason || strings.Contains(rendered, "SECRET_PROMPT") || strings.Contains(rendered, "SECRET_RESPONSE") || strings.Contains(rendered, "http://") {
				t.Fatalf("unsafe participant failure projection: %s", rendered)
			}
		})
	}
}

func TestMarshalConsensusAllUpstream4xxUsesCompatibilityErrors(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		surface  string
		status   int
		wantCode string
		request  string
	}{
		{name: "OpenAI rate limit", path: "/v1/chat/completions", surface: "openai_chat_completions", status: http.StatusTooManyRequests, wantCode: "upstream_rate_limited", request: `{"model":"consensus-test","messages":[]}`},
		{name: "Ollama invalid request", path: "/api/generate", surface: "ollama_generate", status: http.StatusBadRequest, wantCode: "upstream_invalid_request", request: `{"model":"consensus-test","prompt":""}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newConsensusHTTPTestServer(t, 2, 2)
			server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return consensusHTTPResponse(tc.status, `{"error":"SECRET_RESPONSE"}`), nil
			})}
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.request))
			server.marshalProxy(tc.path, tc.surface)(rr, req)
			if rr.Code != tc.status || !strings.Contains(rr.Body.String(), tc.wantCode) || strings.Contains(rr.Body.String(), "SECRET_RESPONSE") {
				t.Fatalf("compatibility failure status=%d body=%s", rr.Code, rr.Body.String())
			}
			event := consensusAuditEvent(t, server)
			assertConsensusFailureCount(t, event, consensusFailureUpstream4xx, 2)
		})
	}
}

func newConsensusHTTPTestServer(t *testing.T, minParticipants, maxParticipants int) *Server {
	t.Helper()
	cfg := config.Default("marshal")
	cfg.ModelAliases["consensus-test"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"consensus-model"},
		ExecutionMode:   "consensus",
		MinParticipants: minParticipants,
		MaxParticipants: maxParticipants,
	}
	server := newIsolatedTestServerWithConfig(t, cfg)
	now := time.Now().UTC()
	for _, item := range []struct {
		id      string
		url     string
		control string
		trust   string
		fresh   bool
	}{
		{id: "a-managed", url: "http://a.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLocal, fresh: true},
		{id: "b-managed", url: "http://b.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLocal, fresh: true},
		{id: "c-managed", url: "http://c.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLocal, fresh: true},
		{id: "d-managed", url: "http://d.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLANTrusted, fresh: true},
		{id: "passive-never", url: "http://passive.test", control: appstate.ControlLevelPassive, trust: appstate.TrustLevelLANTrusted, fresh: true},
		{id: "unverified-never", url: "http://unverified.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLANUnverified, fresh: true},
		{id: "stale-never", url: "http://stale.test", control: appstate.ControlLevelManaged, trust: appstate.TrustLevelLocal, fresh: false},
	} {
		lastReported := now
		if !item.fresh {
			lastReported = now.Add(-10 * time.Minute)
		}
		if err := server.store.UpsertNode(appstate.Node{
			NodeID:          item.id,
			URL:             item.url,
			ControlLevel:    item.control,
			TrustLevel:      item.trust,
			Enabled:         true,
			Approved:        true,
			ApprovalState:   appstate.ApprovalStateApproved,
			Status:          "healthy",
			OllamaAvailable: true,
			Models:          []appstate.ModelState{{Name: "consensus-model", State: "installed"}},
			LastReportedAt:  &lastReported,
			Observed:        map[string]interface{}{"heartbeat_required": true},
		}); err != nil {
			t.Fatalf("upsert consensus node %s: %v", item.id, err)
		}
	}
	return server
}

func consensusHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func consensusAuditEvent(t *testing.T, server *Server) appstate.AuditEvent {
	t.Helper()
	for _, event := range server.store.Snapshot().Audit {
		if event.Type == "consensus" {
			return event
		}
	}
	t.Fatalf("consensus audit event missing")
	return appstate.AuditEvent{}
}

func assertConsensusFailureCount(t *testing.T, event appstate.AuditEvent, reason string, want int) {
	t.Helper()
	counts, ok := event.Fields["failure_reason_counts"].(map[string]interface{})
	if !ok {
		t.Fatalf("consensus failure counts missing or wrong type: %#v", event.Fields["failure_reason_counts"])
	}
	if got := benchmarkJobInt(counts[reason], 0); got != want {
		t.Fatalf("consensus failure count %s = %d, want %d; counts=%#v", reason, got, want, counts)
	}
}

func assertConsensusParticipantFailure(t *testing.T, event appstate.AuditEvent, nodeID, reason string, status int) {
	t.Helper()
	failures, ok := event.Fields["participant_failures"].([]interface{})
	if !ok {
		t.Fatalf("consensus participant failures missing or wrong type: %#v", event.Fields["participant_failures"])
	}
	for _, raw := range failures {
		failure, ok := raw.(map[string]interface{})
		if !ok || failure["node_id"] != nodeID || failure["reason_code"] != reason {
			continue
		}
		if got := benchmarkJobInt(failure["status_code"], 0); got != status {
			t.Fatalf("participant failure status = %d, want %d: %#v", got, status, failure)
		}
		return
	}
	t.Fatalf("participant failure node=%s reason=%s missing: %#v", nodeID, reason, failures)
}
