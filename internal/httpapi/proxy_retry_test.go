package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"llama-wrangler/internal/config"
	"llama-wrangler/internal/routing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

type partialErrReader struct {
	sent bool
}

func (r *partialErrReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("upstream closed after partial output")
	}
	r.sent = true
	return copy(p, "data: partial\n\n"), nil
}

func proxyTestDecision() routing.Decision {
	return routing.Decision{
		ResolvedModel:  "llama3.1:8b",
		SelectedNode:   "primary",
		FallbackNodes:  []string{"backup"},
		CandidateNodes: []string{"primary", "backup"},
	}
}

func configureProxyTestNodes(server *Server) {
	server.cfg.Subscribers = []config.SubscriberConfig{
		{NodeID: "primary", URL: "http://primary.test"},
		{NodeID: "backup", URL: "http://backup.test"},
	}
}

func TestForwardRetriesStatusBeforeFirstToken(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)

	var primaryHits atomic.Int32
	var backupHits atomic.Int32
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "primary.test":
			primaryHits.Add(1)
			return proxyResponse(http.StatusServiceUnavailable, "primary unavailable"), nil
		case "backup.test":
			backupHits.Add(1)
			return proxyResponse(http.StatusOK, "data: backup\n\n"), nil
		default:
			t.Fatalf("unexpected host %s", r.URL.Host)
			return nil, nil
		}
	})}

	rr := httptest.NewRecorder()
	outcome := server.forwardWithFallback(context.Background(), rr, "req_retry_status", "/v1/chat/completions", []byte(`{"model":"local-fast","stream":true}`), proxyTestDecision(), true)
	if outcome.Err != nil {
		t.Fatalf("forwardWithFallback() error = %v", outcome.Err)
	}
	if outcome.SelectedNode != "backup" || !outcome.FallbackUsed || outcome.RetryCount != 1 {
		t.Fatalf("outcome = %+v, want backup fallback retry", outcome)
	}
	if primaryHits.Load() != 1 || backupHits.Load() != 1 {
		t.Fatalf("hits primary=%d backup=%d, want 1/1", primaryHits.Load(), backupHits.Load())
	}
	if got := rr.Body.String(); got != "data: backup\n\n" {
		t.Fatalf("body = %q", got)
	}
	requireAuditEvent(t, server, "upstream_retry", "req_retry_status", map[string]interface{}{
		"previous_node":  "primary",
		"next_node":      "backup",
		"reason":         "upstream_status_5xx",
		"retry_phase":    "before_first_token",
		"retry_allowed":  true,
		"partial_output": false,
	})
}

func TestForwardRetriesBodyReadFailureBeforeFirstToken(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)

	var backupHits atomic.Int32
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "backup.test" {
			backupHits.Add(1)
			return proxyResponse(http.StatusOK, "data: recovered\n\n"), nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(errReader{err: errors.New("upstream closed before first token")}),
		}, nil
	})}

	rr := httptest.NewRecorder()
	outcome := server.forwardWithFallback(context.Background(), rr, "req_retry_body", "/v1/chat/completions", []byte(`{"model":"local-fast","stream":true}`), proxyTestDecision(), true)
	if outcome.Err != nil {
		t.Fatalf("forwardWithFallback() error = %v", outcome.Err)
	}
	if backupHits.Load() != 1 || outcome.SelectedNode != "backup" {
		t.Fatalf("backup hits=%d outcome=%+v, want backup retry", backupHits.Load(), outcome)
	}
	if got := rr.Body.String(); got != "data: recovered\n\n" {
		t.Fatalf("body = %q", got)
	}
	requireAuditEvent(t, server, "upstream_retry", "req_retry_body", map[string]interface{}{
		"reason":         "upstream_body_read_error",
		"retry_phase":    "before_first_token",
		"retry_allowed":  true,
		"partial_output": false,
	})
}

func TestForwardDoesNotRetryAfterPartialStreamingOutput(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)

	var backupHits atomic.Int32
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "backup.test" {
			backupHits.Add(1)
			return proxyResponse(http.StatusOK, "data: backup\n\n"), nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(&partialErrReader{}),
		}, nil
	})}

	rr := httptest.NewRecorder()
	outcome := server.forwardWithFallback(context.Background(), rr, "req_partial", "/v1/chat/completions", []byte(`{"model":"local-fast","stream":true}`), proxyTestDecision(), true)
	if outcome.Err == nil {
		t.Fatalf("forwardWithFallback() error = nil, want partial-output error")
	}
	if !outcome.ResponseCommitted || !outcome.PartialOutput {
		t.Fatalf("outcome = %+v, want committed partial output", outcome)
	}
	if backupHits.Load() != 0 {
		t.Fatalf("backup hits = %d, want no retry after partial output", backupHits.Load())
	}
	if got := rr.Body.String(); got != "data: partial\n\n" {
		t.Fatalf("body = %q", got)
	}
	requireAuditEvent(t, server, "response_partial", "req_partial", map[string]interface{}{
		"selected_node": "primary",
		"retry_phase":   "after_partial_output",
		"retry_allowed": false,
		"bytes_written": float64(len("data: partial\n\n")),
	})
}

func TestForwardEmitsCancellationTelemetryBeforeFirstToken(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)
	server.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	outcome := server.forwardWithFallback(ctx, rr, "req_cancel", "/v1/chat/completions", []byte(`{"model":"local-fast","stream":true}`), proxyTestDecision(), true)
	if outcome.Err == nil || !outcome.ClientCancelled {
		t.Fatalf("outcome = %+v, want client cancellation", outcome)
	}
	requireAuditEvent(t, server, "request_cancelled", "req_cancel", map[string]interface{}{
		"reason":         "client_cancelled_before_first_token",
		"retry_allowed":  false,
		"partial_output": false,
	})
}

func proxyResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func requireAuditEvent(t *testing.T, server *Server, eventType, requestID string, fields map[string]interface{}) {
	t.Helper()
	for _, event := range server.store.Snapshot().Audit {
		if event.Type != eventType || event.RequestID != requestID {
			continue
		}
		matched := true
		for key, want := range fields {
			if got := event.Fields[key]; got != want {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("missing audit event %s request_id=%s fields=%v", eventType, requestID, fields)
}
