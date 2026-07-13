package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"llama-wrangler/internal/config"
)

const (
	benchmarkRunnerStatusDisabled         = "disabled_by_default"
	benchmarkRunnerStatusEnabledDryRun    = "enabled_dry_run"
	benchmarkRunnerStatusEnabledSynthetic = "enabled_synthetic_builtin"
	benchmarkRunnerNodeIDPlaceholder      = "<node-id>"
	benchmarkRunnerCredentialPlaceholder  = "<subscriber-heartbeat-credential>"
	benchmarkRunnerBenchmarkIDPlaceholder = "<benchmark-id-from-claim>"
	benchmarkRunnerFixtureDirEnv          = "LLAMA_WRANGLER_BENCHMARK_FIXTURE_MANIFEST_DIR"
	benchmarkRunnerEnabledEnv             = "LLAMA_WRANGLER_BENCHMARK_RUNNER_ENABLED"
)

type BenchmarkRunnerGuidance struct {
	Status                string                         `json:"status"`
	RunnerImplementation  string                         `json:"runner_implementation"`
	ControlLevel          string                         `json:"control_level"`
	TrustBoundary         string                         `json:"trust_boundary"`
	SupportedSuiteIDs     []string                       `json:"supported_suite_ids"`
	ClaimEndpoint         string                         `json:"claim_endpoint"`
	StatusEndpoint        string                         `json:"status_endpoint"`
	ResultEndpoint        string                         `json:"result_endpoint"`
	HeaderName            string                         `json:"header_name"`
	CredentialPlaceholder string                         `json:"credential_placeholder"`
	Environment           map[string]string              `json:"environment"`
	ConfigSnippet         string                         `json:"config_snippet"`
	RunnerConfig          BenchmarkRunnerConfigSummary   `json:"runner_config"`
	ExecutionFlow         []string                       `json:"execution_flow"`
	MetricFields          []string                       `json:"metric_fields"`
	Commands              BenchmarkRunnerCommands        `json:"commands"`
	PackagingHooks        BenchmarkRunnerPackagingHooks  `json:"packaging_hooks"`
	LocalFixtureGuidance  BenchmarkRunnerFixtureGuidance `json:"local_fixture_guidance"`
	Warnings              []string                       `json:"warnings"`
}

