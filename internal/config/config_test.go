package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMarshalExample(t *testing.T) {
	t.Setenv("SPLUNK_HEC_TOKEN", "token")
	t.Setenv("LLAMA_WRANGLER_IDE_KEY", "ide-token")
	cfg, err := Load(filepath.Join("..", "..", "configs", "marshal.example.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Mode != "marshal" {
		t.Fatalf("mode = %q, want marshal", cfg.Server.Mode)
	}
	if cfg.Routing.DefaultExecutionMode != "single" {
		t.Fatalf("default mode = %q", cfg.Routing.DefaultExecutionMode)
	}
	if cfg.Routing.QueueSchedulingPolicy != "weighted_priority" {
		t.Fatalf("queue scheduling policy = %q", cfg.Routing.QueueSchedulingPolicy)
	}
	if cfg.Routing.QueuePriorityWeights.High != 3 || cfg.Routing.QueuePriorityWeights.Normal != 2 || cfg.Routing.QueuePriorityWeights.Low != 1 {
		t.Fatalf("queue priority weights = %+v", cfg.Routing.QueuePriorityWeights)
	}
	if cfg.Capabilities.BenchmarkScheduler.Policy != BenchmarkSchedulerPolicyBoundedRetryTimeout ||
		cfg.Capabilities.BenchmarkScheduler.MaxAttempts != 3 ||
		cfg.Capabilities.BenchmarkScheduler.LeaseTimeoutSeconds != 600 ||
		cfg.Capabilities.BenchmarkScheduler.RetryDelaySeconds != 60 ||
		cfg.Capabilities.BenchmarkScheduler.BackgroundEnabled ||
		cfg.Capabilities.BenchmarkScheduler.TickIntervalSeconds != 60 {
		t.Fatalf("benchmark scheduler config = %+v", cfg.Capabilities.BenchmarkScheduler)
	}
	if cfg.Telemetry.SplunkHEC.Token != "token" {
		t.Fatalf("HEC token was not resolved from env")
	}
	if cfg.Auth.APIKeys[0].Key != "ide-token" {
		t.Fatalf("API key was not resolved from env")
	}
}

func TestSubscriberHeartbeatCredentialEnvResolution(t *testing.T) {
	t.Setenv("LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL", "lw_hb_test_secret")
	cfg, err := Load(filepath.Join("..", "..", "configs", "subscriber.example.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Mode != "subscriber" {
		t.Fatalf("mode = %q, want subscriber", cfg.Server.Mode)
	}
	if cfg.Registration.HeartbeatCredentialEnv != "LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL" {
		t.Fatalf("heartbeat credential env = %q", cfg.Registration.HeartbeatCredentialEnv)
	}
	if cfg.Registration.HeartbeatCredential != "lw_hb_test_secret" {
		t.Fatalf("heartbeat credential was not resolved from env")
	}
	if cfg.Capabilities.BenchmarkRunner.Enabled ||
		cfg.Capabilities.BenchmarkRunner.Mode != BenchmarkRunnerModeDryRun ||
		cfg.Capabilities.BenchmarkRunner.PollIntervalSeconds != BenchmarkRunnerDefaultPollIntervalSeconds ||
		cfg.Capabilities.BenchmarkRunner.MaxJobsPerTick != BenchmarkRunnerDefaultMaxJobsPerTick ||
		cfg.Capabilities.BenchmarkRunner.ResultBodyPolicy != BenchmarkRunnerResultPolicyMetricsOnly {
		t.Fatalf("benchmark runner config = %+v", cfg.Capabilities.BenchmarkRunner)
	}
}

func TestDefaultSafePosture(t *testing.T) {
	cfg := Default("marshal")
	if cfg.Server.Listen != "127.0.0.1:11435" {
		t.Fatalf("default listen = %q, want localhost marshal port", cfg.Server.Listen)
	}
	if cfg.Frontier.Enabled {
		t.Fatalf("frontier should be disabled by default")
	}
	if !cfg.Frontier.LocalOnly {
		t.Fatalf("local_only should be true by default")
	}
	if cfg.Telemetry.StorePayloads {
		t.Fatalf("payload logging should be disabled by default")
	}
	if cfg.Routing.QueueSchedulingPolicy != "weighted_priority" {
		t.Fatalf("default queue scheduling policy = %q", cfg.Routing.QueueSchedulingPolicy)
	}
	if cfg.Capabilities.BenchmarkScheduler != DefaultBenchmarkSchedulerConfig() {
		t.Fatalf("default benchmark scheduler = %+v", cfg.Capabilities.BenchmarkScheduler)
	}
	if cfg.Capabilities.BenchmarkRunner != DefaultBenchmarkRunnerConfig() {
		t.Fatalf("default benchmark runner = %+v", cfg.Capabilities.BenchmarkRunner)
	}
}

func TestBenchmarkSchedulerNormalizationBounds(t *testing.T) {
	cfg := Default("marshal")
	cfg.Capabilities.BenchmarkScheduler = BenchmarkSchedulerConfig{
		Policy:              "future_unbounded_policy",
		MaxAttempts:         99,
		LeaseTimeoutSeconds: 1,
		RetryDelaySeconds:   99999,
		BackgroundEnabled:   true,
		TickIntervalSeconds: 1,
	}
	Normalize(&cfg)
	if cfg.Capabilities.BenchmarkScheduler.Policy != BenchmarkSchedulerPolicyBoundedRetryTimeout {
		t.Fatalf("policy = %q", cfg.Capabilities.BenchmarkScheduler.Policy)
	}
	if cfg.Capabilities.BenchmarkScheduler.MaxAttempts != BenchmarkSchedulerMaxMaxAttempts {
		t.Fatalf("max attempts = %d", cfg.Capabilities.BenchmarkScheduler.MaxAttempts)
	}
	if cfg.Capabilities.BenchmarkScheduler.LeaseTimeoutSeconds != BenchmarkSchedulerMinLeaseTimeoutSeconds {
		t.Fatalf("lease timeout = %d", cfg.Capabilities.BenchmarkScheduler.LeaseTimeoutSeconds)
	}
	if cfg.Capabilities.BenchmarkScheduler.RetryDelaySeconds != BenchmarkSchedulerMaxRetryDelaySeconds {
		t.Fatalf("retry delay = %d", cfg.Capabilities.BenchmarkScheduler.RetryDelaySeconds)
	}
	if !cfg.Capabilities.BenchmarkScheduler.BackgroundEnabled {
		t.Fatalf("background scheduler enablement was not preserved")
	}
	if cfg.Capabilities.BenchmarkScheduler.TickIntervalSeconds != BenchmarkSchedulerMinTickIntervalSeconds {
		t.Fatalf("tick interval = %d", cfg.Capabilities.BenchmarkScheduler.TickIntervalSeconds)
	}
}

func TestBenchmarkRunnerNormalizationBounds(t *testing.T) {
	cfg := Default("subscriber")
	cfg.Capabilities.BenchmarkRunner = BenchmarkRunnerConfig{
		Enabled:             true,
		Mode:                "future_remote_prompt_runner",
		PollIntervalSeconds: 1,
		MaxJobsPerTick:      99,
		ResultBodyPolicy:    "include_payloads",
	}
	Normalize(&cfg)
	if !cfg.Capabilities.BenchmarkRunner.Enabled {
		t.Fatalf("benchmark runner enablement was not preserved")
	}
	if cfg.Capabilities.BenchmarkRunner.Mode != BenchmarkRunnerModeDryRun {
		t.Fatalf("runner mode = %q", cfg.Capabilities.BenchmarkRunner.Mode)
	}
	if cfg.Capabilities.BenchmarkRunner.PollIntervalSeconds != BenchmarkRunnerMinPollIntervalSeconds {
		t.Fatalf("poll interval = %d", cfg.Capabilities.BenchmarkRunner.PollIntervalSeconds)
	}
	if cfg.Capabilities.BenchmarkRunner.MaxJobsPerTick != BenchmarkRunnerMaxMaxJobsPerTick {
		t.Fatalf("max jobs per tick = %d", cfg.Capabilities.BenchmarkRunner.MaxJobsPerTick)
	}
	if cfg.Capabilities.BenchmarkRunner.ResultBodyPolicy != BenchmarkRunnerResultPolicyMetricsOnly {
		t.Fatalf("result body policy = %q", cfg.Capabilities.BenchmarkRunner.ResultBodyPolicy)
	}
	cfg.Capabilities.BenchmarkRunner = BenchmarkRunnerConfig{
		Enabled:             true,
		Mode:                BenchmarkRunnerModeSyntheticBuiltin,
		PollIntervalSeconds: 30,
		MaxJobsPerTick:      2,
		ResultBodyPolicy:    BenchmarkRunnerResultPolicyMetricsOnly,
	}
	Normalize(&cfg)
	if cfg.Capabilities.BenchmarkRunner.Mode != BenchmarkRunnerModeSyntheticBuiltin {
		t.Fatalf("synthetic runner mode was not preserved: %+v", cfg.Capabilities.BenchmarkRunner)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(os.TempDir(), "no-such-llama-wrangler.yaml"))
	if err == nil {
		t.Fatalf("Load() expected error")
	}
}
