package routing

import (
	"testing"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

func TestSelectPrefersHealthyModelNode(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-code"] = config.ModelAlias{Strategy: "best_code_node", Candidates: []string{"qwen2.5-coder:14b"}}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"slow": {
			NodeID:          "slow",
			Enabled:         true,
			Approved:        true,
			Status:          "healthy",
			OllamaAvailable: true,
			Models:          []appstate.ModelState{{Name: "qwen2.5-coder:14b", State: "installed"}},
			ActiveJobs:      3,
		},
		"fast": {
			NodeID:          "fast",
			Enabled:         true,
			Approved:        true,
			Status:          "healthy",
			OllamaAvailable: true,
			Models:          []appstate.ModelState{{Name: "qwen2.5-coder:14b", State: "warm"}},
			Tags:            []string{"primary-code"},
		},
	}}
	decision, ok := Select(cfg, state, Request{Model: "local-code"})
	if !ok {
		t.Fatalf("Select() found no node")
	}
	if decision.SelectedNode != "fast" {
		t.Fatalf("selected = %q, want fast", decision.SelectedNode)
	}
	if decision.ResolvedModel != "qwen2.5-coder:14b" {
		t.Fatalf("model = %q", decision.ResolvedModel)
	}
}

func TestSelectNoEligibleNode(t *testing.T) {
	cfg := config.Default("marshal")
	state := appstate.State{Nodes: map[string]appstate.Node{
		"disabled": {NodeID: "disabled", Enabled: false, Approved: false, Status: "disabled"},
	}}
	_, ok := Select(cfg, state, Request{Model: "local-fast"})
	if ok {
		t.Fatalf("Select() should fail with no eligible nodes")
	}
}

func TestSelectExcludesPendingAndRevokedNodes(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"pending-passive": {
			NodeID:        "pending-passive",
			ControlLevel:  appstate.ControlLevelPassive,
			Enabled:       true,
			Approved:      false,
			ApprovalState: appstate.ApprovalStatePending,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"revoked-managed": {
			NodeID:        "revoked-managed",
			ControlLevel:  appstate.ControlLevelManaged,
			Enabled:       true,
			Approved:      false,
			ApprovalState: appstate.ApprovalStateRevoked,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"approved-managed": {
			NodeID:        "approved-managed",
			ControlLevel:  appstate.ControlLevelManaged,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}}
	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node")
	}
	if decision.SelectedNode != "approved-managed" {
		t.Fatalf("selected = %q, want approved-managed", decision.SelectedNode)
	}
	for _, candidate := range decision.CandidateNodes {
		if candidate == "pending-passive" || candidate == "revoked-managed" {
			t.Fatalf("ineligible node was routed: %+v", decision.CandidateNodes)
		}
	}
}

