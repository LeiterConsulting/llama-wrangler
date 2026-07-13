package httpapi

import (
	"sort"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

type BenchmarkSchedulerStatus struct {
	Window     string                             `json:"window"`
	Policy     string                             `json:"policy"`
	Config     config.BenchmarkSchedulerConfig    `json:"config"`
	Limits     BenchmarkSchedulerPolicyLimits     `json:"limits"`
	Background BenchmarkSchedulerBackgroundStatus `json:"background"`
	History    BenchmarkSchedulerHistoryStatus    `json:"history"`
	Summary    map[string]int                     `json:"summary"`
	Jobs       []BenchmarkSchedulerJob            `json:"jobs"`
}

type BenchmarkSchedulerPolicyLimits struct {
	MaxAttemptsMin         int `json:"max_attempts_min"`
	MaxAttemptsMax         int `json:"max_attempts_max"`
	LeaseTimeoutSecondsMin int `json:"lease_timeout_seconds_min"`
	LeaseTimeoutSecondsMax int `json:"lease_timeout_seconds_max"`
	RetryDelaySecondsMin   int `json:"retry_delay_seconds_min"`
	RetryDelaySecondsMax   int `json:"retry_delay_seconds_max"`
	TickIntervalSecondsMin int `json:"tick_interval_seconds_min"`
	TickIntervalSecondsMax int `json:"tick_interval_seconds_max"`
}

type BenchmarkSchedulerBackgroundStatus struct {
	Enabled             bool   `json:"enabled"`
	TickIntervalSeconds int    `json:"tick_interval_seconds"`
	LastTickAt          string `json:"last_tick_at,omitempty"`
	NextTickAt          string `json:"next_tick_at,omitempty"`
	LastReason          string `json:"last_reason,omitempty"`
	LastChanged         bool   `json:"last_changed"`
	LastTimedOut        int    `json:"last_timed_out"`
	LastRetried         int    `json:"last_retried"`
	LastExhausted       int    `json:"last_exhausted"`
}

type BenchmarkSchedulerHistoryStatus struct {
	Window     string                         `json:"window"`
	Retention  string                         `json:"retention"`
	MaxEntries int                            `json:"max_entries"`
	Count      int                            `json:"count"`
	Summary    map[string]int                 `json:"summary"`
	Entries    []BenchmarkSchedulerAuditEntry `json:"entries"`
}

type BenchmarkSchedulerAuditEntry struct {
	RecordedAt          string `json:"recorded_at"`
	Trigger             string `json:"trigger"`
	Reason              string `json:"reason"`
	Policy              string `json:"policy"`
	BackgroundEnabled   bool   `json:"background_enabled"`
	TickIntervalSeconds int    `json:"tick_interval_seconds"`
	Changed             bool   `json:"changed"`
	TimedOut            int    `json:"timed_out"`
	Retried             int    `json:"retried"`
	Exhausted           int    `json:"exhausted"`
}

type BenchmarkSchedulerJob struct {
	NodeID              string `json:"node_id"`
	BenchmarkID         string `json:"benchmark_id"`
	Status              string `json:"status"`
	SchedulerState      string `json:"scheduler_state"`
	SchedulerReason     string `json:"scheduler_reason"`
	Attempt             int    `json:"attempt"`
	MaxAttempts         int    `json:"max_attempts"`
	LeaseTimeoutSeconds int    `json:"lease_timeout_seconds"`
	RetryDelaySeconds   int    `json:"retry_delay_seconds"`
	WorkloadSuiteID     string `json:"workload_suite_id,omitempty"`
	WorkloadSource      string `json:"workload_source,omitempty"`
	NextAttemptAt       string `json:"next_attempt_at,omitempty"`
	TimeoutAt           string `json:"timeout_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}

func (s *Server) benchmarkSchedulerStatus() BenchmarkSchedulerStatus {
	return s.benchmarkSchedulerStatusAt(time.Now().UTC())
}

func (s *Server) benchmarkSchedulerStatusAt(now time.Time) BenchmarkSchedulerStatus {
	return s.summarizeBenchmarkScheduler(now)
}

func (s *Server) summarizeBenchmarkScheduler(now time.Time) BenchmarkSchedulerStatus {
	schedulerPolicy := s.benchmarkSchedulerConfig()
	status := summarizeBenchmarkScheduler(s.store.Snapshot().Nodes, now, schedulerPolicy)
	status.Background = s.benchmarkSchedulerBackgroundStatusAt(now, schedulerPolicy)
	status.History = s.benchmarkSchedulerHistoryStatus()
	return status
}

func summarizeBenchmarkScheduler(nodes map[string]appstate.Node, now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) BenchmarkSchedulerStatus {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	status := BenchmarkSchedulerStatus{
		Window: "current_benchmark_jobs",
		Policy: schedulerPolicy.Policy,
		Config: schedulerPolicy,
		Limits: benchmarkSchedulerPolicyLimits(),
		Background: BenchmarkSchedulerBackgroundStatus{
			Enabled:             schedulerPolicy.BackgroundEnabled,
			TickIntervalSeconds: schedulerPolicy.TickIntervalSeconds,
		},
		Summary: map[string]int{},
		Jobs:    []BenchmarkSchedulerJob{},
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		node := nodes[id]
		if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
			continue
		}
		for _, job := range benchmarkJobs(node.Observed["benchmark_jobs"]) {
			item := benchmarkSchedulerJobStatus(node.NodeID, job, now, schedulerPolicy)
			status.Jobs = append(status.Jobs, item)
			status.Summary["jobs"]++
			status.Summary[item.Status]++
			status.Summary[item.SchedulerState]++
			if item.Status == "queued" && benchmarkJobDue(job, now) {
				status.Summary["claimable"]++
			}
			if item.Status == "running" {
				if timeoutAt, ok := benchmarkJobTime(job["timeout_at"]); ok && !now.Before(timeoutAt.UTC()) {
					status.Summary["timeout_due"]++
				}
			}
			if item.SchedulerState == "retry_wait" {
				status.Summary["retry_wait"]++
			}
			if item.SchedulerState == "exhausted" {
				status.Summary["exhausted"]++
			}
		}
	}
	return status
}

func benchmarkSchedulerJobStatus(nodeID string, job map[string]interface{}, now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) BenchmarkSchedulerJob {
	ensureBenchmarkJobSchedulerDefaults(job, now, schedulerPolicy)
	item := BenchmarkSchedulerJob{
		NodeID:              nodeID,
		BenchmarkID:         benchmarkJobString(job["benchmark_id"]),
		Status:              benchmarkJobString(job["status"]),
		SchedulerState:      benchmarkJobString(job["scheduler_state"]),
		SchedulerReason:     benchmarkJobString(job["scheduler_reason"]),
		Attempt:             benchmarkJobInt(job["attempt"], 0),
		MaxAttempts:         benchmarkJobInt(job["max_attempts"], schedulerPolicy.MaxAttempts),
		LeaseTimeoutSeconds: benchmarkJobInt(job["lease_timeout_seconds"], schedulerPolicy.LeaseTimeoutSeconds),
		RetryDelaySeconds:   benchmarkJobInt(job["retry_delay_seconds"], schedulerPolicy.RetryDelaySeconds),
	}
	if suite, ok := job["workload_suite"].(map[string]interface{}); ok {
		item.WorkloadSuiteID = benchmarkJobString(suite["suite_id"])
		item.WorkloadSource = benchmarkJobString(suite["source"])
	}
	if value, ok := benchmarkJobTime(job["next_attempt_at"]); ok {
		item.NextAttemptAt = value.UTC().Format(time.RFC3339)
	}
	if value, ok := benchmarkJobTime(job["timeout_at"]); ok {
		item.TimeoutAt = value.UTC().Format(time.RFC3339)
	}
	if value, ok := benchmarkJobTime(job["updated_at"]); ok {
		item.UpdatedAt = value.UTC().Format(time.RFC3339)
	}
	return item
}

func benchmarkSchedulerPolicyLimits() BenchmarkSchedulerPolicyLimits {
	return BenchmarkSchedulerPolicyLimits{
		MaxAttemptsMin:         config.BenchmarkSchedulerMinMaxAttempts,
		MaxAttemptsMax:         config.BenchmarkSchedulerMaxMaxAttempts,
		LeaseTimeoutSecondsMin: config.BenchmarkSchedulerMinLeaseTimeoutSeconds,
		LeaseTimeoutSecondsMax: config.BenchmarkSchedulerMaxLeaseTimeoutSeconds,
		RetryDelaySecondsMin:   config.BenchmarkSchedulerMinRetryDelaySeconds,
		RetryDelaySecondsMax:   config.BenchmarkSchedulerMaxRetryDelaySeconds,
		TickIntervalSecondsMin: config.BenchmarkSchedulerMinTickIntervalSeconds,
		TickIntervalSecondsMax: config.BenchmarkSchedulerMaxTickIntervalSeconds,
	}
}

func (s *Server) benchmarkSchedulerBackgroundStatusAt(now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) BenchmarkSchedulerBackgroundStatus {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	status := BenchmarkSchedulerBackgroundStatus{
		Enabled:             schedulerPolicy.BackgroundEnabled,
		TickIntervalSeconds: schedulerPolicy.TickIntervalSeconds,
	}
	if s.benchmarkBackground == nil {
		return status
	}
	lastTickAt, nextTickAt, lastReason, lastRun, _ := s.benchmarkBackground.snapshot()
	if status.Enabled && nextTickAt.IsZero() {
		nextTickAt = now.Add(time.Duration(status.TickIntervalSeconds) * time.Second)
	}
	if !lastTickAt.IsZero() {
		status.LastTickAt = lastTickAt.UTC().Format(time.RFC3339)
	}
	if status.Enabled && !nextTickAt.IsZero() {
		status.NextTickAt = nextTickAt.UTC().Format(time.RFC3339)
	}
	status.LastReason = lastReason
	status.LastChanged = lastRun.Changed
	status.LastTimedOut = lastRun.TimedOut
	status.LastRetried = lastRun.Retried
	status.LastExhausted = lastRun.Exhausted
	return status
}

func (s *Server) benchmarkSchedulerHistoryStatus() BenchmarkSchedulerHistoryStatus {
	status := BenchmarkSchedulerHistoryStatus{
		Window:     "recent_reconciliations",
		Retention:  "process_local_reset_on_restart",
		MaxEntries: benchmarkSchedulerAuditMaxEntries,
		Summary:    map[string]int{},
		Entries:    []BenchmarkSchedulerAuditEntry{},
	}
	if s.benchmarkBackground == nil {
		return status
	}
	_, _, _, _, status.Entries = s.benchmarkBackground.snapshot()
	status.Count = len(status.Entries)
	for _, entry := range status.Entries {
		status.Summary[entry.Trigger]++
		if entry.Changed {
			status.Summary["changed_runs"]++
		}
		status.Summary["timed_out"] += entry.TimedOut
		status.Summary["retried"] += entry.Retried
		status.Summary["exhausted"] += entry.Exhausted
	}
	return status
}
