package routing

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

const (
	ManagedHeartbeatFreshnessWindow   = 2 * time.Minute
	BenchmarkPlacementFreshnessWindow = 24 * time.Hour
	ConsensusDefaultMinParticipants   = 2
	ConsensusDefaultMaxParticipants   = 4
	ConsensusMaximumParticipants      = 8
)

type Request struct {
	Model         string
	ExecutionMode string
	SessionID     string
	Streaming     bool
	TaskType      string
}

type Decision struct {
	ModelAlias        string         `json:"model_alias"`
	ResolvedModel     string         `json:"resolved_model"`
	SelectedNode      string         `json:"selected_node"`
	CandidateNodes    []string       `json:"candidate_nodes"`
	CandidateMetadata []NodeMetadata `json:"candidate_metadata,omitempty"`
	FallbackNodes     []string       `json:"fallback_nodes"`
	ExcludedNodes     []NodeMetadata `json:"excluded_nodes,omitempty"`
	Strategy          string         `json:"routing_strategy"`
	Reasons           []string       `json:"routing_reasons"`
	ExecutionMode     string         `json:"execution_mode"`
	Affinity          string         `json:"affinity"`
	ConsensusRequired int            `json:"consensus_required,omitempty"`
	ConsensusLimit    int            `json:"consensus_limit,omitempty"`
}

type NodeMetadata struct {
	NodeID           string   `json:"node_id"`
	ControlLevel     string   `json:"control_level"`
	TrustLevel       string   `json:"trust_level"`
	CapabilitySource string   `json:"capability_source,omitempty"`
	Reasons          []string `json:"reasons,omitempty"`
}

