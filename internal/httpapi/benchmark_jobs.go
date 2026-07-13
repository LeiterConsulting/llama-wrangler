package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
	"llama-wrangler/internal/telemetry"
)

const (
	maxBenchmarkJobs                  = 8
	benchmarkJobSchedulerPolicy       = config.BenchmarkSchedulerPolicyBoundedRetryTimeout
	benchmarkJobTimeoutErrorCode      = "benchmark_timeout"
	benchmarkJobSubscriberFailedCode  = "subscriber_reported_failed"
	benchmarkJobManualReconcileReason = "operator_reconcile"
)

type subscriberBenchmarkJobRequest struct {
	NodeID      string `json:"node_id"`
	BenchmarkID string `json:"benchmark_id"`
	Status      string `json:"status"`
	ErrorCode   string `json:"error_code"`
}

func (s *Server) subscriberBenchmarkJobClaim(w http.ResponseWriter, r *http.Request) {
	var body subscriberBenchmarkJobRequest
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "benchmark jobs are only supported for managed nodes"})
		return
	}
	if !s.verifySubscriberHeartbeatCredential(w, r, node) {
		return
	}
	now := time.Now().UTC()
	schedulerPolicy := s.benchmarkSchedulerConfig()
	reconcile := reconcileBenchmarkJobsForNode(&node, now, "subscriber_claim", schedulerPolicy)
	if reconcile.Changed {
		if err := s.store.UpsertNode(node); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	job, ok := nextQueuedBenchmarkJob(node, now)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "no_job", "job": nil})
		return
	}
	job["status"] = "running"
	job["attempt"] = benchmarkJobInt(job["attempt"], 0) + 1
	job["claimed_at"] = now
	job["updated_at"] = now
	job["timeout_at"] = now.Add(benchmarkJobLeaseTimeout(job, schedulerPolicy))
	job["scheduler_policy"] = benchmarkJobSchedulerPolicy
	job["scheduler_state"] = "running"
	job["scheduler_reason"] = "claimed"
	job["lease_timeout_seconds"] = benchmarkJobInt(job["lease_timeout_seconds"], schedulerPolicy.LeaseTimeoutSeconds)
	job["retry_delay_seconds"] = benchmarkJobInt(job["retry_delay_seconds"], schedulerPolicy.RetryDelaySeconds)
	job["max_attempts"] = benchmarkJobInt(job["max_attempts"], schedulerPolicy.MaxAttempts)
	delete(job, "completed_at")
	delete(job, "failed_at")
	delete(job, "timed_out_at")
	delete(job, "next_attempt_at")
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["benchmark_status"] = "running"
	node.Observed["benchmark_id"] = job["benchmark_id"]
	node.Observed["benchmark_updated_at"] = now
	node.Observed["benchmark_jobs"] = upsertBenchmarkJob(node.Observed["benchmark_jobs"], job)
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stored := s.store.Snapshot().Nodes[node.NodeID]
	s.tele.Emit("benchmark_job_claimed", telemetry.Event{
		"node_id":        stored.NodeID,
		"control_level":  stored.ControlLevel,
		"trust_level":    stored.TrustLevel,
		"approval_state": stored.ApprovalState,
		"benchmark_id":   job["benchmark_id"],
		"status":         job["status"],
		"attempt":        job["attempt"],
		"max_attempts":   job["max_attempts"],
		"scheduler":      job["scheduler_policy"],
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "job_claimed",
		"node":   stored,
		"job":    job,
	})
}