func TestSelectUsesControlAndTrustMetadataForSingleRoute(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"passive-unverified": {
			NodeID:        "passive-unverified",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANUnverified,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"managed-trusted": {
			NodeID:        "managed-trusted",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"external": {
			NodeID:        "external",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelExternal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node: %+v", decision)
	}
	if decision.SelectedNode != "managed-trusted" {
		t.Fatalf("selected = %q, want managed-trusted; decision=%+v", decision.SelectedNode, decision)
	}
	if !containsString(decision.CandidateNodes, "passive-unverified") {
		t.Fatalf("approved passive lan_unverified should remain a single-route candidate: %+v", decision.CandidateNodes)
	}
	if !metadataHasReason(decision.CandidateMetadata, "passive-unverified", "trust_lan_unverified_deprioritized") {
		t.Fatalf("missing passive warning metadata: %+v", decision.CandidateMetadata)
	}
	if !metadataHasReason(decision.ExcludedNodes, "external", "trust_external_excluded") {
		t.Fatalf("missing external exclusion metadata: %+v", decision.ExcludedNodes)
	}
}

func TestSelectConsensusRequiresManagedTrustedNodes(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-consensus"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"llama3.1:8b"},
		ExecutionMode:   "consensus",
		MinParticipants: 2,
	}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"managed-local": {
			NodeID:        "managed-local",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"managed-lan": {
			NodeID:        "managed-lan",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"managed-unverified": {
			NodeID:        "managed-unverified",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLANUnverified,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"passive-trusted": {
			NodeID:        "passive-trusted",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-consensus"})
	if !ok {
		t.Fatalf("Select() found no consensus participants: %+v", decision)
	}
	if len(decision.CandidateNodes) != 2 {
		t.Fatalf("candidate nodes = %+v, want exactly two managed trusted participants", decision.CandidateNodes)
	}
	if containsString(decision.CandidateNodes, "managed-unverified") || containsString(decision.CandidateNodes, "passive-trusted") {
		t.Fatalf("ineligible consensus participant was included: %+v", decision.CandidateNodes)
	}
	if !metadataHasReason(decision.ExcludedNodes, "managed-unverified", "trust_lan_unverified_consensus_excluded") {
		t.Fatalf("missing lan_unverified consensus exclusion: %+v", decision.ExcludedNodes)
	}
	if !metadataHasReason(decision.ExcludedNodes, "passive-trusted", "passive_consensus_excluded") {
		t.Fatalf("missing passive consensus exclusion: %+v", decision.ExcludedNodes)
	}
}

func TestSelectConsensusRequiresMinimumParticipants(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-consensus"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"llama3.1:8b"},
		ExecutionMode:   "consensus",
		MinParticipants: 2,
	}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"managed-local": {
			NodeID:        "managed-local",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
		"passive-trusted": {
			NodeID:        "passive-trusted",
			ControlLevel:  appstate.ControlLevelPassive,
			TrustLevel:    appstate.TrustLevelLANTrusted,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-consensus"})
	if ok {
		t.Fatalf("Select() should fail with too few consensus participants: %+v", decision)
	}
	if !containsString(decision.Reasons, "consensus_min_participants_unmet") {
		t.Fatalf("missing min participant reason: %+v", decision.Reasons)
	}
	if decision.ConsensusRequired != 2 {
		t.Fatalf("consensus required = %d, want 2", decision.ConsensusRequired)
	}
}

func TestSelectConsensusAppliesSafeParticipantBounds(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["bounded-consensus"] = config.ModelAlias{
		Strategy:        "consensus",
		Candidates:      []string{"consensus-model"},
		ExecutionMode:   "consensus",
		MinParticipants: 2,
		MaxParticipants: 3,
	}
	state := appstate.State{Nodes: map[string]appstate.Node{}}
	for _, nodeID := range []string{"managed-d", "managed-b", "managed-a", "managed-c"} {
		state.Nodes[nodeID] = appstate.Node{
			NodeID:        nodeID,
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "consensus-model", State: "installed"}},
		}
	}

	decision, ok := Select(cfg, state, Request{Model: "bounded-consensus"})
	if !ok {
		t.Fatalf("Select() found no bounded consensus participants: %+v", decision)
	}
	want := []string{"managed-a", "managed-b", "managed-c"}
	if len(decision.CandidateNodes) != len(want) {
		t.Fatalf("candidate nodes = %+v, want %+v", decision.CandidateNodes, want)
	}
	for index := range want {
		if decision.CandidateNodes[index] != want[index] {
			t.Fatalf("candidate nodes = %+v, want deterministic %+v", decision.CandidateNodes, want)
		}
	}
	if decision.ConsensusRequired != 2 || decision.ConsensusLimit != 3 || !containsString(decision.Reasons, "consensus_max_participants_applied") {
		t.Fatalf("consensus bounds decision = %+v", decision)
	}

	min, max := consensusParticipantBounds(0, 99)
	if min != ConsensusDefaultMinParticipants || max != ConsensusMaximumParticipants {
		t.Fatalf("normalized participant bounds = %d/%d", min, max)
	}
}

func TestSelectExcludesStaleHeartbeatRequiredManagedNode(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	stale := time.Now().UTC().Add(-10 * time.Minute)
	fresh := time.Now().UTC()
	state := appstate.State{Nodes: map[string]appstate.Node{
		"stale-managed": {
			NodeID:         "stale-managed",
			ControlLevel:   appstate.ControlLevelManaged,
			TrustLevel:     appstate.TrustLevelLocal,
			Enabled:        true,
			Approved:       true,
			ApprovalState:  appstate.ApprovalStateApproved,
			Status:         "healthy",
			Models:         []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
			LastReportedAt: &stale,
			Observed:       map[string]interface{}{"heartbeat_required": true},
		},
		"fresh-managed": {
			NodeID:         "fresh-managed",
			ControlLevel:   appstate.ControlLevelManaged,
			TrustLevel:     appstate.TrustLevelLocal,
			Enabled:        true,
			Approved:       true,
			ApprovalState:  appstate.ApprovalStateApproved,
			Status:         "healthy",
			Models:         []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
			LastReportedAt: &fresh,
			Observed:       map[string]interface{}{"heartbeat_required": true},
		},
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node: %+v", decision)
	}
	if decision.SelectedNode != "fresh-managed" {
		t.Fatalf("selected = %q, want fresh-managed; decision=%+v", decision.SelectedNode, decision)
	}
	if containsString(decision.CandidateNodes, "stale-managed") {
		t.Fatalf("stale node was candidate: %+v", decision.CandidateNodes)
	}
	if !metadataHasReason(decision.ExcludedNodes, "stale-managed", "heartbeat_stale") {
		t.Fatalf("missing stale exclusion metadata: %+v", decision.ExcludedNodes)
	}
}

func TestSelectDoesNotRequireHeartbeatForLegacyManagedNode(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"legacy-managed": {
			NodeID:        "legacy-managed",
			ControlLevel:  appstate.ControlLevelManaged,
			TrustLevel:    appstate.TrustLevelLocal,
			Enabled:       true,
			Approved:      true,
			ApprovalState: appstate.ApprovalStateApproved,
			Status:        "healthy",
			Models:        []appstate.ModelState{{Name: "llama3.1:8b", State: "installed"}},
		},
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok || decision.SelectedNode != "legacy-managed" {
		t.Fatalf("legacy managed selection = %+v ok=%v", decision, ok)
	}
}

func TestSelectPrefersFreshTrustedBenchmarkPlacement(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	state := appstate.State{Nodes: map[string]appstate.Node{
		"base-favored":  benchmarkedManagedNode("base-favored", appstate.TrustLevelLANTrusted, "llama3.1:8b", 21, time.Now().UTC(), []string{"primary-code"}),
		"bench-favored": benchmarkedManagedNode("bench-favored", appstate.TrustLevelLANTrusted, "llama3.1:8b", 64, time.Now().UTC(), nil),
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node: %+v", decision)
	}
	if decision.SelectedNode != "bench-favored" {
		t.Fatalf("selected = %q, want bench-favored; decision=%+v", decision.SelectedNode, decision)
	}
	if !metadataHasReason(decision.CandidateMetadata, "bench-favored", "benchmark_placement_applied") {
		t.Fatalf("missing benchmark placement reason: %+v", decision.CandidateMetadata)
	}
}

func TestSelectIgnoresPassiveProbeForBenchmarkPlacement(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	passive := benchmarkedPassiveNode("passive-probe", "llama3.1:8b", 999, time.Now().UTC())
	passive.Tags = []string{"primary-code", "heavy"}
	passive.Models = append(passive.Models,
		appstate.ModelState{Name: "extra-a", State: "installed"},
		appstate.ModelState{Name: "extra-b", State: "installed"},
		appstate.ModelState{Name: "extra-c", State: "installed"},
		appstate.ModelState{Name: "extra-d", State: "installed"},
		appstate.ModelState{Name: "extra-e", State: "installed"},
		appstate.ModelState{Name: "extra-f", State: "installed"},
	)
	state := appstate.State{Nodes: map[string]appstate.Node{
		"passive-probe": passive,
		"managed-fresh": benchmarkedManagedNode("managed-fresh", appstate.TrustLevelLANTrusted, "llama3.1:8b", 18, time.Now().UTC(), nil),
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node: %+v", decision)
	}
	if decision.SelectedNode != "managed-fresh" {
		t.Fatalf("selected = %q, want managed-fresh; decision=%+v", decision.SelectedNode, decision)
	}
	if !metadataHasReason(decision.CandidateMetadata, "passive-probe", "benchmark_placement_passive_probe_ignored") {
		t.Fatalf("missing passive probe ignored reason: %+v", decision.CandidateMetadata)
	}
}

func TestSelectIgnoresUntrustedOrStaleBenchmarkPlacement(t *testing.T) {
	cfg := config.Default("marshal")
	cfg.ModelAliases["local-fast"] = config.ModelAlias{Strategy: "fastest_available", Candidates: []string{"llama3.1:8b"}, ExecutionMode: "single"}
	staleTime := time.Now().UTC().Add(-(BenchmarkPlacementFreshnessWindow + time.Hour))
	untrusted := benchmarkedManagedNode("untrusted-fast", appstate.TrustLevelLANUnverified, "llama3.1:8b", 900, time.Now().UTC(), []string{"primary-code", "heavy"})
	stale := benchmarkedManagedNode("stale-fast", appstate.TrustLevelLANTrusted, "llama3.1:8b", 800, staleTime, []string{"primary-code"})
	state := appstate.State{Nodes: map[string]appstate.Node{
		"trusted-fresh":  benchmarkedManagedNode("trusted-fresh", appstate.TrustLevelLANTrusted, "llama3.1:8b", 24, time.Now().UTC(), nil),
		"untrusted-fast": untrusted,
		"stale-fast":     stale,
	}}

	decision, ok := Select(cfg, state, Request{Model: "local-fast"})
	if !ok {
		t.Fatalf("Select() found no node: %+v", decision)
	}
	if decision.SelectedNode != "trusted-fresh" {
		t.Fatalf("selected = %q, want trusted-fresh; decision=%+v", decision.SelectedNode, decision)
	}
	if !metadataHasReason(decision.CandidateMetadata, "untrusted-fast", "benchmark_placement_untrusted_ignored") {
		t.Fatalf("missing untrusted ignored reason: %+v", decision.CandidateMetadata)
	}
	if !metadataHasReason(decision.CandidateMetadata, "stale-fast", "benchmark_placement_summary_stale") {
		t.Fatalf("missing stale ignored reason: %+v", decision.CandidateMetadata)
	}
}

func benchmarkedManagedNode(nodeID, trustLevel, model string, tokensPerSecond float64, completedAt time.Time, tags []string) appstate.Node {
	return appstate.Node{
		NodeID:          nodeID,
		ControlLevel:    appstate.ControlLevelManaged,
		TrustLevel:      trustLevel,
		Enabled:         true,
		Approved:        true,
		ApprovalState:   appstate.ApprovalStateApproved,
		Status:          "healthy",
		OllamaAvailable: true,
		BenchmarkSource: appstate.BenchmarkSourceSubscriberReported,
		Models:          []appstate.ModelState{{Name: model, State: "installed", TokensSec: tokensPerSecond}},
		Tags:            tags,
		Observed: map[string]interface{}{
			"benchmark_source":     appstate.BenchmarkSourceSubscriberReported,
			"benchmark_updated_at": completedAt,
			"benchmark_results": []map[string]interface{}{{
				"source":            appstate.BenchmarkSourceSubscriberReported,
				"mode":              "subscriber_reported",
				"model":             model,
				"status":            "completed",
				"completed_at":      completedAt,
				"tokens_per_second": tokensPerSecond,
			}},
		},
	}
}

func benchmarkedPassiveNode(nodeID, model string, tokensPerSecond float64, completedAt time.Time) appstate.Node {
	return appstate.Node{
		NodeID:          nodeID,
		ControlLevel:    appstate.ControlLevelPassive,
		TrustLevel:      appstate.TrustLevelLANTrusted,
		Enabled:         true,
		Approved:        true,
		ApprovalState:   appstate.ApprovalStateApproved,
		Status:          "healthy",
		OllamaAvailable: true,
		BenchmarkSource: appstate.BenchmarkSourceMarshalObserved,
		Models:          []appstate.ModelState{{Name: model, State: "installed", TokensSec: tokensPerSecond}},
		Observed: map[string]interface{}{
			"benchmark_source":     appstate.BenchmarkSourceMarshalObserved,
			"benchmark_updated_at": completedAt,
			"benchmark_results": []map[string]interface{}{{
				"source":            appstate.BenchmarkSourceMarshalObserved,
				"mode":              "marshal_observed_api_tags",
				"model":             model,
				"status":            "probe_ok",
				"completed_at":      completedAt,
				"tokens_per_second": tokensPerSecond,
			}},
		},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func metadataHasReason(nodes []NodeMetadata, nodeID, reason string) bool {
	for _, node := range nodes {
		if node.NodeID != nodeID {
			continue
		}
		return containsString(node.Reasons, reason)
	}
	return false
}
