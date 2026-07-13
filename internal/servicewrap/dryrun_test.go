package servicewrap

import (
	"strings"
	"testing"
)

func TestLaunchdDryRunPlanIncludesServiceModeAndFallbackWarnings(t *testing.T) {
	plan, err := NewDryRunPlan(Options{
		Target:         TargetLaunchd,
		BinaryPath:     "/Applications/Llama Wrangler/llama-wrangler",
		Mode:           "start",
		EnableKeychain: true,
		Environment: map[string]string{
			"HOME":                            "/tmp/llama-wrangler-home",
			"LLAMA_WRANGLER_KEYCHAIN_SERVICE": "llama-wrangler-dryrun-test",
		},
		Label:           "com.example.llama-wrangler",
		WorkingDir:      "/Applications/Llama Wrangler",
		LaunchAgentsDir: "/tmp/llama-wrangler-launch-agents",
	})
	if err != nil {
		t.Fatalf("NewDryRunPlan() error = %v", err)
	}
	if !plan.DryRun || plan.Target != TargetLaunchd || plan.Label != "com.example.llama-wrangler" {
		t.Fatalf("plan metadata = %+v", plan)
	}
	if got := plan.Environment["LLAMA_WRANGLER_SERVICE_MODE"]; got != "1" {
		t.Fatalf("service mode env = %q", got)
	}
	if got := plan.Environment["LLAMA_WRANGLER_SECRET_BACKEND"]; got != "os_keychain" {
		t.Fatalf("secret backend env = %q", got)
	}
	if plan.WrapperPath != "/tmp/llama-wrangler-launch-agents/com.example.llama-wrangler.plist" {
		t.Fatalf("wrapper path = %q", plan.WrapperPath)
	}
	if got := plan.Environment["LLAMA_WRANGLER_KEYCHAIN_SERVICE"]; got != "llama-wrangler-dryrun-test" {
		t.Fatalf("keychain service env = %q", got)
	}
	if got := plan.Environment["HOME"]; got != "/tmp/llama-wrangler-home" {
		t.Fatalf("home env = %q", got)
	}
	for _, required := range []string{
		"<key>ProgramArguments</key>",
		"<string>/Applications/Llama Wrangler/llama-wrangler</string>",
		"<key>EnvironmentVariables</key>",
		"LLAMA_WRANGLER_SERVICE_MODE",
		"LLAMA_WRANGLER_SECRET_BACKEND",
		"LLAMA_WRANGLER_KEYCHAIN_SERVICE",
	} {
		if !strings.Contains(plan.LaunchdPlist, required) {
			t.Fatalf("plist missing %q:\n%s", required, plan.LaunchdPlist)
		}
	}
	joinedWarnings := strings.Join(plan.Warnings, " ")
	if !strings.Contains(joinedWarnings, "Dry run only") || !strings.Contains(joinedWarnings, "Encrypted fallback remains") {
		t.Fatalf("warnings = %+v", plan.Warnings)
	}
	if len(plan.ValidationCommands) == 0 || len(plan.KeychainCheckCommands) == 0 {
		t.Fatalf("validation commands missing: %+v", plan)
	}
}

func TestLaunchdDryRunPlanUsesEncryptedFallbackByDefault(t *testing.T) {
	plan, err := NewDryRunPlan(Options{BinaryPath: "/usr/local/bin/llama-wrangler"})
	if err != nil {
		t.Fatalf("NewDryRunPlan() error = %v", err)
	}
	if got := plan.Environment["LLAMA_WRANGLER_SECRET_BACKEND"]; got != "encrypted_file" {
		t.Fatalf("default secret backend = %q", got)
	}
	if got := plan.Environment["LLAMA_WRANGLER_SERVICE_MODE"]; got != "1" {
		t.Fatalf("service mode env = %q", got)
	}
}

func TestLaunchdDryRunRejectsUnsafeLabel(t *testing.T) {
	for _, label := range []string{"../com.example", "com.example/agent", "com.example agent"} {
		_, err := NewDryRunPlan(Options{BinaryPath: "/usr/local/bin/llama-wrangler", Label: label})
		if err == nil || !strings.Contains(err.Error(), "invalid launchd label") {
			t.Fatalf("label %q error = %v, want invalid-label error", label, err)
		}
	}
}

func TestLaunchdDryRunRejectsConfigWithStartMode(t *testing.T) {
	_, err := NewDryRunPlan(Options{
		BinaryPath: "/usr/local/bin/llama-wrangler",
		Mode:       "start",
		ConfigPath: "/tmp/marshal.yaml",
	})
	if err == nil || !strings.Contains(err.Error(), "config path requires") {
		t.Fatalf("NewDryRunPlan() error = %v, want config/mode error", err)
	}
}

func TestLaunchdDryRunEscapesPlistValues(t *testing.T) {
	plan, err := NewDryRunPlan(Options{
		BinaryPath: "/tmp/llama&wrangler",
		Mode:       "marshal",
		ConfigPath: "/tmp/a<b>.yaml",
		Label:      "com.example.llama-wrangler",
		Environment: map[string]string{
			"TEST_VALUE": "llama\"wrangler",
		},
	})
	if err != nil {
		t.Fatalf("NewDryRunPlan() error = %v", err)
	}
	for _, required := range []string{
		"llama&amp;wrangler",
		"a&lt;b&gt;.yaml",
		"llama&quot;wrangler",
	} {
		if !strings.Contains(plan.LaunchdPlist, required) {
			t.Fatalf("escaped plist missing %q:\n%s", required, plan.LaunchdPlist)
		}
	}
}