func (s *Server) subscriberBenchmarkJobStatus(w http.ResponseWriter, r *http.Request) {
	var body subscriberBenchmarkJobRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.NodeID = strings.TrimSpace(body.NodeID)
	body.BenchmarkID = safeBenchmarkString(body.BenchmarkID)
	status := safeBenchmarkStatus(body.Status)
	if body.NodeID == "" || body.BenchmarkID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and benchmark_id are required"})
		return
	}
	if !benchmarkJobStatusAllowed(status) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid benchmark job status is required"})
		return
	}
	state := s.store.Snapshot()
	node, ok := state.Nodes[body.NodeID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "benchmark jobs are only supported for managed nodes"})
		return
	}
	if !s.verifySubscriberHeartbeatCredential(w, r, node) {
		return
	}
	job, ok := findBenchmarkJob(node, body.BenchmarkID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "benchmark job not found"})
		return
	}
	now := time.Now().UTC()
	schedulerPolicy := s.benchmarkSchedulerConfig()
	job["status"] = status
	job["updated_at"] = now
	if body.ErrorCode != "" {
		job["error_code"] = safeBenchmarkString(body.ErrorCode)
	}
	switch status {
	case "running":
		job["scheduler_state"] = "running"
		job["scheduler_reason"] = "subscriber_status"
		if _, ok := benchmarkJobTime(job["timeout_at"]); !ok {
			job["timeout_at"] = now.Add(benchmarkJobLeaseTimeout(job, schedulerPolicy))
		}
	case "failed":
		if body.ErrorCode == "" {
			job["error_code"] = benchmarkJobSubscriberFailedCode
		}
		job["failed_at"] = now
		scheduleBenchmarkJobRetry(job, now, "retry_after_failure", schedulerPolicy)
	case "cancelled":
		job["scheduler_state"] = "terminal"
		job["scheduler_reason"] = "cancelled"
		job["completed_at"] = now
	case "completed":
		job["scheduler_state"] = "terminal"
		job["scheduler_reason"] = "completed"
		job["completed_at"] = now
	}
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["benchmark_status"] = status
	node.Observed["benchmark_id"] = body.BenchmarkID
	node.Observed["benchmark_updated_at"] = now
	node.Observed["benchmark_jobs"] = upsertBenchmarkJob(node.Observed["benchmark_jobs"], job)
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stored := s.store.Snapshot().Nodes[node.NodeID]
	s.tele.Emit("benchmark_job_status", telemetry.Event{
		"node_id":         stored.NodeID,
		"control_level":   stored.ControlLevel,
		"trust_level":     stored.TrustLevel,
		"approval_state":  stored.ApprovalState,
		"benchmark_id":    body.BenchmarkID,
		"status":          status,
		"error_code":      job["error_code"],
		"attempt":         job["attempt"],
		"max_attempts":    job["max_attempts"],
		"scheduler":       job["scheduler_policy"],
		"scheduler_state": job["scheduler_state"],
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"node":   stored,
		"job":    job,
	})
}

func createManagedBenchmarkJob(node *appstate.Node, policy BenchmarkPolicyNode, workloadSuite map[string]interface{}, schedulerPolicy config.BenchmarkSchedulerConfig) map[string]interface{} {
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	now := time.Now().UTC()
	benchmarkID := "bench_" + randomHex(6)
	job := map[string]interface{}{
		"benchmark_id":          benchmarkID,
		"node_id":               node.NodeID,
		"type":                  "managed_node_metadata_benchmark",
		"source":                appstate.BenchmarkSourceSubscriberReported,
		"mode":                  policy.Mode,
		"status":                "queued",
		"requested_at":          now,
		"updated_at":            now,
		"next_attempt_at":       now,
		"attempt":               0,
		"max_attempts":          schedulerPolicy.MaxAttempts,
		"lease_timeout_seconds": schedulerPolicy.LeaseTimeoutSeconds,
		"retry_delay_seconds":   schedulerPolicy.RetryDelaySeconds,
		"scheduler_policy":      schedulerPolicy.Policy,
		"scheduler_state":       "queued",
		"scheduler_reason":      "created",
		"model_candidates":      benchmarkModelCandidates(*node),
		"workload_suite":        workloadSuite,
		"result_endpoint":       "/subscriber/benchmarks",
		"status_endpoint":       "/subscriber/benchmarks/status",
	}
	node.BenchmarkSource = appstate.BenchmarkSourceSubscriberReported
	node.Observed["benchmark_status"] = "queued"
	node.Observed["benchmark_id"] = benchmarkID
	node.Observed["benchmark_mode"] = policy.Mode
	node.Observed["benchmark_policy"] = "managed_subscriber_reported"
	node.Observed["benchmark_workload_suite"] = workloadSuite
	node.Observed["benchmark_updated_at"] = now
	node.Observed["benchmark_jobs"] = upsertBenchmarkJob(node.Observed["benchmark_jobs"], job)
	return job
}

