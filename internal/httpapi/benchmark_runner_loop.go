package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/config"
	"llama-wrangler/internal/telemetry"
)

const (
	benchmarkRunnerStatusNoJob           = "no_job"
	benchmarkRunnerStatusJobClaimed      = "job_claimed"
	benchmarkRunnerErrorDisabled         = "runner_disabled"
	benchmarkRunnerErrorNotSubscriber    = "runner_requires_subscriber_mode"
	benchmarkRunnerErrorMissingNodeID    = "runner_missing_node_id"
	benchmarkRunnerErrorMissingMarshal   = "runner_missing_marshal_url"
	benchmarkRunnerErrorMissingCred      = "runner_missing_heartbeat_credential"
	benchmarkRunnerErrorClaimFailed      = "runner_claim_failed"
	benchmarkRunnerErrorStatusFailed     = "runner_status_update_failed"
	benchmarkRunnerErrorResultFailed     = "runner_result_report_failed"
	benchmarkRunnerErrorMalformedClaim   = "runner_malformed_claim"
	benchmarkRunnerErrorUnsupportedSuite = "runner_unsupported_suite"
	benchmarkRunnerResultModeDryRun      = "subscriber_local_dry_run"
	benchmarkRunnerResultModeSynthetic   = "subscriber_local_synthetic_builtin"
	benchmarkRunnerSyntheticModel        = "synthetic-dry-run"
	benchmarkRunnerMaxResponseBodyBytes  = 1 << 20
)

type benchmarkRunnerRun struct {
	Status      string
	ErrorCode   string
	Claimed     int
	Completed   int
	Failed      int
	NoJob       bool
	BenchmarkID string
	SuiteID     string
	TaskCount   int
}

func (s *Server) benchmarkRunnerConfig() config.BenchmarkRunnerConfig {
	return config.NormalizeBenchmarkRunnerConfig(s.cfg.Capabilities.BenchmarkRunner)
}

