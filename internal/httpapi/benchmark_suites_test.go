package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llama-wrangler/internal/appstate"
)

func TestBenchmarkWorkloadSuitesExposeDefinitionsWithoutPayloads(t *testing.T) {
	server := newIsolatedTestServer(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrangler/benchmarks/workload-suites", nil)
	server.benchmarkWorkloadSuitesHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("workload suites status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), defaultBenchmarkWorkloadSuiteID) || !strings.Contains(rr.Body.String(), localFixtureWorkloadSuiteID) {
		t.Fatalf("workload suite definitions missing expected suites: %s", rr.Body.String())
	}
	for _, forbidden := range []string{"SECRET_PROMPT", "SECRET_RESPONSE", "Authorization", "lw_hb_"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("workload suites leaked %q: %s", forbidden, rr.Body.String())
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/wrangler/ui/bootstrap", nil)
	server.bootstrap(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "benchmark_workload_suites") || strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("bootstrap workload suites missing or leaked payload: %s", rr.Body.String())
	}
}

func TestManagedBenchmarkJobCarriesSyntheticSuiteMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-suite",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	body := `{"suite_id":"synthetic_code_v1","prompt":"SECRET_PROMPT","response":"SECRET_RESPONSE","headers":{"Authorization":"Bearer SECRET"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-suite/benchmark", bytes.NewBufferString(body))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("benchmark queue status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoBenchmarkSuiteLeak(t, rr.Body.String())
	var queued struct {
		Status string                 `json:"status"`
		Job    map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatalf("decode queued job: %v", err)
	}
	suite, ok := queued.Job["workload_suite"].(map[string]interface{})
	if !ok {
		t.Fatalf("queued job missing workload suite: %#v", queued.Job)
	}
	if suite["suite_id"] != "synthetic_code_v1" || suite["source"] != "builtin_synthetic" || suite["input_policy"] != "subscriber_builtin_id_only" {
		t.Fatalf("unexpected suite metadata: %#v", suite)
	}
	if _, ok := suite["task_ids"].([]interface{}); !ok {
		t.Fatalf("suite task ids missing: %#v", suite)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-suite"}`))
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("claim status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoBenchmarkSuiteLeak(t, rr.Body.String())
	if !strings.Contains(rr.Body.String(), "synthetic_code_v1") || !strings.Contains(rr.Body.String(), "subscriber_builtin_id_only") {
		t.Fatalf("claim missing workload suite metadata: %s", rr.Body.String())
	}

	node := server.store.Snapshot().Nodes["managed-suite"]
	rendered := string(mustJSON(t, node.Observed))
	assertNoBenchmarkSuiteLeak(t, rendered)
	if !strings.Contains(rendered, "synthetic_code_v1") {
		t.Fatalf("stored suite metadata missing: %s", rendered)
	}
}

func TestLocalFixtureBenchmarkSuiteRequiresReferenceAndStoresNoFixtureContents(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-fixture",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-fixture/benchmark", bytes.NewBufferString(`{"suite_id":"operator_local_fixtures_v1"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "local_fixture_manifest_id_required") {
		t.Fatalf("missing fixture reference status = %d body = %s", rr.Code, rr.Body.String())
	}

	body := `{"suite_id":"operator_local_fixtures_v1","fixture_manifest_id":"local-fixture-001","fixture_path":"/tmp/Secret Project/SECRET_PROMPT.json","payload":"SECRET_RESPONSE"}`
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-fixture/benchmark", bytes.NewBufferString(body))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("fixture benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	assertNoBenchmarkSuiteLeak(t, rr.Body.String())
	if !strings.Contains(rr.Body.String(), "local-fixture-001") || strings.Contains(rr.Body.String(), "/tmp/Secret Project") {
		t.Fatalf("fixture metadata missing or leaked path: %s", rr.Body.String())
	}

	for _, route := range []string{"/wrangler/ui/bootstrap", "/wrangler/support-bundle/export"} {
		rr = httptest.NewRecorder()
		if strings.Contains(route, "support-bundle") {
			req = httptest.NewRequest(http.MethodPost, route, nil)
			server.exportSupportBundle(rr, req)
		} else {
			req = httptest.NewRequest(http.MethodGet, route, nil)
			server.bootstrap(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		assertNoBenchmarkSuiteLeak(t, rr.Body.String())
		if strings.Contains(rr.Body.String(), "/tmp/Secret Project") {
			t.Fatalf("%s leaked fixture path: %s", route, rr.Body.String())
		}
	}
}

func TestBenchmarkWorkloadRejectsUnknownSuiteAndPassiveLocalControl(t *testing.T) {
	server := newIsolatedTestServer(t)
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-unknown-suite",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-suite",
		ControlLevel:  appstate.ControlLevelPassive,
		TrustLevel:    appstate.TrustLevelLANTrusted,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-unknown-suite/benchmark", bytes.NewBufferString(`{"suite_id":"github_ci_future_v2"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "unknown_benchmark_workload_suite") {
		t.Fatalf("unknown suite status = %d body = %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-suite/benchmark", bytes.NewBufferString(`{"suite_id":"synthetic_smoke_v1"}`))
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "passive_no_local_benchmark_control") {
		t.Fatalf("passive benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func assertNoBenchmarkSuiteLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"SECRET_PROMPT", "SECRET_RESPONSE", "Authorization", "Bearer SECRET", "lw_hb_"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("benchmark workload metadata leaked %q: %s", forbidden, body)
		}
	}
}
