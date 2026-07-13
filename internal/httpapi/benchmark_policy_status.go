package httpapi

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/routing"
)

type BenchmarkPolicyStatus struct {
	Window  string                `json:"window"`
	Summary map[string]int        `json:"summary"`
	Nodes   []BenchmarkPolicyNode `json:"nodes"`
}

type BenchmarkPolicyNode struct {
	NodeID               string   `json:"node_id"`
	ControlLevel         string   `json:"control_level"`
	TrustLevel           string   `json:"trust_level"`
	ApprovalState        string   `json:"approval_state"`
	BenchmarkSource      string   `json:"benchmark_source"`
	Eligible             bool     `json:"eligible"`
	Mode                 string   `json:"mode"`
	ReasonCodes          []string `json:"reason_codes"`
	Message              string   `json:"message"`
	PlacementEligible    bool     `json:"placement_eligible"`
	PlacementWindow      string   `json:"placement_window"`
	PlacementReasonCodes []string `json:"placement_reason_codes"`
	PlacementMessage     string   `json:"placement_message"`
}

func (s *Server) benchmarkPolicyStatus() BenchmarkPolicyStatus {
	return summarizeBenchmarkPolicy(s.store.Snapshot().Nodes)
}

func summarizeBenchmarkPolicy(nodes map[string]appstate.Node) BenchmarkPolicyStatus {
	now := time.Now().UTC()
	status := BenchmarkPolicyStatus{
		Window:  "current_node_metadata",
		Summary: map[string]int{},
		Nodes:   []BenchmarkPolicyNode{},
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		item := benchmarkPolicyForNode(nodes[id], now)
		status.Nodes = append(status.Nodes, item)
		status.Summary["nodes"]++
		if item.Eligible {
			status.Summary["eligible"]++
		} else {
			status.Summary["ineligible"]++
		}
		if item.PlacementEligible {
			status.Summary["placement_eligible"]++
		} else {
			status.Summary["placement_limited"]++
		}
		status.Summary[item.Mode]++
		for _, code := range item.ReasonCodes {
			status.Summary[code]++
		}
		for _, code := range item.PlacementReasonCodes {
			status.Summary[code]++
		}
	}
	return status
}

func benchmarkPolicyForNode(node appstate.Node, now time.Time) BenchmarkPolicyNode {
	item := BenchmarkPolicyNode{
		NodeID:          node.NodeID,
		ControlLevel:    routingStatusControlLevel(node),
		TrustLevel:      routingStatusTrustLevel(node),
		ApprovalState:   routingStatusApprovalState(node),
		BenchmarkSource: benchmarkSource(node),
		PlacementWindow: routing.BenchmarkPlacementFreshnessWindow.String(),
	}
	item.PlacementEligible, item.PlacementReasonCodes, item.PlacementMessage = benchmarkPlacementPolicyForNode(node, now)
	if item.ControlLevel == appstate.ControlLevelPassive {
		item.Eligible = false
		item.Mode = "marshal_observed_probe_only"
		item.ReasonCodes = []string{"passive_no_local_benchmark_control"}
		item.Message = "Passive Endpoint benchmarks are limited to future marshal-observed probes; local benchmark control is unavailable."
		return item
	}
	item.Mode = "subscriber_reported"
	if !node.Enabled {
		item.ReasonCodes = append(item.ReasonCodes, "node_disabled")
	}
	if !routingStatusApproved(node) {
		item.ReasonCodes = append(item.ReasonCodes, "node_not_approved")
	}
	if node.Status == "disabled" || node.Status == "failed" {
		item.ReasonCodes = append(item.ReasonCodes, "node_status_"+node.Status)
	}
	if len(item.ReasonCodes) > 0 {
		item.Eligible = false
		item.Message = "Managed Node must be approved, enabled, and healthy before queuing subscriber-reported benchmarks."
		return item
	}
	item.Eligible = true
	item.ReasonCodes = []string{"managed_subscriber_benchmark_allowed"}
	item.Message = "Managed Node can queue subscriber-reported benchmark work."
	return item
}

