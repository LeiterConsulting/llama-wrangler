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

func TestSubscriberBenchmarkResultIngestionStoresMetadataOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_benchmark_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-bench",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-bench"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	body := `{"node_id":"managed-bench","benchmark_id":"bench-1","model":"llama3.1:8b","status":"completed","duration_ms":1200,"input_tokens":128,"generated_tokens":256,"tokens_per_second":42.5,"output_tokens_per_second":41.8,"prefill_tokens_per_second":300.1,"prompt":"SECRET_PROMPT","response":"do not store me"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks", bytes.NewBufferString(body))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkResult(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("benchmark result status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "do not store me") {
		t.Fatalf("benchmark response leaked forbidden marker: %s", rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["managed-bench"]
	if node.BenchmarkSource != appstate.BenchmarkSourceSubscriberReported {
		t.Fatalf("benchmark source = %q", node.BenchmarkSource)
	}
	if node.Observed["benchmark_status"] != "completed" {
		t.Fatalf("benchmark status metadata = %#v", node.Observed)
	}
	last, ok := node.Observed["benchmark_last_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("last result missing: %#v", node.Observed["benchmark_last_result"])
	}
	if last["model"] != "llama3.1:8b" || last["tokens_per_second"] != 42.5 {
		t.Fatalf("last result = %#v", last)
	}
	rendered := string(mustJSON(t, node.Observed))
	for _, forbidden := range []string{"SECRET_PROMPT", "do not store me", credential, "Authorization"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("stored benchmark metadata leaked %q: %s", forbidden, rendered)
		}
	}
	if len(node.Models) != 1 || node.Models[0].TokensSec != 42.5 {
		t.Fatalf("model benchmark rate not updated: %#v", node.Models)
	}

	for _, route := range []string{"/wrangler/ui/bootstrap", "/wrangler/support-bundle/export"} {
		rr = httptest.NewRecorder()
		method := http.MethodGet
		if strings.Contains(route, "support-bundle") {
			method = http.MethodPost
		}
		req = httptest.NewRequest(method, route, nil)
		if method == http.MethodPost {
			server.exportSupportBundle(rr, req)
		} else {
			server.bootstrap(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", route, rr.Code, rr.Body.String())
		}
		for _, forbidden := range []string{"SECRET_PROMPT", "do not store me", credential, "Authorization"} {
			if strings.Contains(rr.Body.String(), forbidden) {
				t.Fatalf("%s leaked %q: %s", route, forbidden, rr.Body.String())
			}
		}
	}
}

func TestSubscriberBenchmarkResultRequiresStoredCredential(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_required_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-secure-bench",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-secure-bench"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks", bytes.NewBufferString(`{"node_id":"managed-secure-bench","status":"completed"}`))
	server.subscriberBenchmarkResult(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing credential benchmark status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) {
		t.Fatalf("auth failure leaked credential: %s", rr.Body.String())
	}
}

func TestManagedBenchmarkJobOrchestrationClaimStatusAndResult(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_job_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-job",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-job"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/managed-job/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("benchmark queue status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("benchmark queue response leaked forbidden marker: %s", rr.Body.String())
	}
	var queueBody struct {
		Status string                 `json:"status"`
		Job    map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &queueBody); err != nil {
		t.Fatalf("decode queue response: %v", err)
	}
	benchmarkID, _ := queueBody.Job["benchmark_id"].(string)
	if queueBody.Status != "queued" || benchmarkID == "" || queueBody.Job["status"] != "queued" {
		t.Fatalf("queued job response = %#v", queueBody)
	}
	if queueBody.Job["type"] != "managed_node_metadata_benchmark" || queueBody.Job["result_endpoint"] != "/subscriber/benchmarks" {
		t.Fatalf("queued job contract = %#v", queueBody.Job)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-job"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("claim job status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("claim response leaked forbidden marker: %s", rr.Body.String())
	}
	var claimBody struct {
		Status string                 `json:"status"`
		Job    map[string]interface{} `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &claimBody); err != nil {
		t.Fatalf("decode claim response: %v", err)
	}
	if claimBody.Status != "job_claimed" || claimBody.Job["benchmark_id"] != benchmarkID || claimBody.Job["status"] != "running" {
		t.Fatalf("claim response = %#v", claimBody)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/status", bytes.NewBufferString(`{"node_id":"managed-job","benchmark_id":"`+benchmarkID+`","status":"running","error_code":"SECRET_PROMPT"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobStatus(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status update status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) || strings.Contains(rr.Body.String(), "SECRET_PROMPT") {
		t.Fatalf("status response leaked forbidden marker: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	resultBody := `{"node_id":"managed-job","benchmark_id":"` + benchmarkID + `","model":"llama3.1:8b","status":"completed","duration_ms":900,"generated_tokens":90,"tokens_per_second":100,"prompt":"SECRET_PROMPT"}`
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks", bytes.NewBufferString(resultBody))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkResult(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("result status = %d body = %s", rr.Code, rr.Body.String())
	}
	node := server.store.Snapshot().Nodes["managed-job"]
	jobs := benchmarkJobs(node.Observed["benchmark_jobs"])
	if len(jobs) == 0 || jobs[0]["benchmark_id"] != benchmarkID || jobs[0]["status"] != "completed" {
		t.Fatalf("completed job metadata = %#v", node.Observed["benchmark_jobs"])
	}
	rendered := string(mustJSON(t, node.Observed))
	for _, forbidden := range []string{credential, "SECRET_PROMPT", "Authorization"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("job metadata leaked %q: %s", forbidden, rendered)
		}
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-job"}`))
	req.Header.Set("X-Llama-Wrangler-Subscriber-Token", credential)
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"no_job"`) {
		t.Fatalf("empty claim status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestBenchmarkJobClaimRequiresCredentialAndRejectsPassive(t *testing.T) {
	server := newIsolatedTestServer(t)
	credential := "lw_hb_claim_SECRET"
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "managed-claim",
		ControlLevel:  appstate.ControlLevelManaged,
		TrustLevel:    appstate.TrustLevelLocal,
		ApprovalState: appstate.ApprovalStateApproved,
		Approved:      true,
		Enabled:       true,
		Status:        "healthy",
		Observed:      map[string]interface{}{"heartbeat_auth_method": "shared_secret", "heartbeat_auth_required": true, "benchmark_jobs": []map[string]interface{}{{"benchmark_id": "bench-auth", "status": "queued"}}},
	}); err != nil {
		t.Fatalf("upsert managed node: %v", err)
	}
	if err := server.secrets.Set(subscriberHeartbeatSecretKey("managed-claim"), credential); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:        "passive-claim",
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
	req := httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"managed-claim"}`))
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing credential claim status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), credential) {
		t.Fatalf("claim auth failure leaked credential: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/subscriber/benchmarks/claim", bytes.NewBufferString(`{"node_id":"passive-claim"}`))
	server.subscriberBenchmarkJobClaim(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "managed nodes") {
		t.Fatalf("passive claim status = %d body = %s", rr.Code, rr.Body.String())
	}
}