func Select(cfg config.Config, state appstate.State, req Request) (Decision, bool) {
	now := time.Now().UTC()
	model := req.Model
	if model == "" {
		model = cfg.Routing.DefaultModelAlias
	}
	alias, isAlias := cfg.ModelAliases[model]
	minParticipants := 0
	maxParticipants := 0
	decision := Decision{
		ModelAlias:    model,
		Strategy:      "weighted_best_available",
		ExecutionMode: req.ExecutionMode,
		Affinity:      cfg.Session.DefaultAffinity,
	}
	candidates := []string{model}
	if isAlias {
		candidates = alias.Candidates
		decision.Strategy = alias.Strategy
		if alias.ExecutionMode != "" {
			decision.ExecutionMode = alias.ExecutionMode
		}
		if alias.Affinity != "" {
			decision.Affinity = alias.Affinity
		}
		minParticipants = alias.MinParticipants
		maxParticipants = alias.MaxParticipants
	}
	if decision.ExecutionMode == "" {
		decision.ExecutionMode = cfg.Routing.DefaultExecutionMode
	}
	consensusMode := isConsensusMode(decision.ExecutionMode, decision.Strategy)
	if consensusMode {
		minParticipants, maxParticipants = consensusParticipantBounds(minParticipants, maxParticipants)
		decision.ConsensusRequired = minParticipants
		decision.ConsensusLimit = maxParticipants
	}

	nodes := make([]appstate.Node, 0, len(state.Nodes)+len(cfg.Subscribers))
	seen := map[string]bool{}
	for _, n := range state.Nodes {
		nodes = append(nodes, n)
		seen[n.NodeID] = true
	}
	for _, sub := range cfg.Subscribers {
		if !seen[sub.NodeID] {
			nodes = append(nodes, appstate.Node{
				NodeID:        sub.NodeID,
				URL:           sub.URL,
				Enabled:       true,
				Approved:      true,
				ApprovalState: appstate.ApprovalStateApproved,
				ControlLevel:  appstate.ControlLevelManaged,
				TrustLevel:    appstate.TrustLevelLANTrusted,
				Status:        "configured",
			})
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		left := score(nodes[i], consensusMode)
		right := score(nodes[j], consensusMode)
		if left != right {
			return left > right
		}
		return nodes[i].NodeID < nodes[j].NodeID
	})

	excluded := map[string]bool{}
	for _, wanted := range candidates {
		candidateNodes := []candidateNode{}
		for _, node := range nodes {
			policy := classify(node, consensusMode, now)
			if !policy.Eligible {
				if !excluded[node.NodeID] {
					decision.ExcludedNodes = append(decision.ExcludedNodes, nodeMetadata(node, policy.Reasons))
					excluded[node.NodeID] = true
				}
				continue
			}
			if hasModel(node, wanted) || len(node.Models) == 0 {
				placement := benchmarkPlacement(node, wanted, now)
				reasons := append([]string{}, policy.Reasons...)
				reasons = append(reasons, placement.Reasons...)
				candidateNodes = append(candidateNodes, candidateNode{
					node:      node,
					placement: placement,
					metadata:  nodeMetadata(node, reasons),
				})
			}
		}
		if len(candidateNodes) > 0 {
			sort.SliceStable(candidateNodes, func(i, j int) bool {
				left := candidateNodes[i].placement
				right := candidateNodes[j].placement
				if left.Eligible != right.Eligible {
					return left.Eligible
				}
				if left.Eligible && right.Eligible && left.Score != right.Score {
					return left.Score > right.Score
				}
				return false
			})
			decision.ResolvedModel = wanted
			for _, candidate := range candidateNodes {
				decision.CandidateNodes = append(decision.CandidateNodes, candidate.node.NodeID)
				decision.CandidateMetadata = append(decision.CandidateMetadata, candidate.metadata)
			}
			break
		}
	}

	if len(decision.CandidateNodes) == 0 {
		decision.Reasons = append(decision.Reasons, "no_policy_eligible_node")
		if consensusMode {
			decision.Reasons = append(decision.Reasons, "consensus_policy_applied")
		}
		return decision, false
	}
	if consensusMode && len(decision.CandidateNodes) > maxParticipants {
		decision.CandidateNodes = decision.CandidateNodes[:maxParticipants]
		if len(decision.CandidateMetadata) > maxParticipants {
			decision.CandidateMetadata = decision.CandidateMetadata[:maxParticipants]
		}
		decision.Reasons = append(decision.Reasons, "consensus_max_participants_applied")
	}
	if consensusMode && minParticipants > 0 && len(decision.CandidateNodes) < minParticipants {
		decision.Reasons = append(decision.Reasons, "consensus_min_participants_unmet")
		return decision, false
	}
	decision.SelectedNode = decision.CandidateNodes[0]
	if len(decision.CandidateNodes) > 1 {
		decision.FallbackNodes = append(decision.FallbackNodes, decision.CandidateNodes[1:]...)
	}
	decision.Reasons = append(decision.Reasons, "requested_model_available", "node_enabled", "node_approved", "trust_policy_eligible")
	if consensusMode {
		decision.Reasons = append(decision.Reasons, "consensus_policy_applied")
	} else {
		decision.Reasons = append(decision.Reasons, "single_route_policy_applied")
	}
	if len(decision.CandidateMetadata) > 0 {
		decision.Reasons = append(decision.Reasons, decision.CandidateMetadata[0].Reasons...)
	}
	if strings.Contains(decision.Strategy, "code") {
		decision.Reasons = append(decision.Reasons, "role_primary_code")
	}
	return decision, true
}

func consensusParticipantBounds(minParticipants, maxParticipants int) (int, int) {
	if minParticipants < 1 {
		minParticipants = ConsensusDefaultMinParticipants
	}
	if minParticipants > ConsensusMaximumParticipants {
		minParticipants = ConsensusMaximumParticipants
	}
	if maxParticipants < 1 {
		maxParticipants = ConsensusDefaultMaxParticipants
	}
	if maxParticipants < minParticipants {
		maxParticipants = minParticipants
	}
	if maxParticipants > ConsensusMaximumParticipants {
		maxParticipants = ConsensusMaximumParticipants
	}
	return minParticipants, maxParticipants
}

func IsConsensusDecision(decision Decision) bool {
	return isConsensusMode(decision.ExecutionMode, decision.Strategy)
}

