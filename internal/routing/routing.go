package routing

import (
	"sort"
	"strings"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

type Request struct {
	Model         string
	ExecutionMode string
	SessionID     string
	Streaming     bool
	TaskType      string
}

type Decision struct {
	ModelAlias     string   `json:"model_alias"`
	ResolvedModel  string   `json:"resolved_model"`
	SelectedNode   string   `json:"selected_node"`
	CandidateNodes []string `json:"candidate_nodes"`
	FallbackNodes  []string `json:"fallback_nodes"`
	Strategy       string   `json:"routing_strategy"`
	Reasons        []string `json:"routing_reasons"`
	ExecutionMode  string   `json:"execution_mode"`
	Affinity       string   `json:"affinity"`
}

func Select(cfg config.Config, state appstate.State, req Request) (Decision, bool) {
	model := req.Model
	if model == "" {
		model = cfg.Routing.DefaultModelAlias
	}
	alias, isAlias := cfg.ModelAliases[model]
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
	}
	if decision.ExecutionMode == "" {
		decision.ExecutionMode = cfg.Routing.DefaultExecutionMode
	}

	nodes := make([]appstate.Node, 0, len(state.Nodes)+len(cfg.Subscribers))
	seen := map[string]bool{}
	for _, n := range state.Nodes {
		if n.Enabled || n.Approved {
			nodes = append(nodes, n)
			seen[n.NodeID] = true
		}
	}
	for _, sub := range cfg.Subscribers {
		if !seen[sub.NodeID] {
			nodes = append(nodes, appstate.Node{NodeID: sub.NodeID, URL: sub.URL, Enabled: true, Approved: true, Status: "configured"})
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return score(nodes[i]) > score(nodes[j])
	})

	for _, wanted := range candidates {
		for _, node := range nodes {
			if !eligible(node) {
				continue
			}
			if hasModel(node, wanted) || len(node.Models) == 0 {
				decision.ResolvedModel = wanted
				decision.CandidateNodes = append(decision.CandidateNodes, node.NodeID)
			}
		}
		if len(decision.CandidateNodes) > 0 {
			break
		}
	}

	if len(decision.CandidateNodes) == 0 {
		return decision, false
	}
	decision.SelectedNode = decision.CandidateNodes[0]
	if len(decision.CandidateNodes) > 1 {
		decision.FallbackNodes = append(decision.FallbackNodes, decision.CandidateNodes[1:]...)
	}
	decision.Reasons = append(decision.Reasons, "requested_model_available", "node_enabled", "local_first_policy")
	if strings.Contains(decision.Strategy, "code") {
		decision.Reasons = append(decision.Reasons, "role_primary_code")
	}
	return decision, true
}

func score(n appstate.Node) int {
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
	return score
}

func eligible(n appstate.Node) bool {
	if !n.Enabled && !n.Approved {
		return false
	}
	return n.Status != "disabled" && n.Status != "failed"
}

func hasModel(n appstate.Node, model string) bool {
	for _, m := range n.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}
