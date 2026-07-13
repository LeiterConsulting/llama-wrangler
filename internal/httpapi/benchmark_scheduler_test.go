package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

func TestBenchmarkJobSchedulerLeaseRetryAndExhaustion(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_scheduler_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-scheduler",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-scheduler"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-scheduler/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	var queued struct {
		Job map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatalf("decode queued job: %v", err)
	}
	benchmarkID := benchmarkJobString(queued.Job["benchmark_id"])
	if queued.Job["scheduler_policy"] != benchmarkJobSchedulerPolicy || benchmarkJobInt(queued.Job["max_attempts"], 0) != config.BenchmarkSchedulerDefaultMaxAttempts {
		t.Fatalf("queued scheduler metadata = %#v", queued.Job)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-scheduler"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("claim status = %d body = %s", rr.Code, rr.Body.String())
	}
	var claimed struct {
		Job map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &claimed); err != nil {
		t.Fatalf("decode claimed job: %v", err)
	}
	if claimed.Job["status"] != "running" || benchmarkJobInt(claimed.Job["attempt"], 0) != 1 {
		t.Fatalf("claimed scheduler metadata = %#v", claimed.Job)
	}
	if _, ok := benchmarkJobTime(claimed.Job["timeout_at"]); !ok {
		t.Fatalf("claimed job missing timeout: %#v", claimed.Job)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/status", bytes.NewBufferString(`{"node_id":"managed-scheduler","benchmark_id":"`+benchmarkID+`","status":"failed","error_code":"temporary_runner_error"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("failed status = %d body = %s", rr.Code, rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["managed-scheduler"]
	job, ok := findBenchmarkJob(node, benchmarkID)
	if !ok {
		t.Fatalf("job missing after failure")
	}
	if job["status"] != "failed" || job["scheduler_state"] != "retry_wait" || job["scheduler_reason"] != "retry_after_failure" {
		t.Fatalf("failed retry metadata = %#v", job)
	}
	if _, ok := benchmarkJobTime(job["next_attempt_at"]); !ok {
		t.Fatalf("failed job missing next attempt: %#v", job)
	}

	run := server.reconcileBenchmarkJobs(time.Now().UTC().Add(time.Duration(config.BenchmarkSchedulerDefaultRetryDelaySeconds)*time.Second+time.Second), "test_retry_due")
	if !run.Changed || run.Retried == 0 {
		t.Fatalf("retry reconcile = %#v", run)
	}
	node = server.store.Snapshot().Nodes["managed-scheduler"]
	job, _ = findBenchmarkJob(node, benchmarkID)
	if job["status"] != "queued" || job["scheduler_state"] != "queued" {
		t.Fatalf("retry-ready job = %#v", job)
	}

	job["status"] = "running"
	job["attempt"] = config.BenchmarkSchedulerDefaultMaxAttempts
	job["timeout_at"] = time.Now().UTC().Add(-time.Minute)
	job["scheduler_state"] = "running"
	node.Observed["benchmark_jobs"] = upsertBenchmarkJob(node.Observed["benchmark_jobs"], job)
	if err := server.store.UpsertNode(node); err != nil {
		t.Fatalf("upsert forced timeout: %v", err)
	}
	run = server.reconcileBenchmarkJobs(time.Now().UTC(), "test_timeout")
	if !run.Changed || run.TimedOut != 1 || run.Exhausted != 1 {
		t.Fatalf("timeout reconcile = %#v", run)
	}
	node = server.store.Snapshot().Nodes["managed-scheduler"]
	job, _ = findBenchmarkJob(node, benchmarkID)
	if job["status"] != "failed" || job["scheduler_state"] != "exhausted" || job["error_code"] != benchmarkJobTimeoutErrorCode {
		t.Fatalf("exhausted timeout job = %#v", job)
	}
	rendered := string(mustJSON(t, node.Observed))
	for _, forbidden := range []string{credential, "SECRET_PROMPT", "Authorization", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("scheduler metadata leaked %q: %s", forbidden, rendered)
		}
	}
}