func score(n appstate.Node, consensusMode bool) int {
	score := 0
	if n.Status == "healthy" {
		score += 100
	}
	if n.OllamaAvailable {
		score += 50
	}
	score += len(n.Models) * 5
	score -= n.ActiveJobs * 15
	for _, tag := range n.Tags {
		if tag == "primary-code" || tag == "heavy" {
			score += 20
		}
	}
	switch controlLevel(n) {
	case appstate.ControlLevelManaged:
		score += 15
	case appstate.ControlLevelPassive:
		score -= 20
	}
	switch trustLevel(n) {
	case appstate.TrustLevelLocal:
		score += 20
	case appstate.TrustLevelLANTrusted:
		score += 10
	case appstate.TrustLevelLANUnverified:
		score -= 40
	case appstate.TrustLevelExternal:
		score -= 1000
	}
	if consensusMode && controlLevel(n) == appstate.ControlLevelManaged {
		score += 20
	}
	return score
}

type nodePolicy struct {
	Eligible bool
	Reasons  []string
}

type candidateNode struct {
	node      appstate.Node
	placement benchmarkPlacementPolicy
	metadata  NodeMetadata
}

type benchmarkPlacementPolicy struct {
	Eligible bool
	Score    float64
	Reasons  []string
}

func classify(n appstate.Node, consensusMode bool, now time.Time) nodePolicy {
	reasons := []string{}
	if !n.Enabled || !nodeApproved(n) {
		if !n.Enabled {
			reasons = append(reasons, "node_disabled")
		}
		if !nodeApproved(n) {
			reasons = append(reasons, "node_not_approved")
		}
		return nodePolicy{Reasons: reasons}
	}
	if n.Status == "disabled" || n.Status == "failed" {
		return nodePolicy{Reasons: []string{"node_status_" + n.Status}}
	}
	if heartbeatRequired(n) {
		switch heartbeatFreshness(n, now) {
		case "missing":
			return nodePolicy{Reasons: []string{"heartbeat_missing"}}
		case "stale":
			return nodePolicy{Reasons: []string{"heartbeat_stale"}}
		}
	}
	control := controlLevel(n)
	trust := trustLevel(n)
	if trust == appstate.TrustLevelExternal {
		return nodePolicy{Reasons: []string{"trust_external_excluded"}}
	}
	if consensusMode {
		if control == appstate.ControlLevelPassive {
			return nodePolicy{Reasons: []string{"passive_consensus_excluded"}}
		}
		if trust == appstate.TrustLevelLANUnverified {
			return nodePolicy{Reasons: []string{"trust_lan_unverified_consensus_excluded"}}
		}
	}
	if control == appstate.ControlLevelManaged {
		reasons = append(reasons, "managed_control")
	} else {
		reasons = append(reasons, "passive_single_route_allowed")
	}
	switch trust {
	case appstate.TrustLevelLocal:
		reasons = append(reasons, "trust_local")
	case appstate.TrustLevelLANTrusted:
		reasons = append(reasons, "trust_lan_trusted")
	case appstate.TrustLevelLANUnverified:
		reasons = append(reasons, "trust_lan_unverified_deprioritized")
	}
	return nodePolicy{Eligible: true, Reasons: reasons}
}

func benchmarkPlacement(n appstate.Node, model string, now time.Time) benchmarkPlacementPolicy {
	if controlLevel(n) != appstate.ControlLevelManaged {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_passive_probe_ignored"}}
	}
	switch trustLevel(n) {
	case appstate.TrustLevelLocal, appstate.TrustLevelLANTrusted:
	default:
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_untrusted_ignored"}}
	}
	if benchmarkSource(n) != appstate.BenchmarkSourceSubscriberReported {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_source_ignored"}}
	}
	result, ok := latestBenchmarkResult(n, model)
	if !ok {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_summary_missing"}}
	}
	if resultSource(result) != appstate.BenchmarkSourceSubscriberReported {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_source_ignored"}}
	}
	status := stringValue(result["status"])
	if status != "" && status != "completed" && status != "reported" {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_status_ignored"}}
	}
	completedAt, ok := resultTime(result["completed_at"])
	if !ok {
		completedAt, ok = resultTime(n.Observed["benchmark_updated_at"])
	}
	if !ok || completedAt.IsZero() {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_freshness_missing"}}
	}
	if now.Sub(completedAt.UTC()) > BenchmarkPlacementFreshnessWindow {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_summary_stale"}}
	}
	rate := floatValue(result["tokens_per_second"])
	if rate <= 0 {
		rate = floatValue(result["output_tokens_per_second"])
	}
	if rate <= 0 {
		return benchmarkPlacementPolicy{Reasons: []string{"benchmark_placement_rate_missing"}}
	}
	return benchmarkPlacementPolicy{
		Eligible: true,
		Score:    rate,
		Reasons:  []string{"benchmark_placement_applied", "benchmark_placement_fresh"},
	}
}

