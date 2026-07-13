package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	benchmarkRunnerErrorOllamaUnavailable      = "runner_ollama_unavailable"
	benchmarkRunnerErrorOllamaRequestFailed    = "runner_ollama_request_failed"
	benchmarkRunnerErrorFixtureUnsupported     = "runner_fixture_execution_not_enabled"
	benchmarkRunnerSyntheticMaxPromptEvalCount = 1000000
	benchmarkRunnerSyntheticMaxEvalCount       = 1000000
)

type benchmarkRunnerSyntheticTask struct {
	ID     string
	Prompt string
}

type benchmarkRunnerOllamaGenerateResponse struct {
	Response           string `json:"response"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
	TotalDuration      int64  `json:"total_duration"`
	Done               bool   `json:"done"`
}

func (s *Server) benchmarkRunnerSyntheticResult(ctx context.Context, nodeID, benchmarkID, suiteID, fixtureManifestID string, taskCount int, job map[string]interface{}) map[string]interface{} {
	started := time.Now().UTC()
	model := benchmarkRunnerModel(job)
	if suiteID == localFixtureWorkloadSuiteID {
		return benchmarkRunnerFailedResult(nodeID, benchmarkID, model, suiteID, fixtureManifestID, taskCount, benchmarkRunnerErrorFixtureUnsupported, benchmarkRunnerResultModeSynthetic, started)
	}
	tasks := benchmarkRunnerSyntheticTasksForJob(suiteID, job)
	if len(tasks) == 0 {
		return benchmarkRunnerFailedResult(nodeID, benchmarkID, model, suiteID, fixtureManifestID, taskCount, benchmarkRunnerErrorUnsupportedSuite, benchmarkRunnerResultModeSynthetic, started)
	}
	inputTokens := 0
	generatedTokens := 0
	outputDurationNS := int64(0)
	prefillDurationNS := int64(0)
	for _, task := range tasks {
		result, err := s.runBenchmarkRunnerSyntheticTask(ctx, model, task)
		if err != nil {
			return benchmarkRunnerFailedResult(nodeID, benchmarkID, model, suiteID, fixtureManifestID, len(tasks), benchmarkRunnerErrorOllamaRequestFailed, benchmarkRunnerResultModeSynthetic, started)
		}
		inputTokens += boundedBenchmarkRunnerTokenCount(result.PromptEvalCount)
		generatedTokens += boundedBenchmarkRunnerTokenCount(result.EvalCount)
		outputDurationNS += nonNegativeInt64(result.EvalDuration)
		prefillDurationNS += nonNegativeInt64(result.PromptEvalDuration)
	}
	completed := time.Now().UTC()
	durationMS := int(completed.Sub(started).Milliseconds())
	if durationMS < 0 {
		durationMS = 0
	}
	outputTokensPerSecond := ratePerSecond(generatedTokens, outputDurationNS, durationMS)
	prefillTokensPerSecond := ratePerSecond(inputTokens, prefillDurationNS, durationMS)
	tokensPerSecond := outputTokensPerSecond
	if tokensPerSecond == 0 {
		tokensPerSecond = ratePerSecond(generatedTokens, int64(durationMS)*int64(time.Millisecond), durationMS)
	}
	result := map[string]interface{}{
		"node_id":                   nodeID,
		"benchmark_id":              benchmarkID,
		"model":                     safeBenchmarkString(model),
		"status":                    "completed",
		"started_at":                started,
		"completed_at":              completed,
		"duration_ms":               durationMS,
		"input_tokens":              inputTokens,
		"generated_tokens":          generatedTokens,
		"tokens_per_second":         tokensPerSecond,
		"output_tokens_per_second":  outputTokensPerSecond,
		"prefill_tokens_per_second": prefillTokensPerSecond,
		"suite_id":                  safeBenchmarkString(suiteID),
		"task_count":                len(tasks),
		"runner_mode":               benchmarkRunnerResultModeSynthetic,
	}
	return result
}

func (s *Server) runBenchmarkRunnerSyntheticTask(ctx context.Context, model string, task benchmarkRunnerSyntheticTask) (benchmarkRunnerOllamaGenerateResponse, error) {
	ollamaURL := strings.TrimRight(strings.TrimSpace(s.cfg.Ollama.URL), "/")
	if ollamaURL == "" {
		return benchmarkRunnerOllamaGenerateResponse{}, fmt.Errorf(benchmarkRunnerErrorOllamaUnavailable)
	}
	body := map[string]interface{}{
		"model":  safeBenchmarkString(model),
		"prompt": task.Prompt,
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": 64,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return benchmarkRunnerOllamaGenerateResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return benchmarkRunnerOllamaGenerateResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return benchmarkRunnerOllamaGenerateResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, benchmarkRunnerMaxResponseBodyBytes))
		return benchmarkRunnerOllamaGenerateResponse{}, fmt.Errorf("ollama status %d", resp.StatusCode)
	}
	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, benchmarkRunnerMaxResponseBodyBytes))
	if err != nil {
		return benchmarkRunnerOllamaGenerateResponse{}, err
	}
	var out benchmarkRunnerOllamaGenerateResponse
	if err := json.Unmarshal(rawResp, &out); err != nil {
		return benchmarkRunnerOllamaGenerateResponse{}, err
	}
	out.Response = ""
	return out, nil
}

func benchmarkRunnerSyntheticTasksForJob(suiteID string, job map[string]interface{}) []benchmarkRunnerSyntheticTask {
	allowed := map[string]bool{}
	for _, id := range benchmarkRunnerTaskIDsFromJob(job) {
		allowed[id] = true
	}
	var tasks []benchmarkRunnerSyntheticTask
	for _, task := range benchmarkRunnerBuiltInSyntheticTasks(suiteID) {
		if len(allowed) == 0 || allowed[task.ID] {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func benchmarkRunnerTaskIDsFromJob(job map[string]interface{}) []string {
	workload, ok := job["workload_suite"].(map[string]interface{})
	if !ok {
		return nil
	}
	var ids []string
	switch values := workload["task_ids"].(type) {
	case []string:
		for _, value := range values {
			if id := safeBenchmarkString(value); id != "" {
				ids = append(ids, id)
			}
		}
	case []interface{}:
		for _, value := range values {
			if id := safeBenchmarkString(benchmarkJobString(value)); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func benchmarkRunnerBuiltInSyntheticTasks(suiteID string) []benchmarkRunnerSyntheticTask {
	switch suiteID {
	case defaultBenchmarkWorkloadSuiteID:
		return []benchmarkRunnerSyntheticTask{
			{ID: "short_chat", Prompt: "Reply in one short sentence: local benchmark ready."},
			{ID: "json_shape", Prompt: "Return a compact JSON object with keys status and count. Use count 3."},
			{ID: "tiny_summary", Prompt: "Summarize this sentence in five words or fewer: Llama Wrangler routes local model work across trusted managed nodes."},
		}
	case "synthetic_code_v1":
		return []benchmarkRunnerSyntheticTask{
			{ID: "code_explain", Prompt: "In two sentences, explain what a bounded retry loop does in a queue worker."},
			{ID: "code_transform", Prompt: "Rewrite this JavaScript as concise TypeScript without changing behavior: function add(a,b){return a+b}"},
		}
	default:
		return nil
	}
}

func benchmarkRunnerFailedResult(nodeID, benchmarkID, model, suiteID, fixtureManifestID string, taskCount int, errorCode, runnerMode string, started time.Time) map[string]interface{} {
	completed := time.Now().UTC()
	if taskCount <= 0 {
		taskCount = 1
	}
	result := map[string]interface{}{
		"node_id":           nodeID,
		"benchmark_id":      benchmarkID,
		"model":             safeBenchmarkString(model),
		"status":            "failed",
		"started_at":        started,
		"completed_at":      completed,
		"duration_ms":       int(completed.Sub(started).Milliseconds()),
		"input_tokens":      0,
		"generated_tokens":  0,
		"tokens_per_second": 0,
		"suite_id":          safeBenchmarkString(suiteID),
		"task_count":        taskCount,
		"runner_mode":       safeBenchmarkString(runnerMode),
		"error_code":        safeBenchmarkString(errorCode),
	}
	if fixtureManifestID != "" {
		result["fixture_manifest_id"] = fixtureManifestID
	}
	return result
}

func boundedBenchmarkRunnerTokenCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > benchmarkRunnerSyntheticMaxEvalCount {
		return benchmarkRunnerSyntheticMaxEvalCount
	}
	return value
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func ratePerSecond(tokens int, durationNS int64, fallbackDurationMS int) float64 {
	if tokens <= 0 {
		return 0
	}
	seconds := float64(durationNS) / float64(time.Second)
	if seconds <= 0 && fallbackDurationMS > 0 {
		seconds = float64(fallbackDurationMS) / 1000
	}
	if seconds <= 0 {
		return 0
	}
	rate := float64(tokens) / seconds
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}