func TestBenchmarkSchedulerPolicyControlsApplyToNewJobsClaimsAndRetries(t *testing.T) {
	server := newIsolatedTestServer(t)
	cfg := server.store.Snapshot().Config
	cfg.Capabilities.BenchmarkScheduler = config.BenchmarkSchedulerConfig{
		Policy:              config.BenchmarkSchedulerPolicyBoundedRetryTimeout,
		MaxAttempts:         2,
		LeaseTimeoutSeconds: 45,
		RetryDelaySeconds:   7,
	}
	if err := server.store.SaveConfig(cfg); err != nil {
		t.Fatalf("save scheduler config: %v", err)
	}
	server.cfg = server.store.Snapshot().Config
	credential := "lw_hb_scheduler_policy_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-policy",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-policy"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-policy/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	var queued struct {
		Job map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatalf("decode queued: %v", err)
	}
	benchmarkID := benchmarkJobString(queued.Job["benchmark_id"])
	if benchmarkJobInt(queued.Job["max_attempts"], 0) != 2 ||
		benchmarkJobInt(queued.Job["lease_timeout_seconds"], 0) != 45 ||
		benchmarkJobInt(queued.Job["retry_delay_seconds"], 0) != 7 {
		t.Fatalf("queued job did not use configured scheduler policy: %#v", queued.Job)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-policy"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("claim status = %d body = %s", rr.Code, rr.Body.String())
	}
	var claimed struct {
		Job map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &claimed); err != nil {
		t.Fatalf("decode claimed: %v", err)
	}
	claimedAt, ok := benchmarkJobTime(claimed.Job["claimed_at"])
	if !ok {
		t.Fatalf("claimed job missing claimed_at: %#v", claimed.Job)
	}
	timeoutAt, ok := benchmarkJobTime(claimed.Job["timeout_at"])
	if !ok || int(timeoutAt.Sub(claimedAt).Seconds()) != 45 {
		t.Fatalf("claimed job timeout does not match policy: %#v", claimed.Job)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/status", bytes.NewBufferString(`{"node_id":"managed-policy","benchmark_id":"`+benchmarkID+`","status":"failed","error_code":"temporary_runner_error"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("failed status = %d body = %s", rr.Code, rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["managed-policy"]
	job, ok := findBenchmarkJob(node, benchmarkID)
	if !ok {
		t.Fatalf("job missing")
	}
	updatedAt, ok := benchmarkJobTime(job["updated_at"])
	if !ok {
		t.Fatalf("job missing updated_at: %#v", job)
	}
	nextAttemptAt, ok := benchmarkJobTime(job["next_attempt_at"])
	if !ok || int(nextAttemptAt.Sub(updatedAt).Seconds()) != 7 {
		t.Fatalf("retry delay does not match policy: %#v", job)
	}

	status := server.benchmarkSchedulerStatusAt(time.Now().UTC())
	if status.Config.MaxAttempts != 2 || status.Config.LeaseTimeoutSeconds != 45 || status.Config.RetryDelaySeconds != 7 {
		t.Fatalf("scheduler status config = %#v", status.Config)
	}
	if len(status.Jobs) == 0 || status.Jobs[0].MaxAttempts != 2 || status.Jobs[0].LeaseTimeoutSeconds != 45 || status.Jobs[0].RetryDelaySeconds != 7 {
		t.Fatalf("scheduler job status = %#v", status.Jobs)
	}
	rendered := string(mustJSON(t, status))
	for _, forbidden := range []string{credential, "SECRET_PROMPT", "Authorization", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("scheduler status leaked %q: %s", forbidden, rendered)
		}
	}
}

