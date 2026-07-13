package acceptance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMacOSLaunchdAcceptanceIsDryRunFirstAndDisposable(t *testing.T) {
	root := filepath.Join("..", "..")
	scriptPath := filepath.Join(root, "scripts", "macos_user_launchd_acceptance.sh")
	script := readAcceptanceFile(t, scriptPath)

	for _, required := range []string{
		"set -euo pipefail",
		"LLAMA_WRANGLER_MACOS_LAUNCHD_ACCEPTANCE",
		"mktemp -d",
		"trap cleanup",
		"gui/$(id -u)",
		"LLAMA_WRANGLER_SECRET_BACKEND=encrypted_file",
		"--launch-agents-dir",
		"launchctl bootstrap",
		"launchctl kickstart -k",
		"launchctl bootout",
		"acceptance-initial",
		"acceptance-upgrade",
		"service log exposed a credential",
		"Signed package-candidate, notarization, and packaged keychain acceptance remain pending.",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("macOS packaging harness missing %q", required)
		}
	}

	guardIndex := strings.Index(script, `if [[ "${OPT_IN}" != "1" ]]`)
	bootstrapIndex := strings.Index(script, `launchctl bootstrap "${SERVICE_DOMAIN}"`)
	if guardIndex < 0 || bootstrapIndex < 0 || guardIndex >= bootstrapIndex {
		t.Fatalf("launchctl bootstrap is not behind the explicit opt-in guard")
	}

	lower := strings.ToLower(script)
	for _, forbidden := range []string{
		"sudo ",
		"/library/launchdaemons",
		"systemctl ",
		"sc.exe ",
		"security add-generic-password",
		"llama_wrangler_secret_backend=os_keychain",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("macOS packaging harness contains forbidden mutation %q", forbidden)
		}
	}

	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("stat macOS packaging harness: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("macOS packaging harness is not executable")
	}
}

func TestMacOSReleaseCandidateAndKeychainRowsRemainPending(t *testing.T) {
	matrix := readAcceptanceFile(t, filepath.Join("..", "..", "docs", "21_v1_acceptance_security_matrix.md"))
	for _, id := range []string{"V1-PKG-MAC-002", "V1-PKG-MAC-003", "V1-SIGN-001"} {
		for _, line := range strings.Split(matrix, "\n") {
			if strings.Contains(line, id) && !strings.Contains(line, "Pending") {
				t.Fatalf("macOS external row %s is not explicitly pending: %s", id, line)
			}
		}
	}
}