func (s *Server) runSubscriberBenchmarkRunner(ctx context.Context) {
	for {
		runnerConfig := s.benchmarkRunnerConfig()
		if runnerConfig.Enabled {
			_ = s.runSubscriberBenchmarkRunnerOnce(ctx, runnerConfig)
		}
		wait := time.Duration(runnerConfig.PollIntervalSeconds) * time.Second
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Server) runSubscriberBenchmarkRunnerOnce(ctx context.Context, runnerConfig config.BenchmarkRunnerConfig) benchmarkRunnerRun {
	runnerConfig = config.NormalizeBenchmarkRunnerConfig(runnerConfig)
	run := benchmarkRunnerRun{Status: "skipped"}
	if !runnerConfig.Enabled {
		run.ErrorCode = benchmarkRunnerErrorDisabled
		s.emitBenchmarkRunnerRun(run, runnerConfig)
		return run
	}
	if s.cfg.Server.Mode != "subscriber" {
		run.ErrorCode = benchmarkRunnerErrorNotSubscriber
		s.emitBenchmarkRunnerRun(run, runnerConfig)
		return run
	}
	nodeID := strings.TrimSpace(s.cfg.Node.NodeID)
	if nodeID == "" {
		run.ErrorCode = benchmarkRunnerErrorMissingNodeID
		s.emitBenchmarkRunnerRun(run, runnerConfig)
		return run
	}
	marshalURL := strings.TrimRight(strings.TrimSpace(s.cfg.Registration.MarshalURL), "/")
	if marshalURL == "" {
		run.ErrorCode = benchmarkRunnerErrorMissingMarshal
		s.emitBenchmarkRunnerRun(run, runnerConfig)
		return run
	}
	credential := strings.TrimSpace(s.cfg.Registration.HeartbeatCredential)
	if credential == "" {
		run.ErrorCode = benchmarkRunnerErrorMissingCred
		s.emitBenchmarkRunnerRun(run, runnerConfig)
		return run
	}
	for i := 0; i < runnerConfig.MaxJobsPerTick; i++ {
		claimed, shouldContinue := s.runSubscriberBenchmarkRunnerJob(ctx, marshalURL, credential, nodeID, runnerConfig)
		if claimed.NoJob {
			run.NoJob = true
			if run.Status == "skipped" {
				run.Status = benchmarkRunnerStatusNoJob
			}
			break
		}
		if claimed.ErrorCode != "" {
			run.ErrorCode = claimed.ErrorCode
			run.Claimed += claimed.Claimed
			run.Completed += claimed.Completed
			run.Failed += claimed.Failed
			run.BenchmarkID = claimed.BenchmarkID
			run.SuiteID = claimed.SuiteID
			run.TaskCount = claimed.TaskCount
			if run.Status == "skipped" {
				run.Status = "failed"
			}
			break
		}
		run.Claimed += claimed.Claimed
		run.Completed += claimed.Completed
		run.BenchmarkID = claimed.BenchmarkID
		run.SuiteID = claimed.SuiteID
		run.TaskCount = claimed.TaskCount
		run.Status = "completed"
		if !shouldContinue {
			break
		}
	}
	s.emitBenchmarkRunnerRun(run, runnerConfig)
	return run
}

func (s *Server) runSubscriberBenchmarkRunnerJob(ctx context.Context, marshalURL, credential, nodeID string, runnerConfig config.BenchmarkRunnerConfig) (benchmarkRunnerRun, bool) {
	claimBody := map[string]string{"node_id": nodeID}
	var claim struct {
		Status string                 `json:"status"`
		Job    map[string]interface{} `json:"job"`
	}
	if err := s.benchmarkRunnerPost(ctx, marshalURL+"/subscriber/benchmarks/claim", credential, claimBody, &claim); err != nil {
		return benchmarkRunnerRun{Status: "failed", ErrorCode: benchmarkRunnerErrorClaimFailed, Failed: 1}, false
	}
	if claim.Status == benchmarkRunnerStatusNoJob {
		return benchmarkRunnerRun{Status: benchmarkRunnerStatusNoJob, NoJob: true}, false
	}
	if claim.Status != benchmarkRunnerStatusJobClaimed || claim.Job == nil {
		return benchmarkRunnerRun{Status: "failed", ErrorCode: benchmarkRunnerErrorMalformedClaim, Failed: 1}, false
	}
	benchmarkID := benchmarkJobString(claim.Job["benchmark_id"])
	if benchmarkID == "" {
		return benchmarkRunnerRun{Status: "failed", ErrorCode: benchmarkRunnerErrorMalformedClaim, Failed: 1}, false
	}
	suiteID, taskCount, fixtureManifestID := benchmarkRunnerWorkloadMetadata(claim.Job)
	statusBody := map[string]string{
		"node_id":      nodeID,
		"benchmark_id": benchmarkID,
		"status":       "running",
	}
	var statusResp map[string]interface{}
	if err := s.benchmarkRunnerPost(ctx, marshalURL+"/subscriber/benchmarks/status", credential, statusBody, &statusResp); err != nil {
		return benchmarkRunnerRun{Status: "failed", ErrorCode: benchmarkRunnerErrorStatusFailed, Failed: 1, BenchmarkID: benchmarkID, SuiteID: suiteID, TaskCount: taskCount}, false
	}
	resultBody := s.benchmarkRunnerResult(ctx, nodeID, benchmarkID, suiteID, fixtureManifestID, taskCount, claim.Job, runnerConfig)
	var resultResp map[string]interface{}
	if err := s.benchmarkRunnerPost(ctx, marshalURL+"/subscriber/benchmarks", credential, resultBody, &resultResp); err != nil {
		return benchmarkRunnerRun{Status: "failed", ErrorCode: benchmarkRunnerErrorResultFailed, Failed: 1, BenchmarkID: benchmarkID, SuiteID: suiteID, TaskCount: taskCount}, false
	}
	resultStatus := benchmarkJobString(resultBody["status"])
	if resultStatus == "failed" {
		return benchmarkRunnerRun{
			Status:      "failed",
			ErrorCode:   safeBenchmarkString(benchmarkJobString(resultBody["error_code"])),
			Claimed:     1,
			Failed:      1,
			BenchmarkID: benchmarkID,
			SuiteID:     suiteID,
			TaskCount:   taskCount,
		}, true
	}
	return benchmarkRunnerRun{
		Status:      "completed",
		Claimed:     1,
		Completed:   1,
		BenchmarkID: benchmarkID,
		SuiteID:     suiteID,
		TaskCount:   taskCount,
	}, true
}

func (s *Server) benchmarkRunnerResult(ctx context.Context, nodeID, benchmarkID, suiteID, fixtureManifestID string, taskCount int, job map[string]interface{}, runnerConfig config.BenchmarkRunnerConfig) map[string]interface{} {
	switch runnerConfig.Mode {
	case config.BenchmarkRunnerModeSyntheticBuiltin:
		return s.benchmarkRunnerSyntheticResult(ctx, nodeID, benchmarkID, suiteID, fixtureManifestID, taskCount, job)
	default:
		return benchmarkRunnerDryRunResult(nodeID, benchmarkID, suiteID, fixtureManifestID, taskCount, job)
	}
}

func (s *Server) benchmarkRunnerPost(ctx context.Context, url, credential string, body interface{}, out interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, benchmarkRunnerMaxResponseBodyBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runner request failed with status %d", resp.StatusCode)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func benchmarkRunnerWorkloadMetadata(job map[string]interface{}) (string, int, string) {
	suiteID := defaultBenchmarkWorkloadSuiteID
	taskCount := 1
	fixtureManifestID := ""
	if workload, ok := job["workload_suite"].(map[string]interface{}); ok {
		if value := safeBenchmarkString(benchmarkJobString(workload["suite_id"])); value != "" {
			suiteID = value
		}
		taskCount = benchmarkJobInt(workload["task_count"], taskCount)
		fixtureManifestID = safeFixtureReference(benchmarkJobString(workload["fixture_manifest_id"]))
	}
	if taskCount <= 0 {
		taskCount = 1
	}
	if taskCount > 50 {
		taskCount = 50
	}
	return suiteID, taskCount, fixtureManifestID
}

func benchmarkRunnerDryRunResult(nodeID, benchmarkID, suiteID, fixtureManifestID string, taskCount int, job map[string]interface{}) map[string]interface{} {
	if taskCount <= 0 {
		taskCount = 1
	}
	durationMS := 100 + taskCount*75
	generatedTokens := taskCount * 24
	inputTokens := taskCount * 16
	tokensPerSecond := float64(generatedTokens) / (float64(durationMS) / 1000)
	model := benchmarkRunnerModel(job)
	now := time.Now().UTC()
	result := map[string]interface{}{
		"node_id":                  nodeID,
		"benchmark_id":             benchmarkID,
		"model":                    model,
		"status":                   "completed",
		"started_at":               now.Add(-time.Duration(durationMS) * time.Millisecond),
		"completed_at":             now,
		"duration_ms":              durationMS,
		"input_tokens":             inputTokens,
		"generated_tokens":         generatedTokens,
		"tokens_per_second":        tokensPerSecond,
		"output_tokens_per_second": tokensPerSecond,
		"suite_id":                 safeBenchmarkString(suiteID),
		"task_count":               taskCount,
		"runner_mode":              benchmarkRunnerResultModeDryRun,
	}
	if fixtureManifestID != "" {
		result["fixture_manifest_id"] = fixtureManifestID
	}
	return result
}

func benchmarkRunnerModel(job map[string]interface{}) string {
	if candidates, ok := job["model_candidates"].([]interface{}); ok {
		for _, candidate := range candidates {
			if model := safeBenchmarkString(benchmarkJobString(candidate)); model != "" {
				return model
			}
		}
	}
	if candidates, ok := job["model_candidates"].([]string); ok {
		for _, candidate := range candidates {
			if model := safeBenchmarkString(candidate); model != "" {
				return model
			}
		}
	}
	return benchmarkRunnerSyntheticModel
}

func (s *Server) emitBenchmarkRunnerRun(run benchmarkRunnerRun, runnerConfig config.BenchmarkRunnerConfig) {
	s.tele.Emit("subscriber_benchmark_runner_tick", telemetry.Event{
		"mode":                  runnerConfig.Mode,
		"enabled":               runnerConfig.Enabled,
		"result_body_policy":    runnerConfig.ResultBodyPolicy,
		"status":                run.Status,
		"error_code":            run.ErrorCode,
		"claimed":               run.Claimed,
		"completed":             run.Completed,
		"failed":                run.Failed,
		"no_job":                run.NoJob,
		"benchmark_id":          safeBenchmarkString(run.BenchmarkID),
		"suite_id":              safeBenchmarkString(run.SuiteID),
		"task_count":            run.TaskCount,
		"poll_interval_seconds": runnerConfig.PollIntervalSeconds,
		"max_jobs_per_tick":     runnerConfig.MaxJobsPerTick,
	})
}