func TestBenchmarkSchedulerStatusAndManualReconcileAreMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	timeoutAt := time.Now().UTC().Add(-time.Minute)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-timeout",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed: map[string]interface{}{
			"benchmark_jobs": []map[string]interface{}{{
				"benchmark_id":     "bench-timeout",
				"status":           "running",
				"attempt":          1,
				"max_attempts":     2,
				"scheduler_policy": benchmarkJobSchedulerPolicy,
				"scheduler_state":  "running",
				"timeout_at":       timeoutAt,
				"updated_at":       timeoutAt,
			}},
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	status := server.benchmarkSchedulerStatusAt(time.Now().UTC())
	if status.Policy != benchmarkJobSchedulerPolicy || status.Summary["timeout_due"] != 1 {
		t.Fatalf("scheduler status = %#v", status)
	}

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/wrangler/ui/bootstrap", handler: server.bootstrap},
		{name: "metrics", method: http.MethodGet, path: "/wrangler/metrics", handler: server.metrics},
		{name: "manual_reconcile", method: http.MethodPost, path: "/wrangler/benchmarks/scheduler/reconcile", handler: server.benchmarkSchedulerReconcile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			tc.handler(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s status = %d body = %s", tc.name, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "benchmark_scheduler") && tc.name != "manual_reconcile" {
				t.Fatalf("%s missing scheduler status: %s", tc.name, rr.Body.String())
			}
			for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_hb_", "lw_admin_", "lw_client_", "sk-"} {
				if strings.Contains(rr.Body.String(), forbidden) {
					t.Fatalf("%s leaked %q: %s", tc.name, forbidden, rr.Body.String())
				}
			}
		})
	}
}

func TestBenchmarkSchedulerPolicyEndpointNormalizesAndPersistsMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/wrangler/benchmarks/scheduler/policy", bytes.NewBufferString(`{"policy":"future_unbounded_policy","max_attempts":99,"lease_timeout_seconds":1,"retry_delay_seconds":99999,"background_enabled":true,"tick_interval_seconds":1,"prompt":"SECRET_PROMPT"}`))
	server.putBenchmarkSchedulerPolicy(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("put scheduler policy status = %d body = %s", rr.Code, rr.Body.String())
	}
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_hb_", "sk-"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("put scheduler policy leaked %q: %s", forbidden, rr.Body.String())
		}
	}
	var body struct {
		Config config.BenchmarkSchedulerConfig `json:"config"`
		Limits BenchmarkSchedulerPolicyLimits  `json:"limits"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode policy response: %v", err)
	}
	if body.Config.Policy != config.BenchmarkSchedulerPolicyBoundedRetryTimeout ||
		body.Config.MaxAttempts != config.BenchmarkSchedulerMaxMaxAttempts ||
		body.Config.LeaseTimeoutSeconds != config.BenchmarkSchedulerMinLeaseTimeoutSeconds ||
		body.Config.RetryDelaySeconds != config.BenchmarkSchedulerMaxRetryDelaySeconds ||
		!body.Config.BackgroundEnabled ||
		body.Config.TickIntervalSeconds != config.BenchmarkSchedulerMinTickIntervalSeconds {
		t.Fatalf("normalized scheduler policy = %#v", body.Config)
	}
	if body.Limits.MaxAttemptsMax != config.BenchmarkSchedulerMaxMaxAttempts ||
		body.Limits.TickIntervalSecondsMin != config.BenchmarkSchedulerMinTickIntervalSeconds {
		t.Fatalf("limits missing: %#v", body.Limits)
	}

	stored := server.store.Snapshot().Config.Capabilities.BenchmarkScheduler
	if stored != body.Config {
		t.Fatalf("stored scheduler policy = %#v response = %#v", stored, body.Config)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/benchmarks/scheduler/policy", nil)
	server.benchmarkSchedulerPolicy(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get scheduler policy status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"max_attempts":10`) ||
		!strings.Contains(rr.Body.String(), `"background_enabled":true`) ||
		strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("get scheduler policy response unexpected: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/wrangler/benchmarks/scheduler/policy", bytes.NewBufferString(`{"max_attempts":4}`))
	server.putBenchmarkSchedulerPolicy(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("partial put scheduler policy status = %d body = %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode partial policy response: %v", err)
	}
	if body.Config.MaxAttempts != 4 || !body.Config.BackgroundEnabled || body.Config.TickIntervalSeconds != config.BenchmarkSchedulerMinTickIntervalSeconds {
		t.Fatalf("partial put did not preserve omitted background settings: %#v", body.Config)
	}
}

