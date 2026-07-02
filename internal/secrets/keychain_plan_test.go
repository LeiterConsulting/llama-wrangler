package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOSKeychainPlanDocumentsFeasibilityAndGuardrails(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "14_os_keychain_plan.md"))
	if err != nil {
		t.Fatalf("read OS keychain plan: %v", err)
	}
	doc := string(raw)
	for _, expected := range []string{
		"admin token",
		"generated client API keys",
		"Splunk HEC token",
		"future frontier provider keys",
		"github.com/zalando/go-keyring",
		"github.com/99designs/keyring",
		"SecretBackend interface",
		"LLAMA_WRANGLER_SECRET_BACKEND=os_keychain",
		"LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1",
		"LLAMA_WRANGLER_SERVICE_MODE=1",
		"docs/15_service_wrapper_dry_run.md",
		"os_keychain_runtime",
		"non-destructive",
		"encrypted fallback",
		"support bundles",
		"must not include prompt bodies",
		"Phase A should not be marked complete on planning alone",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("OS keychain plan missing %q", expected)
		}
	}
}

func TestPhaseAClosureDocumentsCredentialDecision(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "17_phase_a_closure_decision.md"))
	if err != nil {
		t.Fatalf("read Phase A closure decision: %v", err)
	}
	doc := string(raw)
	for _, expected := range []string{
		"Phase A is complete",
		"encrypted fallback is the supported default and service credential path",
		"OS keychain remains interactive opt-in",
		"packaging and install hardening",
		"LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1",
		"Disposable launchd validation",
		"backend: encrypted_file",
		"fallback_available: true",
		"os_keychain_status: unavailable",
		"os_keychain_runtime: service_like",
		"support bundles",
		"must continue to keep these values out",
		"admin tokens",
		"prompt bodies",
		"Phase B can now proceed",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("Phase A closure decision missing %q", expected)
		}
	}
}