func TestPassiveBenchmarkProbeIsMarshalObservedOnly(t *testing.T) {
	server := newIsolatedTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" || r.ContentLength > 0 {
			t.Fatalf("passive probe sent auth or body")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
	}))
	defer upstream.Close()
	if err := server.store.UpsertNode(appstate.Node{
		NodeID:          "passive-probe",
		ControlLevel:    appstate.ControlLevelPassive,
		TrustLevel:      appstate.TrustLevelLANTrusted,
		ApprovalState:   appstate.ApprovalStateApproved,
		Approved:        true,
		Enabled:         true,
		Status:          "healthy",
		URL:             upstream.URL,
		BenchmarkSource: appstate.BenchmarkSourceNone,
	}); err != nil {
		t.Fatalf("upsert passive node: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-probe/benchmark-probe", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("passive probe status = %d body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "SECRET_PROMPT") || strings.Contains(rr.Body.String(), "Authorization") {
		t.Fatalf("passive probe leaked forbidden marker: %s", rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode passive probe response: %v", err)
	}
	result := body["result"].(map[string]interface{})
	if result["source"] != appstate.BenchmarkSourceMarshalObserved || result["mode"] != "marshal_observed_api_tags" || result["status"] != "probe_ok" {
		t.Fatalf("probe result = %#v", result)
	}
	node := server.store.Snapshot().Nodes["passive-probe"]
	if node.BenchmarkSource != appstate.BenchmarkSourceMarshalObserved {
		t.Fatalf("passive benchmark source = %q", node.BenchmarkSource)
	}
	if node.ManagementSupported || node.WarmStateSupported {
		t.Fatalf("passive probe enabled local control: %#v", node)
	}
	if node.Observed["benchmark_status"] != "probe_ok" || node.Observed["benchmark_source"] != appstate.BenchmarkSourceMarshalObserved {
		t.Fatalf("passive probe metadata = %#v", node.Observed)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/wrangler/nodes/passive-probe/benchmark", nil)
	server.nodeAction(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "passive_no_local_benchmark_control") {
		t.Fatalf("passive local benchmark rejection changed: status=%d body=%s", rr.Code, rr.Body.String())
	}
}