func completeBenchmarkJobWithResult(node *appstate.Node, result map[string]interface{}) {
	benchmarkID, _ := result["benchmark_id"].(string)
	if benchmarkID == "" {
		return
	}
	job, ok := findBenchmarkJob(*node, benchmarkID)
	if !ok {
		return
	}
	status, _ := result["status"].(string)
	if status == "" || status == "reported" {
		status = "completed"
	}
	now := time.Now().UTC()
	job["status"] = status
	job["updated_at"] = now
	job["completed_at"] = result["completed_at"]
	job["result_status"] = status
	job["scheduler_state"] = "terminal"
	job["scheduler_reason"] = "result_ingested"
	for _, key := range []string{"model", "duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "error_code", "suite_id", "task_count", "fixture_manifest_id", "runner_mode"} {
		if value, ok := result[key]; ok {
			job[key] = value
		}
	}
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["benchmark_jobs"] = upsertBenchmarkJob(node.Observed["benchmark_jobs"], job)
}

func nextQueuedBenchmarkJob(node appstate.Node, now time.Time) (map[string]interface{}, bool) {
	for _, job := range benchmarkJobs(node.Observed["benchmark_jobs"]) {
		status, _ := job["status"].(string)
		if status == "queued" && benchmarkJobDue(job, now) {
			return cloneBenchmarkJob(job), true
		}
	}
	return nil, false
}

func findBenchmarkJob(node appstate.Node, benchmarkID string) (map[string]interface{}, bool) {
	for _, job := range benchmarkJobs(node.Observed["benchmark_jobs"]) {
		id, _ := job["benchmark_id"].(string)
		if id == benchmarkID {
			return cloneBenchmarkJob(job), true
		}
	}
	return nil, false
}

func upsertBenchmarkJob(existing interface{}, job map[string]interface{}) []map[string]interface{} {
	job = cloneBenchmarkJob(job)
	id, _ := job["benchmark_id"].(string)
	out := []map[string]interface{}{job}
	for _, current := range benchmarkJobs(existing) {
		currentID, _ := current["benchmark_id"].(string)
		if id != "" && currentID == id {
			continue
		}
		out = append(out, current)
	}
	if len(out) > maxBenchmarkJobs {
		return out[:maxBenchmarkJobs]
	}
	return out
}

func benchmarkJobs(existing interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	switch values := existing.(type) {
	case []map[string]interface{}:
		for _, value := range values {
			out = append(out, cloneBenchmarkJob(value))
		}
	case []interface{}:
		for _, value := range values {
			item, ok := value.(map[string]interface{})
			if ok {
				out = append(out, cloneBenchmarkJob(item))
			}
		}
	}
	return out
}

func cloneBenchmarkJob(job map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, value := range job {
		out[key] = value
	}
	return out
}

