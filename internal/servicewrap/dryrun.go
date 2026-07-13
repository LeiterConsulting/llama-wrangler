package servicewrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	TargetLaunchd = "launchd"
)

type Options struct {
	Target          string
	BinaryPath      string
	ConfigPath      string
	Mode            string
	EnableKeychain  bool
	Environment     map[string]string
	Label           string
	WorkingDir      string
	LogDir          string
	LaunchAgentsDir string
	KeepAlive       bool
	RunAtLoad       bool
	LimitLoadToUser string
}

type Plan struct {
	Target                string            `json:"target"`
	Platform              string            `json:"platform"`
	DryRun                bool              `json:"dry_run"`
	Supported             bool              `json:"supported"`
	Label                 string            `json:"label"`
	WrapperPath           string            `json:"wrapper_path"`
	ProgramArguments      []string          `json:"program_arguments"`
	Environment           map[string]string `json:"environment"`
	LaunchdPlist          string            `json:"launchd_plist,omitempty"`
	ValidationCommands    []string          `json:"validation_commands"`
	KeychainCheckCommands []string          `json:"keychain_check_commands"`
	Warnings              []string          `json:"warnings,omitempty"`
	Notes                 []string          `json:"notes,omitempty"`
}

func NewDryRunPlan(opts Options) (Plan, error) {
	normalizeOptions(&opts)
	switch opts.Target {
	case TargetLaunchd:
		return launchdPlan(opts)
	default:
		return Plan{}, fmt.Errorf("unsupported service dry-run target %q", opts.Target)
	}
}

func MarshalJSON(plan Plan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}

func normalizeOptions(opts *Options) {
	opts.Target = strings.ToLower(strings.TrimSpace(opts.Target))
	if opts.Target == "" {
		opts.Target = TargetLaunchd
	}
	if opts.Mode == "" {
		opts.Mode = "start"
	}
	if opts.Label == "" {
		opts.Label = "com.llama-wrangler.marshal"
	}
	if opts.LogDir == "" {
		opts.LogDir = filepath.Join(userHomeDir(), "Library", "Logs", "Llama Wrangler")
	} else {
		opts.LogDir = expandUserPath(opts.LogDir)
	}
	if opts.LaunchAgentsDir == "" {
		opts.LaunchAgentsDir = filepath.Join(userHomeDir(), "Library", "LaunchAgents")
	} else {
		opts.LaunchAgentsDir = expandUserPath(opts.LaunchAgentsDir)
	}
	if opts.WorkingDir == "" && opts.BinaryPath != "" {
		opts.WorkingDir = filepath.Dir(opts.BinaryPath)
	}
}

