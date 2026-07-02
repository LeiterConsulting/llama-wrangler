package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/config"
)

func TestQueueTrackerSnapshotAndRecent(t *testing.T) {
	tracker := newQueueTracker(2)
	entry := tracker.enqueue(QueueEntry{
		RequestID: "req_queue",
		Priority:  "urgent",
		Surface:   "openai_chat_completions",
	})
	if entry.Priority != queuePriorityNormal {
		t.Fatalf("priority = %q, want normalized default", entry.Priority)
	}
	entry = tracker.start("req_queue", 1)
	if entry.Status != queueStatusActive || !entry.RetryAllowed {
		t.Fatalf("started entry = %+v", entry)
	}
	entry = tracker.update("req_queue", QueueEntry{
		Priority:  queuePriorityHigh,
		Model:     "local-code",
		Stream:    true,
		SessionID: "session-a",
	})
	if entry.Priority != queuePriorityHigh || entry.Model != "local-code" || !entry.Stream {
		t.Fatalf("updated entry = %+v", entry)
	}

	snapshot := tracker.snapshot(1, QueueSchedulingStatus{Policy: queueSchedulingWeightedPriority, Weights: config.QueuePriorityWeights{High: 3, Normal: 2, Low: 1}})
	if snapshot.MaxDepth != 2 || snapshot.Active != 1 || snapshot.Available != 1 || len(snapshot.Current) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if snapshot.Scheduling.Policy != queueSchedulingWeightedPriority || snapshot.Scheduling.Weights.High != 3 {
		t.Fatalf("scheduling = %+v", snapshot.Scheduling)
	}
	if snapshot.Current[0].Model != "local-code" || snapshot.Current[0].SessionID != "session-a" {
		t.Fatalf("current entry = %+v", snapshot.Current[0])
	}

	finished := tracker.finish("req_queue", queueStatusCompleted, 0)
	if finished.Status != queueStatusCompleted {
		t.Fatalf("finished entry = %+v", finished)
	}
	snapshot = tracker.snapshot(0, QueueSchedulingStatus{Policy: queueSchedulingWeightedPriority, Weights: config.QueuePriorityWeights{High: 3, Normal: 2, Low: 1}})
	if len(snapshot.Current) != 0 || len(snapshot.Recent) != 1 {
		t.Fatalf("snapshot after finish = %+v", snapshot)
	}
	if snapshot.Recent[0].Priority != queuePriorityHigh || snapshot.Recent[0].Status != queueStatusCompleted {
		t.Fatalf("recent entry = %+v", snapshot.Recent[0])
	}
}

func TestQueueSchedulerDispatchesWeightedPriorityAheadOfFIFO(t *testing.T) {
	scheduler := newQueueScheduler(1, config.RoutingConfig{
		QueueSchedulingPolicy: queueSchedulingWeightedPriority,
		QueuePriorityWeights:  config.QueuePriorityWeights{High: 2, Normal: 1, Low: 1},
	})
	if !scheduler.acquire(context.Background(), "active", queuePriorityNormal) {
		t.Fatalf("initial acquire failed")
	}

	acquired := make(chan string, 3)
	startWaitingAcquire := func(requestID, priority string) {
		t.Helper()
		go func() {
			if scheduler.acquire(context.Background(), requestID, priority) {
				acquired <- requestID
			}
		}()
	}
	startWaitingAcquire("low", queuePriorityLow)
	startWaitingAcquire("normal", queuePriorityNormal)
	startWaitingAcquire("high", queuePriorityHigh)
	waitForSchedulerWaiting(t, scheduler, 3)

	scheduler.release()
	if got := readAcquiredRequest(t, acquired); got != "high" {
		t.Fatalf("first dispatched = %q, want high", got)
	}
	scheduler.release()
	if got := readAcquiredRequest(t, acquired); got != "normal" {
		t.Fatalf("second dispatched = %q, want normal", got)
	}
	scheduler.release()
	if got := readAcquiredRequest(t, acquired); got != "low" {
		t.Fatalf("third dispatched = %q, want low", got)
	}
}

func TestQueueSchedulerFIFOPolicyIgnoresPriorityWeights(t *testing.T) {
	scheduler := newQueueScheduler(1, config.RoutingConfig{
		QueueSchedulingPolicy: queueSchedulingFIFO,
		QueuePriorityWeights:  config.QueuePriorityWeights{High: 10, Normal: 1, Low: 1},
	})
	if !scheduler.acquire(context.Background(), "active", queuePriorityNormal) {
		t.Fatalf("initial acquire failed")
	}
	acquired := make(chan string, 2)
	go func() {
		if scheduler.acquire(context.Background(), "low", queuePriorityLow) {
			acquired <- "low"
		}
	}()
	waitForSchedulerWaiting(t, scheduler, 1)
	go func() {
		if scheduler.acquire(context.Background(), "high", queuePriorityHigh) {
			acquired <- "high"
		}
	}()
	waitForSchedulerWaiting(t, scheduler, 2)
	scheduler.release()
	if got := readAcquiredRequest(t, acquired); got != "low" {
		t.Fatalf("first FIFO dispatched = %q, want low", got)
	}
}

