package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/config"
)

func TestBenchmarkRunnerGuidanceIsPlaceholderOnly(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/benchmarks/runner/guidance", nil)
	server.benchmarkRunnerGuidanceHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("runner guidance status = %d body = %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, expected := range []string{
		benchmarkRunnerStatusDisabled,
		"subscriber_local_runner_loop_available",
		defaultBenchmarkWorkloadSuiteID,
		"synthetic_code_v1",
		localFixtureWorkloadSuiteID,
		"available_opt_in_synthetic_loop",
		"subscriber-heartbeat-credential",
		"fixture_contents_and_full_paths_stay_on_subscriber",
		config.BenchmarkRunnerModeSyntheticBuiltin,
		"/subscriber/benchmarks/claim",
		"/subscriber/benchmarks/status",
		"/subscriber/benchmarks",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("runner guidance missing %q: %s", expected, body)
		}
	}
	assertNoBenchmarkRunnerGuidanceLeak(t, body)
}

func TestBootstrapIncludesBenchmarkRunnerGuidanceWithoutPayloads(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "benchmark_runner") ||
		!strings.Contains(body, benchmarkRunnerStatusDisabled) ||
		!strings.Contains(body, "result_body_policy: metrics_only") {
		t.Fatalf("bootstrap missing benchmark runner guidance: %s", body)
	}
	assertNoBenchmarkRunnerGuidanceLeak(t, body)
}

func assertNoBenchmarkRunnerGuidanceLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"SECRET_PROMPT",
		"SECRET_RESPONSE",
		"Bearer SECRET",
		"lw_hb_",
		"lw_enroll_",
		"lw_admin_",
		"lw_client_",
		"sk-",
		"/tmp/Secret Project",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("benchmark runner guidance leaked %q: %s", forbidden, body)
		}
	}
}
