package session

import (
	"testing"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
)

func TestApplyAffinityDoesNotSelectPolicyExcludedNode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := appstate.Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.UpdateSession(appstate.Session{
		SessionID:    "chat-1",
		AffinityMode: "strict",
		NodeID:       "external-or-revoked-node",
		Model:        "llama3.1:8b",
	}); err != nil {
		t.Fatalf("update session: %v", err)
	}

	selected := ApplyAffinity(store, "chat-1", "strict", "managed-local", "llama3.1:8b", "req_1", []string{"managed-local"})
	if selected != "managed-local" {
		t.Fatalf("selected = %q, want policy-eligible managed-local", selected)
	}
	session, ok := store.Session("chat-1")
	if !ok {
		t.Fatalf("session not persisted")
	}
	if session.NodeID != "managed-local" {
		t.Fatalf("session node = %q, want managed-local", session.NodeID)
	}
}

func TestApplyAffinityKeepsEligibleStrictNode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := appstate.Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.UpdateSession(appstate.Session{
		SessionID:    "chat-1",
		AffinityMode: "strict",
		NodeID:       "managed-lan",
		Model:        "llama3.1:8b",
	}); err != nil {
		t.Fatalf("update session: %v", err)
	}

	selected := ApplyAffinity(store, "chat-1", "strict", "managed-local", "llama3.1:8b", "req_1", []string{"managed-local", "managed-lan"})
	if selected != "managed-lan" {
		t.Fatalf("selected = %q, want existing eligible managed-lan", selected)
	}
}
