package routing

import (
	"testing"

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