func TestParseRequestPriority(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("X-Llama-Wrangler-Priority", "high")
	if got := parseRequestPriority([]byte(`{"queue_priority":"low"}`), req); got != queuePriorityHigh {
		t.Fatalf("header priority = %q, want high", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if got := parseRequestPriority([]byte(`{"priority":"low"}`), req); got != queuePriorityLow {
		t.Fatalf("body priority = %q, want low", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if got := parseRequestPriority([]byte(`{"priority":"urgent"}`), req); got != queuePriorityNormal {
		t.Fatalf("unknown priority = %q, want normal", got)
	}
}

func TestBootstrapIncludesQueueSnapshot(t *testing.T) {
	server := newIsolatedTestServer(t)
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
	queue, ok := body["queue"].(map[string]interface{})
	if !ok {
		t.Fatalf("bootstrap queue missing or wrong type: %#v", body["queue"])
	}
	if queue["max_depth"].(float64) <= 0 {
		t.Fatalf("queue max_depth = %#v", queue["max_depth"])
	}
	if _, ok := queue["current"].([]interface{}); !ok {
		t.Fatalf("queue current missing: %#v", queue["current"])
	}
	if _, ok := queue["recent"].([]interface{}); !ok {
		t.Fatalf("queue recent missing: %#v", queue["recent"])
	}
	scheduling, ok := queue["scheduling"].(map[string]interface{})
	if !ok {
		t.Fatalf("queue scheduling missing: %#v", queue["scheduling"])
	}
	if scheduling["policy"] != queueSchedulingWeightedPriority {
		t.Fatalf("queue scheduling policy = %#v", scheduling["policy"])
	}
}

func TestPutRoutingPoliciesUpdatesQueueScheduling(t *testing.T) {
	server := newIsolatedTestServer(t)
	body := strings.NewReader(`{
		"default_model_alias":"local-fast",
		"default_execution_mode":"single",
		"allow_fallback":true,
		"request_timeout_seconds":300,
		"queue_max_depth":4,
		"queue_scheduling_policy":"fifo",
		"queue_priority_weights":{"high":5,"normal":3,"low":1}
	}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/wrangler/routing/policies", body)
	server.putRoutingPolicies(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put routing status = %d body = %s", rr.Code, rr.Body.String())
	}
	snapshot := server.queueSnapshot()
	if snapshot.MaxDepth != 4 || snapshot.Scheduling.Policy != queueSchedulingFIFO {
		t.Fatalf("queue snapshot = %+v", snapshot)
	}
	if snapshot.Scheduling.Weights.High != 5 || snapshot.Scheduling.Weights.Normal != 3 || snapshot.Scheduling.Weights.Low != 1 {
		t.Fatalf("weights = %+v", snapshot.Scheduling.Weights)
	}
}

func TestMarshalProxyRecordsQueuePriorityMetadata(t *testing.T) {
	server := newIsolatedTestServer(t)
	configureProxyTestNodes(server)
	server.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return proxyResponse(http.StatusOK, `{"ok":true}`), nil
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-fast","stream":false}`))
	req.Header.Set("X-Llama-Wrangler-Priority", "high")
	server.marshalProxy("/v1/chat/completions", "openai_chat_completions")(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("marshal status = %d body = %s", rr.Code, rr.Body.String())
	}

	snapshot := server.queueSnapshot()
	if len(snapshot.Recent) == 0 {
		t.Fatalf("queue recent was empty")
	}
	entry := snapshot.Recent[0]
	if entry.Priority != queuePriorityHigh || entry.Surface != "openai_chat_completions" || entry.Model != "local-fast" || entry.Status != queueStatusCompleted {
		t.Fatalf("recent queue entry = %+v", entry)
	}
	requireAuditEvent(t, server, "queue_state", entry.RequestID, map[string]interface{}{
		"status":          queueStatusCompleted,
		"priority":        queuePriorityHigh,
		"surface":         "openai_chat_completions",
		"model_requested": "local-fast",
	})
}

func waitForSchedulerWaiting(t *testing.T, scheduler *queueScheduler, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scheduler.mu.Lock()
		got := len(scheduler.waiting)
		scheduler.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	scheduler.mu.Lock()
	got := len(scheduler.waiting)
	scheduler.mu.Unlock()
	t.Fatalf("scheduler waiting = %d, want %d", got, want)
}

func readAcquiredRequest(t *testing.T, acquired <-chan string) string {
	t.Helper()
	select {
	case requestID := <-acquired:
		return requestID
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for acquired request")
		return ""
	}
}
