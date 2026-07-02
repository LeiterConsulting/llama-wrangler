package appstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedPassivePlanningDocumentsDataModelAndUIFlows(t *testing.T) {
	controlModesRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "16_node_control_modes.md"))
	if err != nil {
		t.Fatalf("read node control modes: %v", err)
	}
	phaseBRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "18_phase_b_managed_passive_plan.md"))
	if err != nil {
		t.Fatalf("read Phase B managed/passive plan: %v", err)
	}
	doc := string(controlModesRaw) + "\n" + string(phaseBRaw)
	for _, expected := range []string{
		"Managed Node",
		"Passive Endpoint",
		"control_level",
		"trust_level",
		"managed",
		"passive",
		"lan_trusted",
		"lan_unverified",
		"capability_source",
		"subscriber_reported",
		"marshal_observed",
		"Install/enroll Wrangler subscriber",
		"Add existing Ollama endpoint",
		"approval_state",
		"warm_state_supported",
		"management_supported",
		"consensus",
		"benchmarks",
		"UI badges",
		"should not place enrollment secrets",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("managed/passive planning docs missing %q", expected)
		}
	}
}
