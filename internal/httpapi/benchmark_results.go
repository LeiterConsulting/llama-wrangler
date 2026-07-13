package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/telemetry"
)

const maxBenchmarkResults = 8

type subscriberBenchmarkResultRequest struct {
	NodeID                 string    `json:"node_id"`
	BenchmarkID            string    `json:"benchmark_id"`
	Model                  string    `json:"model"`
	Status                 string    `json:"status"`
	StartedAt              time.Time `json:"started_at"`
	CompletedAt            time.Time `json:"completed_at"`
	DurationMS             int       `json:"duration_ms"`
	InputTokens            int       `json:"input_tokens"`
	GeneratedTokens        int       `json:"generated_tokens"`
	TokensPerSecond        float64   `json:"tokens_per_second"`
	OutputTokensPerSecond  float64   `json:"output_tokens_per_second"`
	PrefillTokensPerSecond float64   `json:"prefill_tokens_per_second"`
	ErrorCode              string    `json:"error_code"`
	SuiteID                string    `json:"suite_id"`
	TaskCount              int       `json:"task_count"`
	FixtureManifestID      string    `json:"fixture_manifest_id"`
	RunnerMode             string    `json:"runner_mode"`
}

func (s *Server) subscriberBenchmarkResult(w http.ResponseWriter, r *http.Request) {
	var body subscriberBenchmarkResultRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.NodeID = strings.TrimSpace(body.NodeID)
	if body.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
		return
	}
	state := s.store.Snapshot()
	node, ok := state.Nodes[body.NodeID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "benchmark results are only accepted from managed nodes"})
		return
	}
	if !s.verifySubscriberHeartbeatCredential(w, r, node) {
		return
	}
	result := benchmarkResultMap(body, appstate.BenchmarkSourceSubscriberReported, "subscriber_reported")
	applyBenchmarkResult(&node, result, appstate.BenchmarkSourceSubscriberReported)
	completeBenchmarkJobWithResult(&node, result)
	if body.TokensPerSecond > 0 {
		updateModelBenchmarkRate(&node, safeBenchmarkString(body.Model), body.TokensPerSecond)
	}
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stored := s.store.Snapshot().Nodes[node.NodeID]
	s.tele.Emit("benchmark_result_ingested", telemetry.Event{
		"node_id":          stored.NodeID,
		"control_level":    stored.ControlLevel,
		"trust_level":      stored.TrustLevel,
		"approval_state":   stored.ApprovalState,
		"benchmark_source": appstate.BenchmarkSourceSubscriberReported,
		"benchmark_status": result["status"],
		"model":            result["model"],
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"node":   stored,
		"result": result,
	})
}

func (s *Server) runPassiveBenchmarkProbe(node appstate.Node) (appstate.Node, map[string]interface{}, error) {
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Routing.Timeout())
	defer cancel()
	models, err := s.fetchPassiveEndpointModels(ctx, node.URL)
	completed := time.Now().UTC()
	result := map[string]interface{}{
		"benchmark_id": "probe_" + randomHex(6),
		"source":       appstate.BenchmarkSourceMarshalObserved,
		"mode":         "marshal_observed_api_tags",
		"status":       "probe_ok",
		"started_at":   started,
		"completed_at": completed,
		"duration_ms":  int(completed.Sub(started).Milliseconds()),
		"model_count":  len(models),
	}
	if err != nil {
		result["status"] = "probe_failed"
		result["error_code"] = safeBenchmarkString(friendlyProbeErrorCode(err))
	} else {
		node.Models = models
		node.OllamaAvailable = true
		node.Status = "healthy"
	}
	now := completed
	node.LastObservedAt = &now
	applyBenchmarkResult(&node, result, appstate.BenchmarkSourceMarshalObserved)
	return node, result, err
}

