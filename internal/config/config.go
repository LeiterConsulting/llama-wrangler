package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       string                 `json:"version" yaml:"version"`
	Server        ServerConfig           `json:"server" yaml:"server"`
	OllamaCompat  FeatureConfig          `json:"ollama_compat" yaml:"ollama_compat"`
	OpenAICompat  FeatureConfig          `json:"openai_compat" yaml:"openai_compat"`
	Auth          AuthConfig             `json:"auth" yaml:"auth"`
	Routing       RoutingConfig          `json:"routing" yaml:"routing"`
	ModelAliases  map[string]ModelAlias  `json:"model_aliases" yaml:"model_aliases"`
	Subscribers   []SubscriberConfig     `json:"subscribers" yaml:"subscribers"`
	Frontier      FrontierConfig         `json:"frontier" yaml:"frontier"`
	Telemetry     TelemetryConfig        `json:"telemetry" yaml:"telemetry"`
	Node          NodeConfig             `json:"node" yaml:"node"`
	Ollama        OllamaConfig           `json:"ollama" yaml:"ollama"`
	Registration  RegistrationConfig     `json:"registration" yaml:"registration"`
	Capabilities  CapabilitiesConfig     `json:"capabilities" yaml:"capabilities"`
	Limits        LimitsConfig           `json:"limits" yaml:"limits"`
	UI            UIConfig               `json:"ui" yaml:"ui"`
	Session       SessionConfig          `json:"session" yaml:"session"`
	Compatibility map[string]interface{} `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
}

type ServerConfig struct {
	Mode   string `json:"mode" yaml:"mode"`
	Listen string `json:"listen" yaml:"listen"`
}

type FeatureConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type AuthConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	APIKeys []APIKey `json:"api_keys" yaml:"api_keys"`
}

type APIKey struct {
	Name   string `json:"name" yaml:"name"`
	Key    string `json:"-" yaml:"key"`
	KeyEnv string `json:"key_env" yaml:"key_env"`
}

type RoutingConfig struct {
	DefaultModelAlias     string               `json:"default_model_alias" yaml:"default_model_alias"`
	DefaultExecutionMode  string               `json:"default_execution_mode" yaml:"default_execution_mode"`
	AllowFallback         bool                 `json:"allow_fallback" yaml:"allow_fallback"`
	RequestTimeoutSec     int                  `json:"request_timeout_seconds" yaml:"request_timeout_seconds"`
	QueueMaxDepth         int                  `json:"queue_max_depth" yaml:"queue_max_depth"`
	QueueSchedulingPolicy string               `json:"queue_scheduling_policy" yaml:"queue_scheduling_policy"`
	QueuePriorityWeights  QueuePriorityWeights `json:"queue_priority_weights" yaml:"queue_priority_weights"`
}

func (r RoutingConfig) Timeout() time.Duration {
	if r.RequestTimeoutSec <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(r.RequestTimeoutSec) * time.Second
}

type QueuePriorityWeights struct {
	High   int `json:"high" yaml:"high"`
	Normal int `json:"normal" yaml:"normal"`
	Low    int `json:"low" yaml:"low"`
}

type ModelAlias struct {
	Strategy        string   `json:"strategy" yaml:"strategy"`
	Candidates      []string `json:"candidates" yaml:"candidates"`
	ExecutionMode   string   `json:"execution_mode" yaml:"execution_mode"`
	MinParticipants int      `json:"min_participants" yaml:"min_participants"`
	MaxParticipants int      `json:"max_participants" yaml:"max_participants"`
	Affinity        string   `json:"affinity" yaml:"affinity"`
	KeepWarm        bool     `json:"keep_warm" yaml:"keep_warm"`
}

type SubscriberConfig struct {
	NodeID string `json:"node_id" yaml:"node_id"`
	URL    string `json:"url" yaml:"url"`
	Token  string `json:"-" yaml:"token"`
}

type FrontierConfig struct {
	Enabled            bool     `json:"enabled" yaml:"enabled"`
	DefaultMode        string   `json:"default_mode" yaml:"default_mode"`
	RequireApprovalFor []string `json:"require_approval_for" yaml:"require_approval_for"`
	LocalOnly          bool     `json:"local_only" yaml:"local_only"`
	Budget             Budget   `json:"budget" yaml:"budget"`
}

type Budget struct {
	DailyLimitUSD      float64 `json:"daily_limit_usd" yaml:"daily_limit_usd"`
	PerRequestLimitUSD float64 `json:"per_request_limit_usd" yaml:"per_request_limit_usd"`
}

type TelemetryConfig struct {
	JSONLogs       bool            `json:"json_logs" yaml:"json_logs"`
	LocalJSONLogs  bool            `json:"local_json_logs" yaml:"local_json_logs"`
	LoggingLevel   string          `json:"logging_level" yaml:"logging_level"`
	SplunkHEC      SplunkHECConfig `json:"splunk_hec" yaml:"splunk_hec"`
	StorePayloads  bool            `json:"store_payloads" yaml:"store_payloads"`
	RedactPayloads bool            `json:"redact_payloads" yaml:"redact_payloads"`
}

type SplunkHECConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	URL       string `json:"url" yaml:"url"`
	Token     string `json:"-" yaml:"token"`
	TokenEnv  string `json:"token_env" yaml:"token_env"`
	Index     string `json:"index" yaml:"index"`
	Source    string `json:"source" yaml:"source"`
	VerifySSL bool   `json:"verify_ssl" yaml:"verify_ssl"`
	Prefix    string `json:"sourcetype_prefix" yaml:"sourcetype_prefix"`
}

type NodeConfig struct {
	NodeID      string   `json:"node_id" yaml:"node_id"`
	DisplayName string   `json:"display_name" yaml:"display_name"`
	Tags        []string `json:"tags" yaml:"tags"`
}

type OllamaConfig struct {
	URL string `json:"url" yaml:"url"`
}

type RegistrationConfig struct {
	MarshalURL             string `json:"marshal_url" yaml:"marshal_url"`
	IntervalSeconds        int    `json:"interval_seconds" yaml:"interval_seconds"`
	EnrollmentToken        string `json:"-" yaml:"enrollment_token"`
	HeartbeatCredential    string `json:"-" yaml:"heartbeat_credential"`
	HeartbeatCredentialEnv string `json:"heartbeat_credential_env" yaml:"heartbeat_credential_env"`
}

type CapabilitiesConfig struct {
	AutoDetect               bool                     `json:"auto_detect" yaml:"auto_detect"`
	BenchmarkOnStart         bool                     `json:"benchmark_on_start" yaml:"benchmark_on_start"`
	BenchmarkIntervalMinutes int                      `json:"benchmark_interval_minutes" yaml:"benchmark_interval_minutes"`
	BenchmarkScheduler       BenchmarkSchedulerConfig `json:"benchmark_scheduler" yaml:"benchmark_scheduler"`
	BenchmarkRunner          BenchmarkRunnerConfig    `json:"benchmark_runner" yaml:"benchmark_runner"`
}

const (
	BenchmarkSchedulerPolicyBoundedRetryTimeout = "bounded_retry_timeout_v1"
	BenchmarkRunnerModeDryRun                   = "dry_run_v1"
	BenchmarkRunnerModeSyntheticBuiltin         = "synthetic_builtin_v1"
	BenchmarkRunnerResultPolicyMetricsOnly      = "metrics_only"

	BenchmarkSchedulerDefaultMaxAttempts         = 3
	BenchmarkSchedulerDefaultLeaseTimeoutSeconds = 600
	BenchmarkSchedulerDefaultRetryDelaySeconds   = 60
	BenchmarkSchedulerDefaultTickIntervalSeconds = 60
	BenchmarkRunnerDefaultPollIntervalSeconds    = 60
	BenchmarkRunnerDefaultMaxJobsPerTick         = 1

	BenchmarkSchedulerMinMaxAttempts         = 1
	BenchmarkSchedulerMaxMaxAttempts         = 10
	BenchmarkSchedulerMinLeaseTimeoutSeconds = 30
	BenchmarkSchedulerMaxLeaseTimeoutSeconds = 3600
	BenchmarkSchedulerMinRetryDelaySeconds   = 5
	BenchmarkSchedulerMaxRetryDelaySeconds   = 1800
	BenchmarkSchedulerMinTickIntervalSeconds = 10
	BenchmarkSchedulerMaxTickIntervalSeconds = 3600
	BenchmarkRunnerMinPollIntervalSeconds    = 5
	BenchmarkRunnerMaxPollIntervalSeconds    = 3600
	BenchmarkRunnerMinMaxJobsPerTick         = 1
	BenchmarkRunnerMaxMaxJobsPerTick         = 5
)

type BenchmarkSchedulerConfig struct {
	Policy              string `json:"policy" yaml:"policy"`
	MaxAttempts         int    `json:"max_attempts" yaml:"max_attempts"`
	LeaseTimeoutSeconds int    `json:"lease_timeout_seconds" yaml:"lease_timeout_seconds"`
	RetryDelaySeconds   int    `json:"retry_delay_seconds" yaml:"retry_delay_seconds"`
	BackgroundEnabled   bool   `json:"background_enabled" yaml:"background_enabled"`
	TickIntervalSeconds int    `json:"tick_interval_seconds" yaml:"tick_interval_seconds"`
}

type BenchmarkRunnerConfig struct {
	Enabled             bool   `json:"enabled" yaml:"enabled"`
	Mode                string `json:"mode" yaml:"mode"`
	PollIntervalSeconds int    `json:"poll_interval_seconds" yaml:"poll_interval_seconds"`
	MaxJobsPerTick      int    `json:"max_jobs_per_tick" yaml:"max_jobs_per_tick"`
	ResultBodyPolicy    string `json:"result_body_policy" yaml:"result_body_policy"`
}

type LimitsConfig struct {
	MaxConcurrentJobs int  `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`
	MaxContextTokens  int  `json:"max_context_tokens" yaml:"max_context_tokens"`
	AllowModelPull    bool `json:"allow_model_pull" yaml:"allow_model_pull"`
}

type UIConfig struct {
	Enabled     bool `json:"enabled" yaml:"enabled"`
	RequireAuth bool `json:"require_auth" yaml:"require_auth"`
}

type SessionConfig struct {
	DefaultAffinity string `json:"default_affinity" yaml:"default_affinity"`
}

func Default(mode string) Config {
	listen := "127.0.0.1:11435"
	if mode == "subscriber" {
		listen = "127.0.0.1:11436"
	}
	return Config{
		Version:      "0.1.0",
		Server:       ServerConfig{Mode: mode, Listen: listen},
		OllamaCompat: FeatureConfig{Enabled: true},
		OpenAICompat: FeatureConfig{Enabled: true},
		Auth:         AuthConfig{Enabled: false},
		Routing: RoutingConfig{
			DefaultModelAlias:     "local-fast",
			DefaultExecutionMode:  "single",
			AllowFallback:         true,
			RequestTimeoutSec:     300,
			QueueMaxDepth:         128,
			QueueSchedulingPolicy: "weighted_priority",
			QueuePriorityWeights:  QueuePriorityWeights{High: 3, Normal: 2, Low: 1},
		},
		ModelAliases: map[string]ModelAlias{
			"local-fast":      {Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single", Affinity: "soft"},
			"local-code":      {Strategy: "best_code_node", Candidates: []string{"qwen2.5-coder:14b", "llama3.1:8b"}, ExecutionMode: "single", Affinity: "soft"},
			"local-consensus": {Strategy: "consensus", Candidates: []string{"qwen2.5-coder:14b", "llama3.1:8b"}, ExecutionMode: "consensus", MinParticipants: 2, MaxParticipants: 4, Affinity: "task"},
		},
		Frontier: FrontierConfig{Enabled: false, LocalOnly: true, DefaultMode: "frontier_delta"},
		Telemetry: TelemetryConfig{
			JSONLogs:      true,
			LoggingLevel:  "metadata_only",
			StorePayloads: false,
			SplunkHEC: SplunkHECConfig{
				Enabled:   false,
				Index:     "llama_wrangler",
				Source:    "llama-wrangler",
				VerifySSL: true,
				Prefix:    "llama_wrangler",
			},
		},
		Node:   NodeConfig{NodeID: "local"},
		Ollama: OllamaConfig{URL: "http://localhost:11434"},
		Capabilities: CapabilitiesConfig{
			AutoDetect:         true,
			BenchmarkScheduler: DefaultBenchmarkSchedulerConfig(),
			BenchmarkRunner:    DefaultBenchmarkRunnerConfig(),
		},
		Limits:  LimitsConfig{MaxConcurrentJobs: 2, MaxContextTokens: 32768, AllowModelPull: false},
		UI:      UIConfig{Enabled: true, RequireAuth: true},
		Session: SessionConfig{DefaultAffinity: "soft"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default("marshal")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	Normalize(&cfg)
	resolveEnv(&cfg)
	return cfg, nil
}

func Normalize(cfg *Config) {
	def := Default(cfg.Server.Mode)
	if cfg.Version == "" {
		cfg.Version = def.Version
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = def.Server.Listen
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = def.Server.Mode
	}
	if cfg.Routing.DefaultModelAlias == "" {
		cfg.Routing.DefaultModelAlias = def.Routing.DefaultModelAlias
	}
	if cfg.Routing.DefaultExecutionMode == "" {
		cfg.Routing.DefaultExecutionMode = def.Routing.DefaultExecutionMode
	}
	if cfg.Routing.RequestTimeoutSec == 0 {
		cfg.Routing.RequestTimeoutSec = def.Routing.RequestTimeoutSec
	}
	if cfg.Routing.QueueMaxDepth == 0 {
		cfg.Routing.QueueMaxDepth = def.Routing.QueueMaxDepth
	}
	if cfg.Routing.QueueSchedulingPolicy == "" {
		cfg.Routing.QueueSchedulingPolicy = def.Routing.QueueSchedulingPolicy
	}
	if cfg.Routing.QueuePriorityWeights.High <= 0 {
		cfg.Routing.QueuePriorityWeights.High = def.Routing.QueuePriorityWeights.High
	}
	if cfg.Routing.QueuePriorityWeights.Normal <= 0 {
		cfg.Routing.QueuePriorityWeights.Normal = def.Routing.QueuePriorityWeights.Normal
	}
	if cfg.Routing.QueuePriorityWeights.Low <= 0 {
		cfg.Routing.QueuePriorityWeights.Low = def.Routing.QueuePriorityWeights.Low
	}
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = def.ModelAliases
	}
	if cfg.Ollama.URL == "" {
		cfg.Ollama.URL = def.Ollama.URL
	}
	if cfg.Telemetry.LoggingLevel == "" {
		cfg.Telemetry.LoggingLevel = "metadata_only"
	}
	if cfg.Telemetry.SplunkHEC.Index == "" {
		cfg.Telemetry.SplunkHEC.Index = "llama_wrangler"
	}
	if cfg.Telemetry.SplunkHEC.Source == "" {
		cfg.Telemetry.SplunkHEC.Source = "llama-wrangler"
	}
	if cfg.Telemetry.SplunkHEC.Prefix == "" {
		cfg.Telemetry.SplunkHEC.Prefix = "llama_wrangler"
	}
	if cfg.Session.DefaultAffinity == "" {
		cfg.Session.DefaultAffinity = "soft"
	}
	if cfg.UI.Enabled == false {
		cfg.UI.Enabled = true
	}
	cfg.Capabilities.BenchmarkScheduler = NormalizeBenchmarkSchedulerConfig(cfg.Capabilities.BenchmarkScheduler)
	cfg.Capabilities.BenchmarkRunner = NormalizeBenchmarkRunnerConfig(cfg.Capabilities.BenchmarkRunner)
}

func DefaultBenchmarkSchedulerConfig() BenchmarkSchedulerConfig {
	return BenchmarkSchedulerConfig{
		Policy:              BenchmarkSchedulerPolicyBoundedRetryTimeout,
		MaxAttempts:         BenchmarkSchedulerDefaultMaxAttempts,
		LeaseTimeoutSeconds: BenchmarkSchedulerDefaultLeaseTimeoutSeconds,
		RetryDelaySeconds:   BenchmarkSchedulerDefaultRetryDelaySeconds,
		BackgroundEnabled:   false,
		TickIntervalSeconds: BenchmarkSchedulerDefaultTickIntervalSeconds,
	}
}

func NormalizeBenchmarkSchedulerConfig(policy BenchmarkSchedulerConfig) BenchmarkSchedulerConfig {
	defaults := DefaultBenchmarkSchedulerConfig()
	if policy.Policy != BenchmarkSchedulerPolicyBoundedRetryTimeout {
		policy.Policy = defaults.Policy
	}
	policy.MaxAttempts = boundedInt(policy.MaxAttempts, defaults.MaxAttempts, BenchmarkSchedulerMinMaxAttempts, BenchmarkSchedulerMaxMaxAttempts)
	policy.LeaseTimeoutSeconds = boundedInt(policy.LeaseTimeoutSeconds, defaults.LeaseTimeoutSeconds, BenchmarkSchedulerMinLeaseTimeoutSeconds, BenchmarkSchedulerMaxLeaseTimeoutSeconds)
	policy.RetryDelaySeconds = boundedInt(policy.RetryDelaySeconds, defaults.RetryDelaySeconds, BenchmarkSchedulerMinRetryDelaySeconds, BenchmarkSchedulerMaxRetryDelaySeconds)
	policy.TickIntervalSeconds = boundedInt(policy.TickIntervalSeconds, defaults.TickIntervalSeconds, BenchmarkSchedulerMinTickIntervalSeconds, BenchmarkSchedulerMaxTickIntervalSeconds)
	return policy
}

func DefaultBenchmarkRunnerConfig() BenchmarkRunnerConfig {
	return BenchmarkRunnerConfig{
		Enabled:             false,
		Mode:                BenchmarkRunnerModeDryRun,
		PollIntervalSeconds: BenchmarkRunnerDefaultPollIntervalSeconds,
		MaxJobsPerTick:      BenchmarkRunnerDefaultMaxJobsPerTick,
		ResultBodyPolicy:    BenchmarkRunnerResultPolicyMetricsOnly,
	}
}

func NormalizeBenchmarkRunnerConfig(runner BenchmarkRunnerConfig) BenchmarkRunnerConfig {
	defaults := DefaultBenchmarkRunnerConfig()
	switch runner.Mode {
	case BenchmarkRunnerModeDryRun, BenchmarkRunnerModeSyntheticBuiltin:
	default:
		runner.Mode = defaults.Mode
	}
	runner.PollIntervalSeconds = boundedInt(runner.PollIntervalSeconds, defaults.PollIntervalSeconds, BenchmarkRunnerMinPollIntervalSeconds, BenchmarkRunnerMaxPollIntervalSeconds)
	runner.MaxJobsPerTick = boundedInt(runner.MaxJobsPerTick, defaults.MaxJobsPerTick, BenchmarkRunnerMinMaxJobsPerTick, BenchmarkRunnerMaxMaxJobsPerTick)
	if runner.ResultBodyPolicy != BenchmarkRunnerResultPolicyMetricsOnly {
		runner.ResultBodyPolicy = defaults.ResultBodyPolicy
	}
	return runner
}

func boundedInt(value int, fallback int, min int, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func resolveEnv(cfg *Config) {
	for i := range cfg.Auth.APIKeys {
		if cfg.Auth.APIKeys[i].Key == "" && cfg.Auth.APIKeys[i].KeyEnv != "" {
			cfg.Auth.APIKeys[i].Key = os.Getenv(cfg.Auth.APIKeys[i].KeyEnv)
		}
	}
	if cfg.Telemetry.SplunkHEC.Token == "" && cfg.Telemetry.SplunkHEC.TokenEnv != "" {
		cfg.Telemetry.SplunkHEC.Token = os.Getenv(cfg.Telemetry.SplunkHEC.TokenEnv)
	}
	if cfg.Registration.HeartbeatCredential == "" && cfg.Registration.HeartbeatCredentialEnv != "" {
		cfg.Registration.HeartbeatCredential = os.Getenv(cfg.Registration.HeartbeatCredentialEnv)
	}
}