type BenchmarkRunnerConfigSummary struct {
	Enabled             bool   `json:"enabled"`
	Mode                string `json:"mode"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	MaxJobsPerTick      int    `json:"max_jobs_per_tick"`
	ResultBodyPolicy    string `json:"result_body_policy"`
}

type BenchmarkRunnerCommands struct {
	ClaimJob     string `json:"claim_job"`
	ReportStatus string `json:"report_status"`
	ReportResult string `json:"report_result"`
}

type BenchmarkRunnerPackagingHooks struct {
	RunnerLoopStatus string   `json:"runner_loop_status"`
	ExpectedInputs   []string `json:"expected_inputs"`
	ExpectedOutputs  []string `json:"expected_outputs"`
	InstallNotes     []string `json:"install_notes"`
}

type BenchmarkRunnerFixtureGuidance struct {
	SuiteID             string   `json:"suite_id"`
	ManifestIDField     string   `json:"manifest_id_field"`
	FixtureDirectoryEnv string   `json:"fixture_directory_env"`
	AllowedReferences   []string `json:"allowed_references"`
	StoragePolicy       string   `json:"storage_policy"`
}

func (s *Server) benchmarkRunnerGuidanceHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.buildBenchmarkRunnerGuidance(requestBaseURL(r)))
}

func (s *Server) buildBenchmarkRunnerGuidance(baseURL string) BenchmarkRunnerGuidance {
	runnerConfig := s.benchmarkRunnerConfig()
	return buildBenchmarkRunnerGuidance(baseURL, runnerConfig)
}

func buildBenchmarkRunnerGuidance(baseURL string, runnerConfig config.BenchmarkRunnerConfig) BenchmarkRunnerGuidance {
	runnerConfig = config.NormalizeBenchmarkRunnerConfig(runnerConfig)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = marshalURLPlaceholder
	}
	supportedSuiteIDs := benchmarkRunnerSupportedSuiteIDs()
	claimBody, _ := json.Marshal(map[string]string{"node_id": benchmarkRunnerNodeIDPlaceholder})
	statusBody, _ := json.Marshal(map[string]string{
		"node_id":      benchmarkRunnerNodeIDPlaceholder,
		"benchmark_id": benchmarkRunnerBenchmarkIDPlaceholder,
		"status":       "running",
	})
	resultBody, _ := json.Marshal(map[string]interface{}{
		"node_id":           benchmarkRunnerNodeIDPlaceholder,
		"benchmark_id":      benchmarkRunnerBenchmarkIDPlaceholder,
		"model":             "<model-from-local-runner>",
		"status":            "completed",
		"duration_ms":       0,
		"input_tokens":      0,
		"generated_tokens":  0,
		"tokens_per_second": 0,
		"suite_id":          "<suite-id-from-claim>",
		"task_count":        0,
	})
	status := benchmarkRunnerStatusDisabled
	if runnerConfig.Enabled {
		switch runnerConfig.Mode {
		case config.BenchmarkRunnerModeSyntheticBuiltin:
			status = benchmarkRunnerStatusEnabledSynthetic
		default:
			status = benchmarkRunnerStatusEnabledDryRun
		}
	}
	return BenchmarkRunnerGuidance{
		Status:                status,
		RunnerImplementation:  "subscriber_local_runner_loop_available",
		ControlLevel:          "managed_nodes_only",
		TrustBoundary:         "subscriber_executes_prompts_and_fixtures_locally",
		SupportedSuiteIDs:     supportedSuiteIDs,
		ClaimEndpoint:         "/subscriber/benchmarks/claim",
		StatusEndpoint:        "/subscriber/benchmarks/status",
		ResultEndpoint:        "/subscriber/benchmarks",
		HeaderName:            "X-Llama-Wrangler-Subscriber-Token",
		CredentialPlaceholder: benchmarkRunnerCredentialPlaceholder,
		Environment: map[string]string{
			subscriberHeartbeatCredentialEnv: benchmarkRunnerCredentialPlaceholder,
			benchmarkRunnerEnabledEnv:        strconv.FormatBool(runnerConfig.Enabled),
			benchmarkRunnerFixtureDirEnv:     "./benchmark-fixtures",
		},
		ConfigSnippet: strings.Join([]string{
			"registration:",
			"  marshal_url: \"" + marshalURLPlaceholder + "\"",
			"  heartbeat_credential_env: " + subscriberHeartbeatCredentialEnv,
			"benchmark_runner:",
			"  enabled: " + strconv.FormatBool(runnerConfig.Enabled),
			"  mode: " + runnerConfig.Mode,
			"  poll_interval_seconds: " + strconv.Itoa(runnerConfig.PollIntervalSeconds),
			"  max_jobs_per_tick: " + strconv.Itoa(runnerConfig.MaxJobsPerTick),
			"  fixture_manifest_dir_env: " + benchmarkRunnerFixtureDirEnv,
			"  result_body_policy: " + config.BenchmarkRunnerResultPolicyMetricsOnly,
		}, "\n"),
		RunnerConfig: BenchmarkRunnerConfigSummary{
			Enabled:             runnerConfig.Enabled,
			Mode:                runnerConfig.Mode,
			PollIntervalSeconds: runnerConfig.PollIntervalSeconds,
			MaxJobsPerTick:      runnerConfig.MaxJobsPerTick,
			ResultBodyPolicy:    runnerConfig.ResultBodyPolicy,
		},
		ExecutionFlow: []string{
			"Poll claim endpoint with node ID and subscriber heartbeat credential.",
			"Read workload suite ID, task IDs, and optional local fixture manifest ID from the claimed job metadata.",
			"Dry-run mode derives deterministic metric summaries from suite/task metadata without loading prompts.",
			"Synthetic built-in mode resolves packaged task prompts locally on the subscriber and sends them only to the subscriber-local Ollama endpoint.",
			"Send status transitions and final metric summaries only.",
			"Discard prompt and response bodies before reporting to the marshal.",
		},
		MetricFields: []string{"benchmark_id", "model", "status", "duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "output_tokens_per_second", "prefill_tokens_per_second", "suite_id", "task_count", "fixture_manifest_id", "error_code"},
		Commands: BenchmarkRunnerCommands{
			ClaimJob:     benchmarkRunnerCurl(baseURL+"/subscriber/benchmarks/claim", claimBody),
			ReportStatus: benchmarkRunnerCurl(baseURL+"/subscriber/benchmarks/status", statusBody),
			ReportResult: benchmarkRunnerCurl(baseURL+"/subscriber/benchmarks", resultBody),
		},
		PackagingHooks: BenchmarkRunnerPackagingHooks{
			RunnerLoopStatus: "available_opt_in_synthetic_loop",
			ExpectedInputs: []string{
				"node_id",
				"marshal_url",
				"subscriber heartbeat credential from environment",
				"claimed benchmark job metadata",
				"subscriber-local synthetic suite definitions",
				"optional subscriber-local fixture manifest directory",
			},
			ExpectedOutputs: []string{
				"subscriber benchmark status updates",
				"subscriber benchmark result metric summaries",
				"local logs that exclude prompt and response bodies by default",
			},
			InstallNotes: []string{
				"Install and run future runner hooks on the subscriber host, beside Ollama and the Llama Wrangler subscriber service.",
				"The marshal does not remotely install packages, write fixture files, mutate service wrappers, or execute benchmark prompts.",
				"Keep built-in synthetic prompt text packaged with the subscriber runner only; expose suite and task IDs to the marshal.",
				"Dry-run mode verifies claim/status/result plumbing and reports deterministic metrics without loading prompt text.",
				"Synthetic built-in mode executes packaged task IDs against the subscriber-local Ollama endpoint and discards response bodies locally.",
			},
		},
		LocalFixtureGuidance: BenchmarkRunnerFixtureGuidance{
			SuiteID:             localFixtureWorkloadSuiteID,
			ManifestIDField:     "fixture_manifest_id",
			FixtureDirectoryEnv: benchmarkRunnerFixtureDirEnv,
			AllowedReferences:   []string{"manifest ID", "basename hint"},
			StoragePolicy:       "fixture_contents_and_full_paths_stay_on_subscriber",
		},
		Warnings: []string{
			"Real synthetic prompt execution is opt-in with mode synthetic_builtin_v1; dry_run_v1 remains the safe default.",
			"Prompt and response bodies must be discarded locally and never returned to the marshal.",
			"Local fixture contents and full fixture paths must stay on the subscriber host.",
			"Passive Endpoints cannot use subscriber benchmark runners.",
			"Do not paste subscriber credentials into support bundles, tickets, screenshots, or shared logs.",
		},
	}
}

func benchmarkRunnerSupportedSuiteIDs() []string {
	suites := benchmarkWorkloadSuites()
	out := make([]string, 0, len(suites))
	for _, suite := range suites {
		out = append(out, suite.ID)
	}
	return out
}

func benchmarkRunnerCurl(url string, body []byte) string {
	return strings.Join([]string{
		"curl -fsS -X POST " + shellSingleQuote(url),
		"-H " + shellSingleQuote("Content-Type: application/json"),
		"-H " + shellSingleQuote("X-Llama-Wrangler-Subscriber-Token: "+benchmarkRunnerCredentialPlaceholder),
		"-d " + shellSingleQuote(string(body)),
	}, " ")
}