func benchmarkResultMap(body subscriberBenchmarkResultRequest, source, mode string) map[string]interface{} {
	now := time.Now().UTC()
	status := safeBenchmarkStatus(body.Status)
	completed := body.CompletedAt
	if completed.IsZero() {
		completed = now
	}
	result := map[string]interface{}{
		"benchmark_id":              safeBenchmarkString(defaultString(body.BenchmarkID, "bench_"+randomHex(6))),
		"source":                    source,
		"mode":                      mode,
		"model":                     safeBenchmarkString(body.Model),
		"status":                    status,
		"completed_at":              completed,
		"duration_ms":               nonNegativeInt(body.DurationMS),
		"input_tokens":              nonNegativeInt(body.InputTokens),
		"generated_tokens":          nonNegativeInt(body.GeneratedTokens),
		"tokens_per_second":         nonNegativeFloat(body.TokensPerSecond),
		"output_tokens_per_second":  nonNegativeFloat(body.OutputTokensPerSecond),
		"prefill_tokens_per_second": nonNegativeFloat(body.PrefillTokensPerSecond),
	}
	if !body.StartedAt.IsZero() {
		result["started_at"] = body.StartedAt
	}
	if body.ErrorCode != "" {
		result["error_code"] = safeBenchmarkString(body.ErrorCode)
	}
	if suiteID := safeBenchmarkString(body.SuiteID); suiteID != "" {
		result["suite_id"] = suiteID
	}
	if body.TaskCount > 0 {
		result["task_count"] = nonNegativeInt(body.TaskCount)
	}
	if fixtureID := safeFixtureReference(body.FixtureManifestID); fixtureID != "" {
		result["fixture_manifest_id"] = fixtureID
	}
	if runnerMode := safeBenchmarkString(body.RunnerMode); runnerMode != "" {
		result["runner_mode"] = runnerMode
	}
	return result
}

func applyBenchmarkResult(node *appstate.Node, result map[string]interface{}, source string) {
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.BenchmarkSource = source
	node.Observed["benchmark_status"] = result["status"]
	node.Observed["benchmark_source"] = source
	node.Observed["benchmark_last_result"] = result
	node.Observed["benchmark_updated_at"] = time.Now().UTC()
	node.Observed["benchmark_results"] = appendBenchmarkResult(node.Observed["benchmark_results"], result)
}

func appendBenchmarkResult(existing interface{}, result map[string]interface{}) []map[string]interface{} {
	out := []map[string]interface{}{result}
	switch values := existing.(type) {
	case []map[string]interface{}:
		out = append(out, values...)
	case []interface{}:
		for _, value := range values {
			item, ok := value.(map[string]interface{})
			if ok {
				out = append(out, item)
			}
		}
	}
	if len(out) > maxBenchmarkResults {
		return out[:maxBenchmarkResults]
	}
	return out
}

func updateModelBenchmarkRate(node *appstate.Node, model string, tokensPerSecond float64) {
	if model == "" || tokensPerSecond <= 0 {
		return
	}
	for i := range node.Models {
		if node.Models[i].Name == model {
			node.Models[i].TokensSec = tokensPerSecond
			return
		}
	}
	node.Models = append(node.Models, appstate.ModelState{Name: model, State: "installed", TokensSec: tokensPerSecond})
}

func safeBenchmarkStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "queued", "running", "completed", "failed", "cancelled", "reported", "probe_ok", "probe_failed":
		return strings.TrimSpace(status)
	case "":
		return "reported"
	default:
		return "reported"
	}
}

func safeBenchmarkString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 160 {
		value = value[:160]
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"lw_hb_", "lw_enroll_", "lw_admin_", "lw_client_", "authorization", "secret_prompt", "secret_response", "sk-"} {
		if strings.Contains(lower, marker) {
			return "[redacted]"
		}
	}
	return value
}

func friendlyProbeErrorCode(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "timeout") || strings.Contains(text, "deadline"):
		return "timeout"
	case strings.Contains(text, "connection refused"):
		return "connection_refused"
	default:
		return "api_tags_unavailable"
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