func benchmarkModelCandidates(node appstate.Node) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, model := range node.Models {
		name := safeBenchmarkString(model.Name)
		if name == "" || seen[name] {
			continue
		}
		out = append(out, name)
		seen[name] = true
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func benchmarkJobStatusAllowed(status string) bool {
	switch status {
	case "queued", "running", "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

type benchmarkSchedulerRun struct {
	Changed   bool
	TimedOut  int
	Retried   int
	Exhausted int
}

func (s *Server) reconcileBenchmarkJobs(now time.Time, reason string) benchmarkSchedulerRun {
	state := s.store.Snapshot()
	schedulerPolicy := s.benchmarkSchedulerConfig()
	total := benchmarkSchedulerRun{}
	for _, node := range state.Nodes {
		if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
			continue
		}
		run := reconcileBenchmarkJobsForNode(&node, now, reason, schedulerPolicy)
		if !run.Changed {
			continue
		}
		if err := s.store.UpsertNode(node); err != nil {
			continue
		}
		total.Changed = true
		total.TimedOut += run.TimedOut
		total.Retried += run.Retried
		total.Exhausted += run.Exhausted
		s.tele.Emit("benchmark_scheduler_reconcile", telemetry.Event{
			"node_id":   node.NodeID,
			"timed_out": run.TimedOut,
			"retried":   run.Retried,
			"exhausted": run.Exhausted,
			"reason":    reason,
			"scheduler": benchmarkJobSchedulerPolicy,
		})
	}
	return total
}

func reconcileBenchmarkJobsForNode(node *appstate.Node, now time.Time, reason string, schedulerPolicy config.BenchmarkSchedulerConfig) benchmarkSchedulerRun {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	if node.Observed == nil {
		return benchmarkSchedulerRun{}
	}
	jobs := benchmarkJobs(node.Observed["benchmark_jobs"])
	if len(jobs) == 0 {
		return benchmarkSchedulerRun{}
	}
	run := benchmarkSchedulerRun{}
	for _, job := range jobs {
		status, _ := job["status"].(string)
		switch status {
		case "running":
			timeoutAt, ok := benchmarkJobTime(job["timeout_at"])
			if !ok {
				claimedAt, claimedOK := benchmarkJobTime(job["claimed_at"])
				if claimedOK {
					timeoutAt = claimedAt.Add(benchmarkJobLeaseTimeout(job, schedulerPolicy))
				} else {
					timeoutAt = now
				}
				job["timeout_at"] = timeoutAt
				job["lease_timeout_seconds"] = benchmarkJobInt(job["lease_timeout_seconds"], schedulerPolicy.LeaseTimeoutSeconds)
				run.Changed = true
			}
			if !now.Before(timeoutAt.UTC()) {
				job["timed_out_at"] = now
				job["error_code"] = benchmarkJobTimeoutErrorCode
				run.TimedOut++
				if scheduleBenchmarkJobRetry(job, now, "retry_after_timeout", schedulerPolicy) {
					run.Retried++
				} else {
					run.Exhausted++
				}
				run.Changed = true
			}
		case "failed":
			state, _ := job["scheduler_state"].(string)
			if state == "retry_wait" && benchmarkJobDue(job, now) {
				job["status"] = "queued"
				job["updated_at"] = now
				job["scheduler_state"] = "queued"
				job["scheduler_reason"] = "retry_ready"
				job["last_reconciled_reason"] = reason
				run.Retried++
				run.Changed = true
			}
		case "queued":
			ensureBenchmarkJobSchedulerDefaults(job, now, schedulerPolicy)
		}
	}
	if run.Changed {
		node.Observed["benchmark_jobs"] = jobs
		node.Observed["benchmark_updated_at"] = now
		node.Observed["benchmark_status"] = jobs[0]["status"]
		node.Observed["benchmark_id"] = jobs[0]["benchmark_id"]
	}
	return run
}

func scheduleBenchmarkJobRetry(job map[string]interface{}, now time.Time, reason string, schedulerPolicy config.BenchmarkSchedulerConfig) bool {
	ensureBenchmarkJobSchedulerDefaults(job, now, schedulerPolicy)
	attempt := benchmarkJobInt(job["attempt"], 0)
	maxAttempts := benchmarkJobInt(job["max_attempts"], schedulerPolicy.MaxAttempts)
	job["updated_at"] = now
	job["scheduler_policy"] = benchmarkJobSchedulerPolicy
	if attempt < maxAttempts {
		job["status"] = "failed"
		job["scheduler_state"] = "retry_wait"
		job["scheduler_reason"] = reason
		job["next_attempt_at"] = now.Add(benchmarkJobRetryDelay(job, schedulerPolicy))
		job["retry_delay_seconds"] = benchmarkJobInt(job["retry_delay_seconds"], schedulerPolicy.RetryDelaySeconds)
		return true
	}
	job["status"] = "failed"
	job["scheduler_state"] = "exhausted"
	job["scheduler_reason"] = "max_attempts_exhausted"
	job["completed_at"] = now
	delete(job, "next_attempt_at")
	return false
}

func ensureBenchmarkJobSchedulerDefaults(job map[string]interface{}, now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	if benchmarkJobString(job["scheduler_policy"]) == "" {
		job["scheduler_policy"] = schedulerPolicy.Policy
	}
	jobPolicy := config.NormalizeBenchmarkSchedulerConfig(config.BenchmarkSchedulerConfig{
		Policy:              benchmarkJobString(job["scheduler_policy"]),
		MaxAttempts:         benchmarkJobInt(job["max_attempts"], schedulerPolicy.MaxAttempts),
		LeaseTimeoutSeconds: benchmarkJobInt(job["lease_timeout_seconds"], schedulerPolicy.LeaseTimeoutSeconds),
		RetryDelaySeconds:   benchmarkJobInt(job["retry_delay_seconds"], schedulerPolicy.RetryDelaySeconds),
	})
	job["scheduler_policy"] = jobPolicy.Policy
	job["max_attempts"] = jobPolicy.MaxAttempts
	job["lease_timeout_seconds"] = jobPolicy.LeaseTimeoutSeconds
	job["retry_delay_seconds"] = jobPolicy.RetryDelaySeconds
	if benchmarkJobString(job["scheduler_state"]) == "" {
		job["scheduler_state"] = benchmarkJobString(job["status"])
	}
	if _, ok := benchmarkJobTime(job["next_attempt_at"]); !ok && benchmarkJobString(job["status"]) == "queued" {
		job["next_attempt_at"] = now
	}
}

func benchmarkJobLeaseTimeout(job map[string]interface{}, schedulerPolicy config.BenchmarkSchedulerConfig) time.Duration {
	policy := config.NormalizeBenchmarkSchedulerConfig(config.BenchmarkSchedulerConfig{
		Policy:              benchmarkJobSchedulerPolicy,
		MaxAttempts:         schedulerPolicy.MaxAttempts,
		LeaseTimeoutSeconds: benchmarkJobInt(job["lease_timeout_seconds"], schedulerPolicy.LeaseTimeoutSeconds),
		RetryDelaySeconds:   schedulerPolicy.RetryDelaySeconds,
	})
	seconds := policy.LeaseTimeoutSeconds
	return time.Duration(seconds) * time.Second
}

func benchmarkJobRetryDelay(job map[string]interface{}, schedulerPolicy config.BenchmarkSchedulerConfig) time.Duration {
	policy := config.NormalizeBenchmarkSchedulerConfig(config.BenchmarkSchedulerConfig{
		Policy:              benchmarkJobSchedulerPolicy,
		MaxAttempts:         schedulerPolicy.MaxAttempts,
		LeaseTimeoutSeconds: schedulerPolicy.LeaseTimeoutSeconds,
		RetryDelaySeconds:   benchmarkJobInt(job["retry_delay_seconds"], schedulerPolicy.RetryDelaySeconds),
	})
	seconds := policy.RetryDelaySeconds
	return time.Duration(seconds) * time.Second
}

func benchmarkJobDue(job map[string]interface{}, now time.Time) bool {
	nextAttemptAt, ok := benchmarkJobTime(job["next_attempt_at"])
	return !ok || !now.Before(nextAttemptAt.UTC())
}

func benchmarkJobInt(value interface{}, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case json.Number:
		number, err := typed.Int64()
		if err == nil {
			return int(number)
		}
	default:
		return fallback
	}
	return fallback
}

func benchmarkJobString(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func benchmarkJobTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}
