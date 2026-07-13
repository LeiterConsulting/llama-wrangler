package hec

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var consensusFailureReasons = []string{
	"missing_proxy_url",
	"connection_error",
	"upstream_4xx",
	"upstream_5xx",
	"body_read_failure",
	"response_size_limit",
	"timeout",
	"cancellation",
}

func TestSplunkOperationsAssetsAreValidAndMetadataOnly(t *testing.T) {
	root := filepath.Join("..", "..", "splunk_app")
	operations := readSplunkAsset(t, root, "default", "data", "ui", "views", "llama_wrangler_operations.xml")
	overview := readSplunkAsset(t, root, "default", "data", "ui", "views", "llama_wrangler_overview.xml")
	nav := readSplunkAsset(t, root, "default", "data", "ui", "nav", "default.xml")
	for name, raw := range map[string][]byte{
		"operations dashboard": operations,
		"overview dashboard":   overview,
		"navigation":           nav,
	} {
		decoder := xml.NewDecoder(strings.NewReader(string(raw)))
		for {
			if _, err := decoder.Token(); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("parse %s XML: %v", name, err)
			}
		}
	}

	operationsText := string(operations)
	for _, required := range []string{
		"Consensus Participant Failure Reasons",
		"Queue Scheduling Outcomes",
		"Streaming Retry, Partial, and Cancellation Events",
		"Benchmark Scheduler History",
		"Subscriber Benchmark Runner History",
		"Routing Policy Exclusion Reasons",
		"Model Lifecycle Action History",
		"`llama_wrangler_consensus`",
		"`llama_wrangler_queue`",
		"`llama_wrangler_streaming_outcomes`",
		"`llama_wrangler_benchmark_scheduler`",
		"`llama_wrangler_benchmark_runner`",
		"`llama_wrangler_routing`",
		"`llama_wrangler_model_lifecycle_actions`",
	} {
		if !strings.Contains(operationsText, required) {
			t.Fatalf("operations dashboard missing %q", required)
		}
	}
	for _, reason := range consensusFailureReasons {
		if !strings.Contains(operationsText, "failure_reason_counts."+reason) {
			t.Fatalf("operations dashboard missing consensus reason %q", reason)
		}
	}
	if strings.Count(operationsText, "<query>") < 12 {
		t.Fatalf("operations dashboard has too few searches")
	}
	if !strings.Contains(string(overview), "Consensus Participant Failures") || !strings.Contains(string(overview), "Peak Queue Depth") || !strings.Contains(string(overview), "Streaming Outcome Events") {
		t.Fatalf("overview dashboard missing compact operations signals")
	}
	if !strings.Contains(string(nav), `name="llama_wrangler_overview"`) || !strings.Contains(string(nav), `name="llama_wrangler_operations"`) {
		t.Fatalf("navigation does not expose both dashboards")
	}

	knowledge := append([]byte(nil), operations...)
	for _, path := range [][]string{
		{"default", "data", "ui", "views", "llama_wrangler_overview.xml"},
		{"default", "macros.conf"},
		{"default", "eventtypes.conf"},
		{"default", "savedsearches.conf"},
		{"default", "props.conf"},
	} {
		knowledge = append(knowledge, readSplunkAsset(t, root, path...)...)
	}
	lower := strings.ToLower(string(knowledge))
	for _, forbidden := range []string{
		"prompt_body",
		"response_body",
		"request_body",
		"raw_header",
		"authorization",
		"api_key",
		"client_api_key",
		"hec_token",
		"enrollment_token",
		"heartbeat_credential",
		"provider_key",
		"fixture_content",
		"fixture_path",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("Splunk knowledge objects reference forbidden field %q", forbidden)
		}
	}
	rawPayloadField := regexp.MustCompile(`(?m)(^|[\s,|])payload($|[\s,|])`)
	if rawPayloadField.MatchString(lower) {
		t.Fatalf("Splunk knowledge objects reference forbidden raw payload field")
	}
}

