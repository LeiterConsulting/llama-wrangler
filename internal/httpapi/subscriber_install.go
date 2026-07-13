package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/servicewrap"
)

const (
	subscriberHeartbeatCredentialEnv = "LLAMA_WRANGLER_SUBSCRIBER_HEARTBEAT_CREDENTIAL"
	subscriberCredentialPlaceholder  = "<credential-from-rotation-response>"
	marshalURLPlaceholder            = "<marshal-url>"
)

type subscriberCredentialInstallPlan struct {
	NodeID                 string                              `json:"node_id"`
	CredentialHint         string                              `json:"credential_hint"`
	CredentialPlaceholder  string                              `json:"credential_placeholder"`
	Header                 string                              `json:"header"`
	EnvironmentVariable    string                              `json:"environment_variable"`
	ConfigKey              string                              `json:"config_key"`
	ConfigSnippet          string                              `json:"config_snippet"`
	ShellExportCommand     string                              `json:"shell_export_command"`
	EnvFilePath            string                              `json:"env_file_path"`
	EnvFileTemplate        string                              `json:"env_file_template"`
	LaunchdDryRunCommand   string                              `json:"launchd_dry_run_command"`
	HeartbeatCheckCommand  string                              `json:"heartbeat_check_command"`
	ServiceWrapper         subscriberServiceWrapperInstallPlan `json:"service_wrapper"`
	SubscriberRestartNotes []string                            `json:"subscriber_restart_notes"`
	Warnings               []string                            `json:"warnings"`
}

type subscriberServiceWrapperInstallPlan struct {
	Target               string            `json:"target"`
	Label                string            `json:"label"`
	Mode                 string            `json:"mode"`
	BinaryPath           string            `json:"binary_path"`
	ConfigPath           string            `json:"config_path"`
	PlistPath            string            `json:"plist_path"`
	LogDir               string            `json:"log_dir"`
	Environment          map[string]string `json:"environment"`
	LaunchdPlistTemplate string            `json:"launchd_plist_template"`
	InstallCommands      []string          `json:"install_commands"`
	ValidationCommands   []string          `json:"validation_commands"`
	UninstallCommands    []string          `json:"uninstall_commands"`
	Notes                []string          `json:"notes"`
}

