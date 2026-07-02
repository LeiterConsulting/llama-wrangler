package appstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"llama-wrangler/internal/config"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func writeState(t *testing.T, value interface{}) string {
	t.Helper()
	dir, err := AppDataDir()
	if err != nil {
		t.Fatalf("AppDataDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir app data: %v", err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}

func TestOpenCreatesVersionedState(t *testing.T) {
	isolatedHome(t)
	store, err := Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	state := store.Snapshot()
	if state.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, CurrentSchemaVersion)
	}
	if state.ConfigVersion != 1 {
		t.Fatalf("config version = %d, want 1", state.ConfigVersion)
	}
	if state.Nodes == nil || state.Sessions == nil || state.ClientAPIKeys == nil || state.MigrationHistory == nil {
		t.Fatalf("new state did not initialize collections")
	}
}

func TestOpenMigratesLegacyUnversionedState(t *testing.T) {
	isolatedHome(t)
	statePath := writeState(t, map[string]interface{}{
		"setup_complete": true,
		"node_id":        "legacy-node",
		"role":           "marshal",
		"config": map[string]interface{}{
			"version": "0.1.0",
			"server": map[string]interface{}{
				"mode":   "marshal",
				"listen": "127.0.0.1:11435",
			},
		},
	})

	store, err := Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	state := store.Snapshot()
	if state.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, CurrentSchemaVersion)
	}
	if state.ConfigVersion != 1 {
		t.Fatalf("config version = %d, want 1", state.ConfigVersion)
	}
	if len(state.MigrationHistory) != 2 {
		t.Fatalf("migration history length = %d, want 2", len(state.MigrationHistory))
	}
	if state.Nodes == nil || state.Sessions == nil || state.ClientAPIKeys == nil || state.EnrollmentQueue == nil {
		t.Fatalf("legacy migration did not initialize collections")
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read migrated state: %v", err)
	}
	var persisted State
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode migrated state: %v", err)
	}
	if persisted.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("persisted schema version = %d", persisted.SchemaVersion)
	}
}

func TestOpenMigratesV1NodesToControlTrustMetadata(t *testing.T) {
	isolatedHome(t)
	writeState(t, map[string]interface{}{
		"schema_version": 1,
		"config_version": 1,
		"setup_complete": true,
		"node_id":        "local-node",
		"role":           "marshal",
		"config":         config.Default("marshal"),
		"nodes": map[string]interface{}{
			"local-node": map[string]interface{}{
				"node_id":          "local-node",
				"display_name":     "Local Node",
				"role":             "marshal",
				"ollama_url":       "http://localhost:11434",
				"status":           "healthy",
				"enabled":          true,
				"approved":         true,
				"ollama_available": true,
				"last_seen":        "2026-07-02T18:00:00Z",
			},
			"manual-worker": map[string]interface{}{
				"node_id":      "manual-worker",
				"display_name": "Manual Worker",
				"url":          "http://worker.local:11436",
				"role":         "subscriber",
				"status":       "degraded",
				"enabled":      true,
				"approved":     true,
				"last_seen":    "2026-07-02T18:01:00Z",
			},
		},
		"sessions":          map[string]interface{}{},
		"client_api_keys":   []interface{}{},
		"audit":             []interface{}{},
		"enrollment_queue":  []interface{}{},
		"migration_history": []interface{}{},
		"created_at":        "2026-07-02T18:00:00Z",
		"updated_at":        "2026-07-02T18:00:00Z",
	})

	store, err := Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	state := store.Snapshot()
	if state.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, CurrentSchemaVersion)
	}
	if len(state.MigrationHistory) != 1 {
		t.Fatalf("migration history length = %d, want 1", len(state.MigrationHistory))
	}
	local := state.Nodes["local-node"]
	if local.ControlLevel != ControlLevelManaged || local.TrustLevel != TrustLevelLocal || local.ApprovalState != ApprovalStateApproved {
		t.Fatalf("local node metadata = %#v", local)
	}
	if local.CapabilitySource != CapabilitySourceSubscriberReported || local.HealthSource != HealthSourceSubscriberReported || local.ModelInventorySource != ModelInventorySourceSubscriberReported {
		t.Fatalf("local node sources = %#v", local)
	}
	if local.TelemetryLevel != TelemetryLevelRichMetadata || !local.ManagementSupported || local.WarmStateSupported {
		t.Fatalf("local node support metadata = %#v", local)
	}
	manual := state.Nodes["manual-worker"]
	if manual.ControlLevel != ControlLevelManaged || manual.TrustLevel != TrustLevelLANUnverified || manual.ApprovalState != ApprovalStateApproved {
		t.Fatalf("manual node metadata = %#v", manual)
	}
	if manual.LastObservedAt == nil || manual.LastReportedAt == nil {
		t.Fatalf("manual freshness metadata missing: %#v", manual)
	}
}

func TestUpsertPassiveEndpointDefaultsToObservedLimitedControl(t *testing.T) {
	isolatedHome(t)
	store, err := Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	node := Node{
		NodeID:       "studio-endpoint",
		DisplayName:  "Studio Endpoint",
		Role:         "passive",
		ControlLevel: ControlLevelPassive,
		URL:          "http://studio.local:11434",
		Enabled:      true,
		Approved:     false,
		Status:       "unknown",
	}
	if err := store.UpsertNode(node); err != nil {
		t.Fatalf("UpsertNode() error = %v", err)
	}
	got := store.Snapshot().Nodes["studio-endpoint"]
	if got.ControlLevel != ControlLevelPassive || got.TrustLevel != TrustLevelLANUnverified {
		t.Fatalf("passive control/trust metadata = %#v", got)
	}
	if got.CapabilitySource != CapabilitySourceMarshalObserved || got.HealthSource != HealthSourceMarshalObserved || got.ModelInventorySource != ModelInventorySourceMarshalObserved {
		t.Fatalf("passive sources = %#v", got)
	}
	if got.ApprovalState != ApprovalStatePending || got.Approved {
		t.Fatalf("passive approval metadata = %#v", got)
	}
	if got.ManagementSupported || got.WarmStateSupported || got.TelemetryLevel != TelemetryLevelMarshalObservedMetadata {
		t.Fatalf("passive support metadata = %#v", got)
	}
	if got.LastObservedAt == nil || got.LastReportedAt != nil {
		t.Fatalf("passive freshness metadata = %#v", got)
	}
}

func TestOpenRejectsFutureSchemaVersion(t *testing.T) {
	isolatedHome(t)
	writeState(t, map[string]interface{}{
		"schema_version": CurrentSchemaVersion + 100,
		"config_version": 1,
	})

	if _, err := Open(config.Default("marshal")); err == nil {
		t.Fatalf("Open() expected future schema error")
	}
}

func TestSaveConfigIncrementsConfigVersion(t *testing.T) {
	isolatedHome(t)
	store, err := Open(config.Default("marshal"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cfg := store.Snapshot().Config
	cfg.Routing.DefaultModelAlias = "local-code"
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	state := store.Snapshot()
	if state.ConfigVersion != 2 {
		t.Fatalf("config version = %d, want 2", state.ConfigVersion)
	}
	if state.Config.Routing.DefaultModelAlias != "local-code" {
		t.Fatalf("config was not saved")
	}
}
