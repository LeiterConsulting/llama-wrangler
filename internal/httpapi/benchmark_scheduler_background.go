package httpapi

import (
	"context"
	"sync"
	"time"

	"llama-wrangler/internal/config"
	"llama-wrangler/internal/telemetry"
)

const benchmarkJobBackgroundReconcileReason = "background_tick"

const (
	benchmarkSchedulerAuditMaxEntries        = 24
	benchmarkSchedulerAuditTriggerBackground = "background"
	benchmarkSchedulerAuditTriggerOperator   = "operator"
	benchmarkSchedulerAuditTriggerInternal   = "internal"
)

type benchmarkSchedulerBackground struct {
	mu         sync.Mutex
	lastTickAt time.Time
	nextTickAt time.Time
	lastReason string
	lastRun    benchmarkSchedulerRun
	history    []BenchmarkSchedulerAuditEntry
}

func newBenchmarkSchedulerBackground() *benchmarkSchedulerBackground {
	return &benchmarkSchedulerBackground{}
}

func (b *benchmarkSchedulerBackground) resetPlan(now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	b.mu.Lock()
	defer b.mu.Unlock()
	if !schedulerPolicy.BackgroundEnabled {
		b.nextTickAt = time.Time{}
		return
	}
	b.nextTickAt = now.Add(time.Duration(schedulerPolicy.TickIntervalSeconds) * time.Second)
}

func (b *benchmarkSchedulerBackground) due(now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig) bool {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	b.mu.Lock()
	defer b.mu.Unlock()
	if !schedulerPolicy.BackgroundEnabled {
		b.nextTickAt = time.Time{}
		return false
	}
	if b.nextTickAt.IsZero() {
		b.nextTickAt = now.Add(time.Duration(schedulerPolicy.TickIntervalSeconds) * time.Second)
		return false
	}
	return !now.Before(b.nextTickAt.UTC())
}

func (b *benchmarkSchedulerBackground) record(now time.Time, schedulerPolicy config.BenchmarkSchedulerConfig, reason string, run benchmarkSchedulerRun) {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastTickAt = now
	b.nextTickAt = now.Add(time.Duration(schedulerPolicy.TickIntervalSeconds) * time.Second)
	b.lastReason = reason
	b.lastRun = run
	b.appendAuditLocked(now, benchmarkSchedulerAuditTriggerBackground, reason, schedulerPolicy, run)
}

func (b *benchmarkSchedulerBackground) recordAudit(now time.Time, trigger string, reason string, schedulerPolicy config.BenchmarkSchedulerConfig, run benchmarkSchedulerRun) {
	schedulerPolicy = config.NormalizeBenchmarkSchedulerConfig(schedulerPolicy)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.appendAuditLocked(now, trigger, reason, schedulerPolicy, run)
}

func (b *benchmarkSchedulerBackground) appendAuditLocked(now time.Time, trigger string, reason string, schedulerPolicy config.BenchmarkSchedulerConfig, run benchmarkSchedulerRun) {
	entry := BenchmarkSchedulerAuditEntry{
		RecordedAt:          now.UTC().Format(time.RFC3339),
		Trigger:             safeBenchmarkSchedulerAuditTrigger(trigger),
		Reason:              safeBenchmarkSchedulerAuditReason(reason),
		Policy:              schedulerPolicy.Policy,
		BackgroundEnabled:   schedulerPolicy.BackgroundEnabled,
		TickIntervalSeconds: schedulerPolicy.TickIntervalSeconds,
		Changed:             run.Changed,
		TimedOut:            run.TimedOut,
		Retried:             run.Retried,
		Exhausted:           run.Exhausted,
	}
	b.history = append([]BenchmarkSchedulerAuditEntry{entry}, b.history...)
	if len(b.history) > benchmarkSchedulerAuditMaxEntries {
		b.history = b.history[:benchmarkSchedulerAuditMaxEntries]
	}
}

func (b *benchmarkSchedulerBackground) snapshot() (time.Time, time.Time, string, benchmarkSchedulerRun, []BenchmarkSchedulerAuditEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	history := make([]BenchmarkSchedulerAuditEntry, len(b.history))
	copy(history, b.history)
	return b.lastTickAt, b.nextTickAt, b.lastReason, b.lastRun, history
}

func safeBenchmarkSchedulerAuditTrigger(trigger string) string {
	switch trigger {
	case benchmarkSchedulerAuditTriggerBackground, benchmarkSchedulerAuditTriggerOperator:
		return trigger
	default:
		return benchmarkSchedulerAuditTriggerInternal
	}
}

func safeBenchmarkSchedulerAuditReason(reason string) string {
	switch reason {
	case benchmarkJobBackgroundReconcileReason, benchmarkJobManualReconcileReason:
		return reason
	default:
		return "internal_reconcile"
	}
}

func (s *Server) runBenchmarkSchedulerBackground(ctx context.Context) {
	s.benchmarkBackground.resetPlan(time.Now().UTC(), s.benchmarkSchedulerConfig())
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.runBenchmarkSchedulerBackgroundTick(now.UTC(), false)
		}
	}
}

func (s *Server) runBenchmarkSchedulerBackgroundTick(now time.Time, force bool) bool {
	schedulerPolicy := s.benchmarkSchedulerConfig()
	if !schedulerPolicy.BackgroundEnabled {
		s.benchmarkBackground.resetPlan(now, schedulerPolicy)
		return false
	}
	if !force && !s.benchmarkBackground.due(now, schedulerPolicy) {
		return false
	}
	run := s.reconcileBenchmarkJobs(now, benchmarkJobBackgroundReconcileReason)
	s.benchmarkBackground.record(now, schedulerPolicy, benchmarkJobBackgroundReconcileReason, run)
	s.tele.Emit("benchmark_scheduler_background_tick", telemetry.Event{
		"changed":               run.Changed,
		"timed_out":             run.TimedOut,
		"retried":               run.Retried,
		"exhausted":             run.Exhausted,
		"scheduler":             schedulerPolicy.Policy,
		"background_enabled":    schedulerPolicy.BackgroundEnabled,
		"tick_interval_seconds": schedulerPolicy.TickIntervalSeconds,
	})
	return true
}
