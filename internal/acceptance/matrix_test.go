package acceptance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestV1AcceptanceMatrixAndHarnessStaySynchronizedAndSafe(t *testing.T) {
	root := filepath.Join("..", "..")
	matrix := readAcceptanceFile(t, filepath.Join(root, "docs", "21_v1_acceptance_security_matrix.md"))
	harnessPath := filepath.Join(root, "scripts", "v1_acceptance.sh")
	harness := readAcceptanceFile(t, harnessPath)

	for _, id := range []string{
		"V1-LIFE-001",
		"V1-LIFE-002",
		"V1-SAFE-001",
		"V1-STATE-001",
		"V1-AUTH-001",
		"V1-AUTH-002",
		"V1-SEC-001",
		"V1-NODE-001",
		"V1-NODE-002",
		"V1-PASS-001",
		"V1-ROUTE-001",
		"V1-CONS-001",
		"V1-CONS-002",
		"V1-BENCH-001",
		"V1-MODEL-001",
		"V1-SPLUNK-001",
		"V1-SUPPORT-001",
		"V1-UI-001",
		"V1-V2-001",
		"V1-PKG-MAC-DRY-001",
		"V1-PKG-MAC-001",
		"V1-PKG-MAC-002",
		"V1-PKG-MAC-003",
		"V1-PKG-LINUX-001",
		"V1-PKG-WIN-001",
		"V1-SPLUNK-RUNTIME-001",
		"V1-SIGN-001",
		"V1-CLIENT-001",
	} {
		if !strings.Contains(matrix, id) {
			t.Fatalf("acceptance matrix missing %s", id)
		}
	}

	for _, required := range []string{
		"set -euo pipefail",
		"mktemp -d",
		"trap cleanup",
		"127.0.0.1",
		"LLAMA_WRANGLER_SECRET_BACKEND=\"encrypted_file\"",
		"go test -count=1 ./...",
		"go build -o",
		"setup/complete",
		"wrangler/enrollment-tokens",
		"subscriber/enroll",
		"support-bundle/export",
		"secrets.enc.json",
		"secrets.key",
		"xmllint --noout",
	} {
		if !strings.Contains(harness, required) {
			t.Fatalf("acceptance harness missing safety/evidence hook %q", required)
		}
	}

	lowerHarness := strings.ToLower(harness)
	for _, forbidden := range []string{
		"launchctl load",
		"launchctl bootstrap",
		"systemctl enable",
		"systemctl start",
		"sc.exe create",
		"sudo ",
		"/library/launchdaemons",
		"security add-generic-password",
	} {
		if strings.Contains(lowerHarness, forbidden) {
			t.Fatalf("acceptance harness contains forbidden OS mutation command %q", forbidden)
		}
	}
	if strings.Contains(harness, "LLAMA_WRANGLER_ACCEPTANCE_KEEP_TEMP") {
		t.Fatalf("acceptance harness must not offer to retain disposable credential state")
	}

	info, err := os.Stat(harnessPath)
	if err != nil {
		t.Fatalf("stat acceptance harness: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("acceptance harness is not executable")
	}
}

func TestV1CapabilityBoundaryRemainsDocumentationOnly(t *testing.T) {
	futurePlan := readAcceptanceFile(t, filepath.Join("..", "..", "docs", "19_capability_endpoint_future_plan.md"))
	for _, required := range []string{
		"V1 remains Ollama-first.",
		"After V1 is functional",
		"No runtime implementation changes",
		"documentation",
		"no non-Ollama integrations are implemented prematurely",
	} {
		if !strings.Contains(futurePlan, required) {
			t.Fatalf("future capability plan missing boundary marker %q", required)
		}
	}
}

func TestV1AcceptanceMatrixDoesNotPrematurelyPassExternalRows(t *testing.T) {
	matrix := readAcceptanceFile(t, filepath.Join("..", "..", "docs", "21_v1_acceptance_security_matrix.md"))
	for _, id := range []string{"V1-PKG-MAC-002", "V1-PKG-MAC-003", "V1-PKG-LINUX-001", "V1-PKG-WIN-001", "V1-SPLUNK-RUNTIME-001", "V1-SIGN-001", "V1-CLIENT-001"} {
		for _, line := range strings.Split(matrix, "\n") {
			if strings.Contains(line, id) && !strings.Contains(line, "Pending") {
				t.Fatalf("external acceptance row %s is not explicitly pending: %s", id, line)
			}
		}
	}
}

func TestMacOSDisposableLifecycleRowHasRecordedEvidence(t *testing.T) {
	matrix := readAcceptanceFile(t, filepath.Join("..", "..", "docs", "21_v1_acceptance_security_matrix.md"))
	for _, line := range strings.Split(matrix, "\n") {
		if strings.Contains(line, "V1-PKG-MAC-001") {
			if !strings.Contains(line, "Validated 2026-07-10") || !strings.Contains(line, "M01-M08") {
				t.Fatalf("macOS disposable lifecycle row lacks recorded evidence: %s", line)
			}
			return
		}
	}
	t.Fatal("macOS disposable lifecycle row missing")
}

func readAcceptanceFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