func launchdPlan(opts Options) (Plan, error) {
	if strings.TrimSpace(opts.BinaryPath) == "" {
		return Plan{}, fmt.Errorf("binary path is required for launchd dry-run")
	}
	if opts.ConfigPath != "" && opts.Mode == "start" {
		return Plan{}, fmt.Errorf("config path requires marshal, subscriber, or standalone mode")
	}
	if !validLaunchdLabel(opts.Label) {
		return Plan{}, fmt.Errorf("invalid launchd label %q", opts.Label)
	}
	args := []string{opts.BinaryPath, opts.Mode}
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	env := map[string]string{
		"LLAMA_WRANGLER_SECRET_BACKEND": "encrypted_file",
		"LLAMA_WRANGLER_SERVICE_MODE":   "1",
	}
	for key, value := range opts.Environment {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = value
	}
	env["LLAMA_WRANGLER_SERVICE_MODE"] = "1"
	if opts.EnableKeychain {
		env["LLAMA_WRANGLER_SECRET_BACKEND"] = "os_keychain"
	}
	plist := renderLaunchdPlist(opts, args, env)
	plistPath := filepath.Join(opts.LaunchAgentsDir, opts.Label+".plist")
	warnings := []string{
		"Dry run only: this command does not write a plist, load launchd, start a service, or modify keychain items.",
		"launchd may run with a different environment and keychain session than an interactive shell; verify before relying on OS keychain storage.",
		"Encrypted fallback remains required and available even when OS keychain is enabled.",
	}
	if runtime.GOOS != "darwin" {
		warnings = append(warnings, "launchd dry-run output is intended for macOS; current runtime is "+runtime.GOOS+".")
	}
	return Plan{
		Target:           TargetLaunchd,
		Platform:         runtime.GOOS,
		DryRun:           true,
		Supported:        runtime.GOOS == "darwin",
		Label:            opts.Label,
		WrapperPath:      plistPath,
		ProgramArguments: args,
		Environment:      env,
		LaunchdPlist:     plist,
		ValidationCommands: []string{
			"plutil -lint " + plistPath,
			"launchctl bootstrap gui/$(id -u) " + plistPath,
			"launchctl print gui/$(id -u)/" + opts.Label,
			"launchctl bootout gui/$(id -u)/" + opts.Label,
		},
		KeychainCheckCommands: []string{
			"LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1 go test -v ./internal/secrets -run TestOSKeychainBackendPlatformOptIn",
			"LLAMA_WRANGLER_SECRET_BACKEND=os_keychain LLAMA_WRANGLER_SERVICE_MODE=1 " + opts.BinaryPath + " start",
			"curl -sS http://localhost:11435/wrangler/ui/bootstrap | jq '.secret_storage'",
		},
		Warnings: warnings,
		Notes: []string{
			"Review the plist before writing it to disk.",
			"Use a dedicated local test user or disposable app-data directory for service-wrapper keychain experiments when practical.",
			"Expected metadata in service-like runs includes os_keychain_runtime=service_like and os_keychain_service_mode=true.",
		},
	}, nil
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return "~"
}

func expandUserPath(path string) string {
	if path == "~" {
		return userHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(userHomeDir(), strings.TrimPrefix(path, "~/"))
	}
	return path
}

func validLaunchdLabel(label string) bool {
	if label == "" || len(label) > 128 || strings.Contains(label, "..") {
		return false
	}
	for _, char := range label {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func renderLaunchdPlist(opts Options, args []string, env map[string]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	writeKeyString(&b, "Label", opts.Label, 1)
	b.WriteString("\t<key>ProgramArguments</key>\n")
	b.WriteString("\t<array>\n")
	for _, arg := range args {
		writeString(&b, arg, 2)
	}
	b.WriteString("\t</array>\n")
	if opts.WorkingDir != "" {
		writeKeyString(&b, "WorkingDirectory", opts.WorkingDir, 1)
	}
	b.WriteString("\t<key>EnvironmentVariables</key>\n")
	b.WriteString("\t<dict>\n")
	for _, key := range sortedEnvKeys(env) {
		writeKeyString(&b, key, env[key], 2)
	}
	b.WriteString("\t</dict>\n")
	writeKeyBool(&b, "RunAtLoad", opts.RunAtLoad, 1)
	writeKeyBool(&b, "KeepAlive", opts.KeepAlive, 1)
	writeKeyString(&b, "StandardOutPath", filepath.Join(opts.LogDir, "launchd.out.log"), 1)
	writeKeyString(&b, "StandardErrorPath", filepath.Join(opts.LogDir, "launchd.err.log"), 1)
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func sortedEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func writeKeyString(b *strings.Builder, key, value string, tabs int) {
	indent(b, tabs)
	b.WriteString("<key>")
	b.WriteString(escapePlist(key))
	b.WriteString("</key>\n")
	writeString(b, value, tabs)
}

func writeString(b *strings.Builder, value string, tabs int) {
	indent(b, tabs)
	b.WriteString("<string>")
	b.WriteString(escapePlist(value))
	b.WriteString("</string>\n")
}

func writeKeyBool(b *strings.Builder, key string, value bool, tabs int) {
	indent(b, tabs)
	b.WriteString("<key>")
	b.WriteString(escapePlist(key))
	b.WriteString("</key>\n")
	indent(b, tabs)
	if value {
		b.WriteString("<true/>\n")
		return
	}
	b.WriteString("<false/>\n")
}

func indent(b *strings.Builder, tabs int) {
	for i := 0; i < tabs; i++ {
		b.WriteString("\t")
	}
}

func escapePlist(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	return value
}