func TestBenchmarkSchedulerBackgroundTickDisabledByDefault(t *testing.T) {
	server := newIsolatedTestServer(t)
	now := time.Now().UTC()
	if server.benchmarkSchedulerConfig().BackgroundEnabled {
		t.Fatalf("background scheduler should be disabled by default")
	}
	if ran := server.runBenchmarkSchedulerBackgroundTick(now, true); ran {
		t.Fatalf("background scheduler tick ran while disabled")
	}
	status := server.benchmarkSchedulerStatusAt(now)
	if status.Background.Enabled || status.Background.LastTickAt != "" || status.Background.NextTickAt != "" {
		t.Fatalf("background status should be disabled with no tick metadata: %#v", status.Background)
	}
	if status.History.Count != 0 || len(status.History.Entries) != 0 || status.History.Retention != "process_local_reset_on_restart" {
		t.Fatalf("new process-local scheduler history should be empty: %#v", status.History)
	}
	if rendered := string(mustJSON(t, status.History)); !strings.Contains(rendered, `"entries":[]`) {
		t.Fatalf("empty scheduler history must serialize entries as an array: %s", rendered)
	}
}

func TestBenchmarkSchedulerBackgroundTickReconcilesMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	cfg := server.store.Snapshot().Config
	cfg.Capabilities.BenchmarkScheduler = config.BenchmarkSchedulerConfig{
		Policy:              config.BenchmarkSchedulerPolicyBoundedRetryTimeout,
		MaxAttempts:         2,
		LeaseTimeoutSeconds: 30,
		RetryDelaySeconds:   5,
		BackgroundEnabled:   true,
		TickIntervalSeconds: 10,
	}
	if err := server.store.SaveConfig(cfg); err != nil {
		t.Fatalf("save scheduler config: %v", err)
	}
	server.cfg = server.store.Snapshot().Config
	now := time.Now().UTC()
	timeoutAt := now.Add(-time.Minute)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-background",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed: map[string]interface{}{
			"benchmark_jobs": []map[string]interface{}{{
				"benchmark_id":     "bench-background",
				"status":           "running",
				"attempt":          1,
				"max_attempts":     2,
				"scheduler_policy": benchmarkJobSchedulerPolicy,
				"scheduler_state":  "running",
				"timeout_at":       timeoutAt,
				"updated_at":       timeoutAt,
			}},
		},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	if ran := server.runBenchmarkSchedulerBackgroundTick(now, true); !ran {
		t.Fatalf("background scheduler did not run when enabled")
	}
	node := server.store.Snapshot().Nodes["managed-background"]
	job, ok := findBenchmarkJob(node, "bench-background")
	if !ok {
		t.Fatalf("job missing")
	}
	if job["status"] != "failed" || job["scheduler_state"] != "retry_wait" || job["scheduler_reason"] != "retry_after_timeout" {
		t.Fatalf("background tick did not reconcile timed-out job: %#v", job)
	}
	status := server.benchmarkSchedulerStatusAt(now)
	if !status.Background.Enabled ||
		status.Background.TickIntervalSeconds != 10 ||
		status.Background.LastTickAt == "" ||
		status.Background.NextTickAt == "" ||
		!status.Background.LastChanged ||
		status.Background.LastTimedOut != 1 ||
		status.Background.LastRetried != 1 {
		t.Fatalf("background status = %#v", status.Background)
	}
	audit := string(mustJSON(t, server.store.Snapshot().Audit))
	if !strings.Contains(audit, "benchmark_scheduler_background_tick") {
		t.Fatalf("background tick telemetry missing: %s", audit)
	}
	rendered := string(mustJSON(t, status)) + audit
	for _, forbidden := range []string{"SECRET_PROMPT", "Authorization", "lw_hb_", "lw_admin_", "lw_client_", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("background scheduler leaked %q: %s", forbidden, rendered)
		}
	}
}

