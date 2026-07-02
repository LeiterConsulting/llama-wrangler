package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
)

func TestOpenAIStreamingSSEIsReadableBeforeUpstreamCompletes(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstreamFirstFlushed := make(chan struct{})
	allowUpstreamFinish := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriber/proxy/v1/chat/completions" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if payload["model"] != "llama3.1:8b" || payload["stream"] != true {
			t.Fatalf("upstream payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		writeAndFlush(t, w, `data: {"id":"chatcmpl-compat","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hel"},"finish_reason":null}]}`+"\n\n")
		close(upstreamFirstFlushed)
		select {
		case <-allowUpstreamFinish:
		case <-time.After(2 * time.Second):
			t.Fatalf("client did not read first SSE chunk before upstream finish")
		}
		writeAndFlush(t, w, `data: {"id":"chatcmpl-compat","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}`+"\n\n")
		writeAndFlush(t, w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	configureStreamingCompatNode(t, server, upstream.URL)

	mux := http.NewServeMux()
	server.routes(mux)
	api := httptest.NewServer(mux)
	defer api.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.URL+"/v1/chat/completions", strings.NewReader(`{"model":"local-fast","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client().Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}

	reader := bufio.NewReader(resp.Body)
	firstLine := readLineWithTimeout(t, reader)
	if !strings.HasPrefix(firstLine, "data: ") || !strings.Contains(firstLine, "chat.completion.chunk") || !strings.Contains(firstLine, `"content":"hel"`) {
		t.Fatalf("first SSE line = %q", firstLine)
	}
	select {
	case <-upstreamFirstFlushed:
	default:
		t.Fatalf("client read before upstream reported first flush")
	}
	close(allowUpstreamFinish)
	readBlankLineWithTimeout(t, reader)
	secondLine := readLineWithTimeout(t, reader)
	if !strings.Contains(secondLine, `"content":"lo"`) {
		t.Fatalf("second SSE line = %q", secondLine)
	}
	readBlankLineWithTimeout(t, reader)
	doneLine := readLineWithTimeout(t, reader)
	if doneLine != "data: [DONE]\n" {
		t.Fatalf("done SSE line = %q", doneLine)
	}
}

func TestOllamaStreamingJSONLIsReadableBeforeUpstreamCompletes(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstreamFirstFlushed := make(chan struct{})
	allowUpstreamFinish := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriber/proxy/api/chat" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if payload["model"] != "llama3.1:8b" || payload["stream"] != true {
			t.Fatalf("upstream payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		writeAndFlush(t, w, `{"model":"llama3.1:8b","message":{"role":"assistant","content":"hel"},"done":false}`+"\n")
		close(upstreamFirstFlushed)
		select {
		case <-allowUpstreamFinish:
		case <-time.After(2 * time.Second):
			t.Fatalf("client did not read first JSONL chunk before upstream finish")
		}
		writeAndFlush(t, w, `{"model":"llama3.1:8b","message":{"role":"assistant","content":"lo"},"done":false}`+"\n")
		writeAndFlush(t, w, `{"model":"llama3.1:8b","done":true}`+"\n")
	}))
	defer upstream.Close()
	configureStreamingCompatNode(t, server, upstream.URL)

	mux := http.NewServeMux()
	server.routes(mux)
	api := httptest.NewServer(mux)
	defer api.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.URL+"/api/chat", strings.NewReader(`{"model":"local-fast","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client().Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		t.Fatalf("content-type = %q, want application/x-ndjson", got)
	}

	reader := bufio.NewReader(resp.Body)
	firstLine := readLineWithTimeout(t, reader)
	var first map[string]interface{}
	if err := json.Unmarshal([]byte(firstLine), &first); err != nil {
		t.Fatalf("decode first JSONL line %q: %v", firstLine, err)
	}
	message, ok := first["message"].(map[string]interface{})
	if !ok || message["content"] != "hel" || first["done"] != false {
		t.Fatalf("first JSONL object = %#v", first)
	}
	select {
	case <-upstreamFirstFlushed:
	default:
		t.Fatalf("client read before upstream reported first flush")
	}
	close(allowUpstreamFinish)
	secondLine := readLineWithTimeout(t, reader)
	if !strings.Contains(secondLine, `"content":"lo"`) {
		t.Fatalf("second JSONL line = %q", secondLine)
	}
	doneLine := readLineWithTimeout(t, reader)
	var done map[string]interface{}
	if err := json.Unmarshal([]byte(doneLine), &done); err != nil {
		t.Fatalf("decode done JSONL line %q: %v", doneLine, err)
	}
	if done["done"] != true {
		t.Fatalf("done JSONL object = %#v", done)
	}
}

func configureStreamingCompatNode(t *testing.T, server *Server, upstreamURL string) {
	t.Helper()
	state := server.store.Snapshot()
	for _, node := range state.Nodes {
		node.Enabled = false
		node.Approved = false
		node.Status = "disabled"
		if err := server.store.UpsertNode(node); err != nil {
			t.Fatalf("disable existing node: %v", err)
		}
	}
	node := appstate.Node{
		NodeID:          "streaming-client-node",
		DisplayName:     "Streaming Client Node",
		URL:             upstreamURL,
		Role:            "subscriber",
		Status:          "healthy",
		Enabled:         true,
		Approved:        true,
		OllamaAvailable: true,
	}
	if err := server.store.UpsertNode(node); err != nil {
		t.Fatalf("upsert streaming node: %v", err)
	}
}

func writeAndFlush(t *testing.T, w http.ResponseWriter, chunk string) {
	t.Helper()
	if _, err := w.Write([]byte(chunk)); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatalf("response writer does not support flush")
	}
	flusher.Flush()
}

func readLineWithTimeout(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("read line: %v", got.err)
		}
		return got.line
	case <-time.After(time.Second):
		t.Fatalf("timed out reading stream line")
		return ""
	}
}

func readBlankLineWithTimeout(t *testing.T, reader *bufio.Reader) {
	t.Helper()
	if line := readLineWithTimeout(t, reader); line != "\n" {
		t.Fatalf("blank SSE line = %q", line)
	}
}