func TestSplunkKnowledgeObjectsCoverOperationalEventTypes(t *testing.T) {
	root := filepath.Join("..", "..", "splunk_app")
	macros := string(readSplunkAsset(t, root, "default", "macros.conf"))
	eventtypes := string(readSplunkAsset(t, root, "default", "eventtypes.conf"))
	props := string(readSplunkAsset(t, root, "default", "props.conf"))
	saved := string(readSplunkAsset(t, root, "default", "savedsearches.conf"))
	tags := string(readSplunkAsset(t, root, "default", "tags.conf"))

	for _, macro := range []string{
		"llama_wrangler_consensus",
		"llama_wrangler_queue",
		"llama_wrangler_streaming_outcomes",
		"llama_wrangler_benchmark_scheduler",
		"llama_wrangler_benchmark_runner",
		"llama_wrangler_benchmark_operations",
		"llama_wrangler_routing",
		"llama_wrangler_model_lifecycle_actions",
	} {
		if !strings.Contains(macros, "["+macro+"]") {
			t.Fatalf("macros.conf missing %s", macro)
		}
	}
	for _, eventtype := range []string{
		"llama_wrangler_consensus",
		"llama_wrangler_queue",
		"llama_wrangler_streaming_outcomes",
		"llama_wrangler_benchmark_operations",
		"llama_wrangler_routing_policy",
		"llama_wrangler_model_lifecycle_actions",
	} {
		if !strings.Contains(eventtypes, "["+eventtype+"]") {
			t.Fatalf("eventtypes.conf missing %s", eventtype)
		}
	}
	for _, sourcetype := range []string{
		"queue_state",
		"upstream_retry",
		"response_partial",
		"request_cancelled",
		"benchmark_job_claimed",
		"benchmark_job_status",
		"benchmark_scheduler_reconcile",
		"benchmark_scheduler_manual_reconcile",
		"benchmark_scheduler_background_tick",
		"benchmark_scheduler_policy_updated",
		"subscriber_benchmark_runner_tick",
		"model_lifecycle_action_queued",
		"model_lifecycle_action_claimed",
		"model_lifecycle_action_status",
		"model_lifecycle_action_rejected",
	} {
		if !strings.Contains(props, "[llama_wrangler:"+sourcetype+"]") {
			t.Fatalf("props.conf missing sourcetype %s", sourcetype)
		}
	}
	for _, report := range []string{
		"Consensus Participant Failure Summary",
		"Queue Scheduling Summary",
		"Streaming Outcome Summary",
		"Benchmark Scheduler and Runner History",
		"Routing Policy Exclusion Summary",
		"Model Lifecycle Action History",
	} {
		if !strings.Contains(saved, "[Llama Wrangler - "+report+"]") {
			t.Fatalf("savedsearches.conf missing report %q", report)
		}
	}
	if strings.Count(saved, "disabled = 1") < 9 || strings.Contains(saved, "disabled = 0") {
		t.Fatalf("packaged saved searches must remain disabled by default")
	}
	for _, eventtype := range []string{"llama_wrangler_consensus", "llama_wrangler_queue", "llama_wrangler_streaming_outcomes", "llama_wrangler_benchmark_operations", "llama_wrangler_routing_policy", "llama_wrangler_model_lifecycle_actions"} {
		if !strings.Contains(tags, "[eventtype="+eventtype+"]") {
			t.Fatalf("tags.conf missing operational eventtype %s", eventtype)
		}
	}
}

func TestSplunkConsensusFailureVocabularyMatchesHECSchema(t *testing.T) {
	root := filepath.Join("..", "..")
	operations := string(readSplunkAsset(t, root, "splunk_app", "default", "data", "ui", "views", "llama_wrangler_operations.xml"))
	saved := string(readSplunkAsset(t, root, "splunk_app", "default", "savedsearches.conf"))
	reasonPattern := regexp.MustCompile(`failure_reason_counts\.([a-z0-9_]+)`)
	actualSet := map[string]struct{}{}
	for _, match := range reasonPattern.FindAllStringSubmatch(operations+saved, -1) {
		actualSet[match[1]] = struct{}{}
	}
	actual := sortedKeys(actualSet)
	want := append([]string(nil), consensusFailureReasons...)
	sort.Strings(want)
	if strings.Join(actual, ",") != strings.Join(want, ",") {
		t.Fatalf("Splunk consensus reasons = %v, want %v", actual, want)
	}

	schemaRaw := readSplunkAsset(t, root, "schemas", "hec_events.schema.json")
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("decode HEC schema: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	failures := properties["participant_failures"].(map[string]interface{})
	items := failures["items"].(map[string]interface{})
	itemProperties := items["properties"].(map[string]interface{})
	reasonCode := itemProperties["reason_code"].(map[string]interface{})
	enumValues := reasonCode["enum"].([]interface{})
	schemaReasons := make([]string, 0, len(enumValues))
	for _, value := range enumValues {
		schemaReasons = append(schemaReasons, value.(string))
	}
	sort.Strings(schemaReasons)
	if strings.Join(schemaReasons, ",") != strings.Join(want, ",") {
		t.Fatalf("HEC schema consensus reasons = %v, want %v", schemaReasons, want)
	}
}

func readSplunkAsset(t *testing.T, root string, parts ...string) []byte {
	t.Helper()
	path := filepath.Join(append([]string{root}, parts...)...)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
