package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMarshalExample(t *testing.T) {
	t.Setenv("SPLUNK_HEC_TOKEN", "token")
	t.Setenv("LLAMA_WRANGLER_IDE_KEY", "ide-token")
	cfg, err := Load(filepath.Join("..", "..", "configs", "marshal.example.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Mode != "marshal" {
		t.Fatalf("mode = %q, want marshal", cfg.Server.Mode)
	}
	if cfg.Routing.DefaultExecutionMode != "single" {
		t.Fatalf("default mode = %q", cfg.Routing.DefaultExecutionMode)
	}
	if cfg.Routing.QueueSchedulingPolicy != "weighted_priority" {
		t.Fatalf("queue scheduling policy = %q", cfg.Routing.QueueSchedulingPolicy)
	}
	if cfg.Routing.QueuePriorityWeights.High != 3 || cfg.Routing.QueuePriorityWeights.Normal != 2 || cfg.Routing.QueuePriorityWeights.Low != 1 {
		t.Fatalf("queue priority weights = %+v", cfg.Routing.QueuePriorityWeights)
	}
	if cfg.Telemetry.SplunkHEC.Token != "token" {
		t.Fatalf("HEC token was not resolved from env")
	}
	if cfg.Auth.APIKeys[0].Key != "ide-token" {
		t.Fatalf("API key was not resolved from env")
	}
}

func TestDefaultSafePosture(t *testing.T) {
	cfg := Default("marshal")
	if cfg.Server.Listen != "127.0.0.1:11435" {
		t.Fatalf("default listen = %q, want localhost marshal port", cfg.Server.Listen)
	}
	if cfg.Frontier.Enabled {
		t.Fatalf("frontier should be disabled by default")
	}
	if !cfg.Frontier.LocalOnly {
		t.Fatalf("local_only should be true by default")
	}
	if cfg.Telemetry.StorePayloads {
		t.Fatalf("payload logging should be disabled by default")
	}
	if cfg.Routing.QueueSchedulingPolicy != "weighted_priority" {
		t.Fatalf("default queue scheduling policy = %q", cfg.Routing.QueueSchedulingPolicy)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(os.TempDir(), "no-such-llama-wrangler.yaml"))
	if err == nil {
		t.Fatalf("Load() expected error")
	}
}
