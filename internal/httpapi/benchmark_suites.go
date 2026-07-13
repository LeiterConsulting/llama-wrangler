package httpapi

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultBenchmarkWorkloadSuiteID = "synthetic_smoke_v1"
	localFixtureWorkloadSuiteID     = "operator_local_fixtures_v1"
)

type BenchmarkWorkloadTask struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	InputSource     string   `json:"input_source"`
	OutputPolicy    string   `json:"output_policy"`
	ExpectedMetrics []string `json:"expected_metrics"`
}

type BenchmarkWorkloadSuite struct {
	ID            string                  `json:"id"`
	DisplayName   string                  `json:"display_name"`
	Source        string                  `json:"source"`
	InputPolicy   string                  `json:"input_policy"`
	FixturePolicy string                  `json:"fixture_policy"`
	LocalOnly     bool                    `json:"local_only"`
	TaskCount     int                     `json:"task_count"`
	Tasks         []BenchmarkWorkloadTask `json:"tasks"`
	ResultMetrics []string                `json:"result_metrics"`
	Warnings      []string                `json:"warnings"`
}

type benchmarkJobRequest struct {
	SuiteID           string `json:"suite_id"`
	FixtureManifestID string `json:"fixture_manifest_id"`
	FixtureID         string `json:"fixture_id"`
	FixturePath       string `json:"fixture_path"`
}

func (s *Server) benchmarkWorkloadSuitesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, benchmarkWorkloadSuites())
}

func benchmarkWorkloadSuites() []BenchmarkWorkloadSuite {
	suites := []BenchmarkWorkloadSuite{
		{
			ID:            defaultBenchmarkWorkloadSuiteID,
			DisplayName:   "Synthetic Smoke",
			Source:        "builtin_synthetic",
			InputPolicy:   "subscriber_builtin_id_only",
			FixturePolicy: "none",
			LocalOnly:     true,
			Tasks: []BenchmarkWorkloadTask{
				{ID: "short_chat", Category: "chat", InputSource: "subscriber_builtin_synthetic", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second"}},
				{ID: "json_shape", Category: "structured_output", InputSource: "subscriber_builtin_synthetic", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "generated_tokens", "tokens_per_second"}},
				{ID: "tiny_summary", Category: "summarization", InputSource: "subscriber_builtin_synthetic", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second"}},
			},
			ResultMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "output_tokens_per_second", "prefill_tokens_per_second", "error_code"},
			Warnings:      []string{"Prompt and response bodies are owned by the subscriber runner and are not returned to the marshal."},
		},
		{
			ID:            "synthetic_code_v1",
			DisplayName:   "Synthetic Code",
			Source:        "builtin_synthetic",
			InputPolicy:   "subscriber_builtin_id_only",
			FixturePolicy: "none",
			LocalOnly:     true,
			Tasks: []BenchmarkWorkloadTask{
				{ID: "code_explain", Category: "code", InputSource: "subscriber_builtin_synthetic", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second"}},
				{ID: "code_transform", Category: "code", InputSource: "subscriber_builtin_synthetic", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "generated_tokens", "tokens_per_second"}},
			},
			ResultMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "output_tokens_per_second", "prefill_tokens_per_second", "error_code"},
			Warnings:      []string{"Synthetic task text is selected by suite/task ID on the subscriber; the marshal does not persist it."},
		},
		{
			ID:            localFixtureWorkloadSuiteID,
			DisplayName:   "Local Fixture Manifest",
			Source:        "operator_local_fixture",
			InputPolicy:   "subscriber_local_fixture_only",
			FixturePolicy: "fixture_manifest_id_required",
			LocalOnly:     true,
			Tasks: []BenchmarkWorkloadTask{
				{ID: "fixture_manifest", Category: "operator_fixture", InputSource: "subscriber_local_fixture_manifest", OutputPolicy: "discard_response_body", ExpectedMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "error_code"}},
			},
			ResultMetrics: []string{"duration_ms", "input_tokens", "generated_tokens", "tokens_per_second", "output_tokens_per_second", "prefill_tokens_per_second", "error_code"},
			Warnings:      []string{"Only a fixture manifest ID or basename hint may pass through the marshal; fixture contents stay local to the subscriber/operator host."},
		},
	}
	for i := range suites {
		suites[i].TaskCount = len(suites[i].Tasks)
	}
	return suites
}

func benchmarkWorkloadSuiteByID(id string) (BenchmarkWorkloadSuite, bool) {
	id = safeBenchmarkString(id)
	if id == "" {
		id = defaultBenchmarkWorkloadSuiteID
	}
	for _, suite := range benchmarkWorkloadSuites() {
		if suite.ID == id {
			return suite, true
		}
	}
	return BenchmarkWorkloadSuite{}, false
}

func benchmarkWorkloadSuiteStatus() map[string]interface{} {
	suites := benchmarkWorkloadSuites()
	sources := map[string]int{}
	for _, suite := range suites {
		sources[suite.Source]++
	}
	return map[string]interface{}{
		"default_suite_id": defaultBenchmarkWorkloadSuiteID,
		"suite_count":      len(suites),
		"sources":          sources,
		"input_policy":     "suite_ids_and_local_fixture_references_only",
		"content_storage":  "prompt_and_response_bodies_excluded",
	}
}

func benchmarkWorkloadSuiteJobMetadata(req benchmarkJobRequest) (map[string]interface{}, bool, string) {
	suite, ok := benchmarkWorkloadSuiteByID(req.SuiteID)
	if !ok {
		return nil, false, "unknown_benchmark_workload_suite"
	}
	metadata := map[string]interface{}{
		"suite_id":       suite.ID,
		"display_name":   suite.DisplayName,
		"source":         suite.Source,
		"input_policy":   suite.InputPolicy,
		"fixture_policy": suite.FixturePolicy,
		"local_only":     suite.LocalOnly,
		"task_count":     suite.TaskCount,
		"task_ids":       benchmarkWorkloadTaskIDs(suite),
		"result_metrics": suite.ResultMetrics,
	}
	if suite.ID == localFixtureWorkloadSuiteID {
		manifestID := safeFixtureReference(defaultString(req.FixtureManifestID, req.FixtureID))
		if manifestID == "" {
			manifestID = safeFixtureReference(filepath.Base(strings.TrimSpace(req.FixturePath)))
		}
		if manifestID == "" || manifestID == "." || manifestID == string(filepath.Separator) {
			return nil, false, "local_fixture_manifest_id_required"
		}
		metadata["fixture_manifest_id"] = manifestID
		if hint := safeFixtureReference(filepath.Base(strings.TrimSpace(req.FixturePath))); hint != "" && hint != "." {
			metadata["fixture_reference_hint"] = hint
		}
	}
	return metadata, true, ""
}

func benchmarkWorkloadTaskIDs(suite BenchmarkWorkloadSuite) []string {
	out := make([]string, 0, len(suite.Tasks))
	for _, task := range suite.Tasks {
		if task.ID != "" {
			out = append(out, task.ID)
		}
	}
	sort.Strings(out)
	return out
}

func safeFixtureReference(value string) string {
	value = safeBenchmarkString(value)
	value = strings.TrimSpace(value)
	if len(value) > 80 {
		value = value[:80]
	}
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-':
			return r
		default:
			return -1
		}
	}, value)
	return strings.Trim(value, ".-_")
}