func latestBenchmarkResult(n appstate.Node, model string) (map[string]interface{}, bool) {
	results := benchmarkResults(n)
	if len(results) == 0 && n.Observed != nil {
		if result, ok := n.Observed["benchmark_last_result"].(map[string]interface{}); ok {
			results = append(results, result)
		}
	}
	var best map[string]interface{}
	var bestTime time.Time
	for _, result := range results {
		resultModel := stringValue(result["model"])
		if model != "" && resultModel != "" && resultModel != model {
			continue
		}
		completedAt, _ := resultTime(result["completed_at"])
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

func benchmarkResults(n appstate.Node) []map[string]interface{} {
	if n.Observed == nil {
		return nil
	}
	switch values := n.Observed["benchmark_results"].(type) {
	case []map[string]interface{}:
		return append([]map[string]interface{}{}, values...)
	case []interface{}:
		results := make([]map[string]interface{}, 0, len(values))
		for _, value := range values {
			if result, ok := value.(map[string]interface{}); ok {
				results = append(results, result)
			}
		}
		return results
	default:
		return nil
	}
}

func benchmarkSource(n appstate.Node) string {
	if n.BenchmarkSource != "" {
		return n.BenchmarkSource
	}
	if n.Observed != nil {
		if source := stringValue(n.Observed["benchmark_source"]); source != "" {
			return source
		}
	}
	if controlLevel(n) == appstate.ControlLevelManaged {
		return appstate.BenchmarkSourceSubscriberReported
	}
	return appstate.BenchmarkSourceNone
}

func resultSource(result map[string]interface{}) string {
	source := stringValue(result["source"])
	if source == "" {
		return appstate.BenchmarkSourceSubscriberReported
	}
	return source
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func floatValue(value interface{}) float64 {
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

func resultTime(value interface{}) (time.Time, bool) {
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

func heartbeatRequired(n appstate.Node) bool {
	if controlLevel(n) != appstate.ControlLevelManaged {
		return false
	}
	if n.Observed == nil {
		return false
	}
	required, ok := n.Observed["heartbeat_required"].(bool)
	return ok && required
}

func heartbeatFreshness(n appstate.Node, now time.Time) string {
	if n.LastReportedAt == nil || n.LastReportedAt.IsZero() {
		return "missing"
	}
	if now.Sub(n.LastReportedAt.UTC()) > ManagedHeartbeatFreshnessWindow {
		return "stale"
	}
	return "fresh"
}

func nodeApproved(n appstate.Node) bool {
	if n.ApprovalState == "" {
		return n.Approved
	}
	return n.ApprovalState == appstate.ApprovalStateApproved && n.Approved
}

func nodeMetadata(n appstate.Node, reasons []string) NodeMetadata {
	return NodeMetadata{
		NodeID:           n.NodeID,
		ControlLevel:     controlLevel(n),
		TrustLevel:       trustLevel(n),
		CapabilitySource: n.CapabilitySource,
		Reasons:          append([]string{}, reasons...),
	}
}

func controlLevel(n appstate.Node) string {
	if n.ControlLevel == appstate.ControlLevelPassive {
		return appstate.ControlLevelPassive
	}
	return appstate.ControlLevelManaged
}

func trustLevel(n appstate.Node) string {
	switch n.TrustLevel {
	case appstate.TrustLevelLANTrusted, appstate.TrustLevelLANUnverified, appstate.TrustLevelExternal:
		return n.TrustLevel
	default:
		return appstate.TrustLevelLocal
	}
}

func isConsensusMode(executionMode, strategy string) bool {
	return strings.Contains(executionMode, "consensus") || strings.Contains(strategy, "consensus")
}

func hasModel(n appstate.Node, model string) bool {
	for _, m := range n.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}