func buildSubscriberCredentialInstallPlan(node appstate.Node, credentialHint string) subscriberCredentialInstallPlan {
	nodeID := strings.TrimSpace(node.NodeID)
	if nodeID == "" {
		nodeID = "<node-id>"
	}
	label := "com.llama-wrangler.subscriber." + launchdSafeLabelSuffix(nodeID)
	binaryPath := "./llama-wrangler"
	configPath := "./subscriber.yaml"
	envFilePath := "./llama-wrangler-subscriber.env"
	logDir := "~/Library/Logs/Llama Wrangler"
	plistPath := "~/Library/LaunchAgents/" + label + ".plist"
	launchAgentsDir := "$HOME/Library/LaunchAgents"
	logDirCommandPath := "$HOME/Library/Logs/Llama Wrangler"
	plistCommandPath := "$HOME/Library/LaunchAgents/" + label + ".plist"
	env := map[string]string{
		subscriberHeartbeatCredentialEnv: subscriberCredentialPlaceholder,
	}
	dryRun, _ := servicewrap.NewDryRunPlan(servicewrap.Options{
		Target:      servicewrap.TargetLaunchd,
		BinaryPath:  binaryPath,
		Mode:        "subscriber",
		ConfigPath:  configPath,
		Environment: env,
		Label:       label,
		LogDir:      logDir,
	})
	heartbeatBody, _ := json.Marshal(map[string]string{
		"node_id": nodeID,
		"status":  "healthy",
	})
	envFileTemplate := subscriberHeartbeatCredentialEnv + "=" + shellSingleQuote(subscriberCredentialPlaceholder)
	launchdDryRunCommand := strings.Join([]string{
		"llama-wrangler service-dry-run",
		"--target launchd",
		"--mode subscriber",
		"--config " + shellSingleQuote(configPath),
		"--label " + shellSingleQuote(label),
		"--env " + shellSingleQuote(subscriberHeartbeatCredentialEnv+"="+subscriberCredentialPlaceholder),
	}, " ")
	return subscriberCredentialInstallPlan{
		NodeID:                nodeID,
		CredentialHint:        credentialHint,
		CredentialPlaceholder: subscriberCredentialPlaceholder,
		Header:                "X-Llama-Wrangler-Subscriber-Token",
		EnvironmentVariable:   subscriberHeartbeatCredentialEnv,
		ConfigKey:             "registration.heartbeat_credential_env",
		EnvFilePath:           envFilePath,
		EnvFileTemplate:       envFileTemplate,
		ConfigSnippet: strings.Join([]string{
			"registration:",
			"  marshal_url: \"" + marshalURLPlaceholder + "\"",
			"  heartbeat_credential_env: " + subscriberHeartbeatCredentialEnv,
		}, "\n"),
		ShellExportCommand:   "export " + subscriberHeartbeatCredentialEnv + "=" + shellSingleQuote(subscriberCredentialPlaceholder),
		LaunchdDryRunCommand: launchdDryRunCommand,
		HeartbeatCheckCommand: fmt.Sprintf(
			"curl -fsS -X POST %s -H %s -H %s -d %s",
			shellSingleQuote(marshalURLPlaceholder+"/subscriber/heartbeat"),
			shellSingleQuote("Content-Type: application/json"),
			shellSingleQuote("X-Llama-Wrangler-Subscriber-Token: "+subscriberCredentialPlaceholder),
			shellSingleQuote(string(heartbeatBody)),
		),
		ServiceWrapper: subscriberServiceWrapperInstallPlan{
			Target:               servicewrap.TargetLaunchd,
			Label:                label,
			Mode:                 "subscriber",
			BinaryPath:           binaryPath,
			ConfigPath:           configPath,
			PlistPath:            plistPath,
			LogDir:               logDir,
			Environment:          env,
			LaunchdPlistTemplate: dryRun.LaunchdPlist,
			InstallCommands: []string{
				"install -d " + shellDoubleQuote(launchAgentsDir) + " " + shellDoubleQuote(logDirCommandPath),
				"# Write service_wrapper.launchd_plist_template to " + plistCommandPath,
				"plutil -lint " + shellDoubleQuote(plistCommandPath),
				"launchctl bootstrap gui/$(id -u) " + shellDoubleQuote(plistCommandPath),
				"launchctl kickstart -k gui/$(id -u)/" + shellSingleQuote(label),
			},
			ValidationCommands: []string{
				"launchctl print gui/$(id -u)/" + shellSingleQuote(label),
				fmt.Sprintf(
					"curl -fsS -X POST %s -H %s -H %s -d %s",
					shellSingleQuote(marshalURLPlaceholder+"/subscriber/heartbeat"),
					shellSingleQuote("Content-Type: application/json"),
					shellSingleQuote("X-Llama-Wrangler-Subscriber-Token: "+subscriberCredentialPlaceholder),
					shellSingleQuote(string(heartbeatBody)),
				),
			},
			UninstallCommands: []string{
				"launchctl bootout gui/$(id -u)/" + shellSingleQuote(label),
				"# Remove " + plistCommandPath + " after confirming the service is stopped.",
			},
			Notes: []string{
				"Run these commands on the subscriber host as the service user.",
				"The plist template embeds the environment variable value placeholder; substitute the one-time credential locally before loading launchd.",
				"Use the env file template for shell/systemd-style wrappers or local operator notes; launchd uses EnvironmentVariables in the plist template.",
			},
		},
		SubscriberRestartNotes: []string{
			"Set the environment variable for the subscriber service user before restart.",
			"Restart the subscriber after installing the new credential.",
			"Send one heartbeat with the new credential; the marshal clears re-provisioning-required metadata only after a successful authenticated heartbeat.",
		},
		Warnings: []string{
			"The raw credential is returned only in the rotation response. It is not stored in app state, bootstrap, telemetry, or support bundles.",
			"Do not paste the credential into support bundles, tickets, screenshots, or shared logs.",
			"Install commands are manual operator guidance for the subscriber host; the marshal does not remotely mutate subscriber machines.",
			"Passive Endpoints do not use subscriber heartbeat credentials.",
		},
	}
}

func launchdSafeLabelSuffix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "node"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "node"
	}
	return out
}

func shellSingleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellDoubleQuote(value string) string {
	if value == "" {
		return `""`
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
