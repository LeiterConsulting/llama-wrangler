package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

func TestSubscriberBenchmarkRunnerSyntheticBuiltinExecutesLocallyAndReportsMetricsOnly(t *testing.T) {
	marshal := newIsolatedTestServer(t)
	credential := "lw_hb_runner_SECRET"
	if err := marshal.store.UpsertNode(appstate.Node{
		NodeID:        "managed-synthetic-runner",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed: map[string]interface{}{
			"heartbeat_auth_method":   "shared_secret",
			"heartbeat_auth_required": true,
			"heartbeat_state":         "fresh",
		},
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := marshal.secrets.Set(subscriberHeartbeatSecretKey("managed-synthetic-runner"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-synthetic-runner/benchmark", strings.NewReader(`{"suite_id":"synthetic_code_v1","prompt":"SECRET_PROMPT"}`))
	marshal.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue synthetic benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	var mux http.ServeMux
	marshal.routes(&mux)
	marshalHTTP := httptest.NewServer(&mux)
	defer marshalHTTP.Close()

	ollamaCalls := 0
	ollamaSawPrompt := false
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
			return
		}
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected ollama path %s", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read ollama body: %v", err)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode ollama body: %v body=%s", err, string(raw))
		}
		if prompt, _ := body["prompt"].(string); strings.Contains(prompt, "bounded retry loop") || strings.Contains(prompt, "function add") {
			ollamaSawPrompt = true
		}
		if body["model"] != "llama3.1:8b" || body["stream"] != false {
			t.Fatalf("ollama request body = %#v", body)
		}
		ollamaCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"SECRET_RESPONSE","done":true,"prompt_eval_count":11,"prompt_eval_duration":100000000,"eval_count":7,"eval_duration":200000000,"total_duration":300000000}`))
	}))
	defer ollama.Close()

	cfg := config.Default("subscriber")
	cfg.Server.Mode = "subscriber"
	cfg.Node.NodeID = "managed-synthetic-runner"
	cfg.Registration.MarshalURL = marshalHTTP.URL
	cfg.Registration.HeartbeatCredential = credential
	cfg.Ollama.URL = ollama.URL
	cfg.Capabilities.BenchmarkRunner = config.BenchmarkRunnerConfig{
		Enabled:             true,
		Mode:                config.BenchmarkRunnerModeSyntheticBuiltin,
		PollIntervalSeconds: 5,
		MaxJobsPerTick:      1,
		ResultBodyPolicy:    config.BenchmarkRunnerResultPolicyMetricsOnly,
	}
	subscriber := newIsolatedTestServerWithConfig(t, cfg)

	run := subscriber.runSubscriberBenchmarkRunnerOnce(context.Background(), subscriber.benchmarkRunnerConfig())
	if run.Status != "completed" || run.Claimed != 1 || run.Completed != 1 || run.ErrorCode != "" || run.SuiteID != "synthetic_code_v1" {
		t.Fatalf("synthetic runner run = %#v", run)
	}
	if ollamaCalls != 2 || !ollamaSawPrompt {
		t.Fatalf("ollama calls = %d sawPrompt=%v", ollamaCalls, ollamaSawPrompt)
	}
	node := marshal.store.Snapshot().Nodes["managed-synthetic-runner"]
	result, ok := node.Observed["benchmark_last_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("benchmark_last_result missing: %#v", node.Observed)
	}
	if result["runner_mode"] != benchmarkRunnerResultModeSynthetic ||
		result["suite_id"] != "synthetic_code_v1" ||
		benchmarkJobInt(result["task_count"], 0) != 2 ||
		benchmarkJobInt(result["generated_tokens"], 0) != 14 ||
		benchmarkJobInt(result["input_tokens"], 0) != 22 {
		t.Fatalf("synthetic result metadata = %#v", result)
	}
	rendered := string(mustJSON(t, node.Observed))
	assertNoBenchmarkRunnerGuidanceLeak(t, rendered)
	for _, forbidden := range []string{"bounded retry loop", "function add", "SECRET_PROMPT", "SECRET_RESPONSE"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("synthetic runner persisted forbidden marker %q: %s", forbidden, rendered)
		}
	}
}

func TestSubscriberBenchmarkRunnerSyntheticBuiltinRejectsFixtureExecutionWithoutLoadingContents(t *testing.T) {
	marshal := newIsolatedTestServer(t)
	credential := "lw_hb_fixture_SECRET"
	if err := marshal.store.UpsertNode(appstate.Node{
		NodeID:        "managed-fixture-runner",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed: map[string]interface{}{
			"heartbeat_auth_method":   "shared_secret",
			"heartbeat_auth_required": true,
			"heartbeat_state":         "fresh",
		},
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := marshal.secrets.Set(subscriberHeartbeatSecretKey("managed-fixture-runner"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-fixture-runner/benchmark", strings.NewReader(`{"suite_id":"operator_local_fixtures_v1","fixture_manifest_id":"local-fixture-001","fixture_path":"/tmp/Secret Project/SECRET_PROMPT.json"}`))
	marshal.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue fixture benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	var mux http.ServeMux
	marshal.routes(&mux)
	marshalHTTP := httptest.NewServer(&mux)
	defer marshalHTTP.Close()

	ollamaCalls := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
			return
		}
		ollamaCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	cfg := config.Default("subscriber")
	cfg.Server.Mode = "subscriber"
	cfg.Node.NodeID = "managed-fixture-runner"
	cfg.Registration.MarshalURL = marshalHTTP.URL
	cfg.Registration.HeartbeatCredential = credential
	cfg.Ollama.URL = ollama.URL
	cfg.Capabilities.BenchmarkRunner = config.BenchmarkRunnerConfig{
		Enabled:             true,
		Mode:                config.BenchmarkRunnerModeSyntheticBuiltin,
		PollIntervalSeconds: 5,
		MaxJobsPerTick:      1,
		ResultBodyPolicy:    config.BenchmarkRunnerResultPolicyMetricsOnly,
	}
	subscriber := newIsolatedTestServerWithConfig(t, cfg)

	run := subscriber.runSubscriberBenchmarkRunnerOnce(context.Background(), subscriber.benchmarkRunnerConfig())
	if run.Status != "failed" || run.Claimed != 1 || run.Failed != 1 || run.ErrorCode != benchmarkRunnerErrorFixtureUnsupported {
		t.Fatalf("fixture runner run = %#v", run)
	}
	if ollamaCalls != 0 {
		t.Fatalf("fixture execution should not call ollama, calls=%d", ollamaCalls)
	}
	node := marshal.store.Snapshot().Nodes["managed-fixture-runner"]
	result, ok := node.Observed["benchmark_last_result"].(map[string]interface{})
	if !ok || result["status"] != "failed" || result["error_code"] != benchmarkRunnerErrorFixtureUnsupported || result["fixture_manifest_id"] != "local-fixture-001" {
		t.Fatalf("fixture result metadata = %#v", result)
	}
	rendered := string(mustJSON(t, node.Observed))
	assertNoBenchmarkRunnerGuidanceLeak(t, rendered)
	if strings.Contains(rendered, "/tmp/Secret Project") {
		t.Fatalf("fixture runner leaked full fixture path: %s", rendered)
	}
}

func TestSubscriberBenchmarkRunnerDryRunCompletesClaimedJob(t *testing.T) {
	marshal := newIsolatedTestServer(t)
	credential := "lw_hb_runner_SECRET"
	if err := marshal.store.UpsertNode(appstate.Node{
		NodeID:        "managed-runner",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed: map[string]interface{}{
			"heartbeat_auth_method":   "shared_secret",
			"heartbeat_auth_required": true,
			"heartbeat_state":         "fresh",
		},
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := marshal.secrets.Set(subscriberHeartbeatSecretKey("managed-runner"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-runner/benchmark", nil)
	marshal.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("queue benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	var mux http.ServeMux
	marshal.routes(&mux)
	marshalHTTP := httptest.NewServer(&mux)
	defer marshalHTTP.Close()

	cfg := config.Default("subscriber")
	cfg.Server.Mode = "subscriber"
	cfg.Node.NodeID = "managed-runner"
	cfg.Registration.MarshalURL = marshalHTTP.URL
	cfg.Registration.HeartbeatCredential = credential
	cfg.Capabilities.BenchmarkRunner = config.BenchmarkRunnerConfig{
		Enabled:             true,
		Mode:                config.BenchmarkRunnerModeDryRun,
		PollIntervalSeconds: 5,
		MaxJobsPerTick:      1,
		ResultBodyPolicy:    config.BenchmarkRunnerResultPolicyMetricsOnly,
	}
	subscriber := newIsolatedTestServerWithConfig(t, cfg)

	run := subscriber.runSubscriberBenchmarkRunnerOnce(context.Background(), subscriber.benchmarkRunnerConfig())
	if run.Status != "completed" || run.Claimed != 1 || run.Completed != 1 || run.ErrorCode != "" {
		t.Fatalf("runner run = %#v", run)
	}
	node := marshal.store.Snapshot().Nodes["managed-runner"]
	jobs := benchmarkJobs(node.Observed["benchmark_jobs"])
	if len(jobs) == 0 || jobs[0]["status"] != "completed" || jobs[0]["scheduler_state"] != "terminal" {
		t.Fatalf("job metadata = %#v", node.Observed["benchmark_jobs"])
	}
	result, ok := node.Observed["benchmark_last_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("benchmark_last_result missing: %#v", node.Observed)
	}
	if result["suite_id"] != defaultBenchmarkWorkloadSuiteID || benchmarkJobInt(result["task_count"], 0) <= 0 || benchmarkJobString(result["model"]) != "llama3.1:8b" {
		t.Fatalf("dry-run result metadata = %#v", result)
	}
	rendered := string(mustJSON(t, node.Observed))
	assertNoBenchmarkRunnerGuidanceLeak(t, rendered)
}

func TestSubscriberBenchmarkRunnerDryRunDisabledAndPassiveSafe(t *testing.T) {
	marshal := newIsolatedTestServer(t)
	if err := marshal.store.UpsertNode(appstate.Node{
		NodeID:        "passive-runner",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed:      map[string]interface{}{"benchmark_jobs": []map[string]interface{}{{"benchmark_id": "bench-passive", "status": "queued"}}},
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}
	var mux http.ServeMux
	marshal.routes(&mux)
	marshalHTTP := httptest.NewServer(&mux)
	defer marshalHTTP.Close()

	cfg := config.Default("subscriber")
	cfg.Server.Mode = "subscriber"
	cfg.Node.NodeID = "passive-runner"
	cfg.Registration.MarshalURL = marshalHTTP.URL
	cfg.Registration.HeartbeatCredential = "lw_hb_passive_SECRET"
	cfg.Capabilities.BenchmarkRunner = config.DefaultBenchmarkRunnerConfig()
	subscriber := newIsolatedTestServerWithConfig(t, cfg)

	run := subscriber.runSubscriberBenchmarkRunnerOnce(context.Background(), subscriber.benchmarkRunnerConfig())
	if run.ErrorCode != benchmarkRunnerErrorDisabled || run.Claimed != 0 || run.Completed != 0 {
		t.Fatalf("disabled runner run = %#v", run)
	}
	cfg.Capabilities.BenchmarkRunner.Enabled = true
	subscriber = newIsolatedTestServerWithConfig(t, cfg)
	run = subscriber.runSubscriberBenchmarkRunnerOnce(context.Background(), subscriber.benchmarkRunnerConfig())
	if run.ErrorCode != benchmarkRunnerErrorClaimFailed || run.Completed != 0 {
		t.Fatalf("passive runner run = %#v", run)
	}
	node := marshal.store.Snapshot().Nodes["passive-runner"]
	if node.Observed["benchmark_status"] == "completed" {
		t.Fatalf("passive benchmark job was completed: %#v", node.Observed)
	}
	rendered := string(mustJSON(t, node.Observed))
	if strings.Contains(rendered, "lw_hb_passive_SECRET") || strings.Contains(rendered, "SECRET_PROMPT") || strings.Contains(rendered, "SECRET_RESPONSE") {
		t.Fatalf("passive runner leaked forbidden marker: %s", rendered)
	}
}

func TestMetricsIncludesBenchmarkRunnerStatus(t *testing.T) {
	server := newIsolatedTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/metrics", nil)
	server.metrics(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	runner, ok := body["benchmark_runner"].(map[string]interface{})
	if !ok || runner["status"] != benchmarkRunnerStatusDisabled {
		t.Fatalf("metrics benchmark runner = %#v", body["benchmark_runner"])
	}
	assertNoBenchmarkRunnerGuidanceLeak(t, rr.Body.String())
}