func TestBenchmarkSchedulerHistoryRecordsBackgroundAndOperatorReconciliations(t *testing.T) {
	server := newIsolatedTestServer(t)
	cfg := server.store.Snapshot().Config
	cfg.Capabilities.BenchmarkScheduler.BackgroundEnabled = true
	cfg.Capabilities.BenchmarkScheduler.TickIntervalSeconds = 10
	if err := server.store.SaveConfig(cfg); err != nil {
		t.Fatalf("save scheduler config: %v", err)
	}
	server.cfg = server.store.Snapshot().Config

	backgroundAt := time.Now().UTC().Add(-time.Minute)
	if ran := server.runBenchmarkSchedulerBackgroundTick(backgroundAt, true); !ran {
		t.Fatalf("background scheduler did not run")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/benchmarks/scheduler/reconcile", nil)
	server.benchmarkSchedulerReconcile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("manual reconcile status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/benchmarks/scheduler/history", nil)
	server.benchmarkSchedulerHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("history status = %d body = %s", rr.Code, rr.Body.String())
	}
	var history BenchmarkSchedulerHistoryStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if history.Count != 2 || len(history.Entries) != 2 || history.MaxEntries != benchmarkSchedulerAuditMaxEntries {
		t.Fatalf("history bounds/status = %#v", history)
	}
	if history.Entries[0].Trigger != benchmarkSchedulerAuditTriggerOperator || history.Entries[0].Reason != benchmarkJobManualReconcileReason {
		t.Fatalf("newest history entry = %#v", history.Entries[0])
	}
	if history.Entries[1].Trigger != benchmarkSchedulerAuditTriggerBackground || history.Entries[1].Reason != benchmarkJobBackgroundReconcileReason {
		t.Fatalf("older history entry = %#v", history.Entries[1])
	}
	if history.Summary[benchmarkSchedulerAuditTriggerOperator] != 1 || history.Summary[benchmarkSchedulerAuditTriggerBackground] != 1 {
		t.Fatalf("history summary = %#v", history.Summary)
	}

	for _, handler := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		path string
	}{
		{name: "bootstrap", call: server.bootstrap, path: "/wrangler/ui/bootstrap"},
		{name: "metrics", call: server.metrics, path: "/wrangler/metrics"},
	} {
		t.Run(handler.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, handler.path, nil)
			handler.call(rr, req)
			if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"recent_reconciliations"`) {
				t.Fatalf("%s missing scheduler history: status=%d body=%s", handler.name, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestBenchmarkSchedulerHistoryIsBoundedAndSanitizesInternalLabels(t *testing.T) {
	server := newIsolatedTestServer(t)
	policy := server.benchmarkSchedulerConfig()
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < benchmarkSchedulerAuditMaxEntries+5; i++ {
		server.benchmarkBackground.recordAudit(
			base.Add(time.Duration(i)*time.Second),
			"SECRET_TRIGGER_lw_admin_unsafe",
			"SECRET_PROMPT Authorization: Bearer SECRET",
			policy,
			benchmarkSchedulerRun{Changed: i%2 == 0, TimedOut: 1, Retried: 1},
		)
	}

	history := server.benchmarkSchedulerHistoryStatus()
	if history.Count != benchmarkSchedulerAuditMaxEntries || len(history.Entries) != benchmarkSchedulerAuditMaxEntries {
		t.Fatalf("history not bounded: %#v", history)
	}
	wantNewest := base.Add(time.Duration(benchmarkSchedulerAuditMaxEntries+4) * time.Second).Format(time.RFC3339)
	if history.Entries[0].RecordedAt != wantNewest {
		t.Fatalf("history not newest-first: got %q want %q", history.Entries[0].RecordedAt, wantNewest)
	}
	if history.Entries[0].Trigger != benchmarkSchedulerAuditTriggerInternal || history.Entries[0].Reason != "internal_reconcile" {
		t.Fatalf("unsafe labels were not normalized: %#v", history.Entries[0])
	}
	rendered := string(mustJSON(t, history)) + string(mustJSON(t, server.buildSupportBundle()))
	for _, forbidden := range []string{"SECRET_TRIGGER", "SECRET_PROMPT", "Authorization", "Bearer SECRET", "lw_admin_", "lw_hb_", "lw_client_", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("scheduler history/support bundle leaked %q: %s", forbidden, rendered)
		}
	}
}