func benchmarkPlacementPolicyForNode(node appstate.Node, now time.Time) (bool, []string, string) {
	if routingStatusControlLevel(node) != appstate.ControlLevelManaged {
		return false, []string{"benchmark_placement_passive_probe_ignored"}, "Passive Endpoint probe metadata is not used for local-control benchmark placement."
	}
	switch routingStatusTrustLevel(node) {
	case appstate.TrustLevelLocal, appstate.TrustLevelLANTrusted:
	default:
		return false, []string{"benchmark_placement_untrusted_ignored"}, "Benchmark placement waits for local or LAN trusted node trust."
	}
	if benchmarkSource(node) != appstate.BenchmarkSourceSubscriberReported {
		return false, []string{"benchmark_placement_source_ignored"}, "Benchmark placement requires subscriber-reported Managed Node summaries."
	}
	result, ok := latestBenchmarkPolicyResult(node)
	if !ok {
		return false, []string{"benchmark_placement_summary_missing"}, "No subscriber-reported benchmark summary is available for placement yet."
	}
	if benchmarkPolicyString(result["source"]) != appstate.BenchmarkSourceSubscriberReported {
		return false, []string{"benchmark_placement_source_ignored"}, "Only subscriber-reported summaries can drive Managed Node placement."
	}
	status := benchmarkPolicyString(result["status"])
	if status != "" && status != "completed" && status != "reported" {
		return false, []string{"benchmark_placement_status_ignored"}, "Only completed or reported benchmark summaries can drive placement."
	}
	completedAt, ok := benchmarkPolicyTime(result["completed_at"])
	if !ok {
		completedAt, ok = benchmarkPolicyTime(node.Observed["benchmark_updated_at"])
	}
	if !ok || completedAt.IsZero() {
		return false, []string{"benchmark_placement_freshness_missing"}, "Benchmark summary is missing freshness metadata."
	}
	if now.Sub(completedAt.UTC()) > routing.BenchmarkPlacementFreshnessWindow {
		return false, []string{"benchmark_placement_summary_stale"}, "Benchmark summary is stale and ignored for placement."
	}
	rate := benchmarkPolicyFloat(result["tokens_per_second"])
	if rate <= 0 {
		rate = benchmarkPolicyFloat(result["output_tokens_per_second"])
	}
	if rate <= 0 {
		return false, []string{"benchmark_placement_rate_missing"}, "Benchmark summary is missing token-rate metadata."
	}
	return true, []string{"benchmark_placement_applied", "benchmark_placement_fresh"}, "Fresh trusted subscriber-reported benchmark summary can influence routing placement."
}

func latestBenchmarkPolicyResult(node appstate.Node) (map[string]interface{}, bool) {
	if node.Observed == nil {
		return nil, false
	}
	results := []map[string]interface{}{}
	switch values := node.Observed["benchmark_results"].(type) {
	case []map[string]interface{}:
		results = append(results, values...)
	case []interface{}:
		for _, value := range values {
			if item, ok := value.(map[string]interface{}); ok {
				results = append(results, item)
			}
		}
	}
	if len(results) == 0 {
		if result, ok := node.Observed["benchmark_last_result"].(map[string]interface{}); ok {
			results = append(results, result)
		}
	}
	var best map[string]interface{}
	var bestTime time.Time
	for _, result := range results {
		completedAt, _ := benchmarkPolicyTime(result["completed_at"])
		if best == nil || completedAt.After(bestTime) {
			best = result
			bestTime = completedAt
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

func benchmarkPolicyString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func benchmarkPolicyFloat(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		float, _ := typed.Float64()
		return float
	default:
		return 0
	}
}

func benchmarkPolicyTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}

func benchmarkSource(node appstate.Node) string {
	if node.BenchmarkSource != "" {
		return node.BenchmarkSource
	}
	if routingStatusControlLevel(node) == appstate.ControlLevelPassive {
		return appstate.BenchmarkSourceNone
	}
	return appstate.BenchmarkSourceSubscriberReported
}
