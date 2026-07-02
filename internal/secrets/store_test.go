package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedStoreDoesNotPersistPlaintext(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Set("admin_token", "super-secret-token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if got := store.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("Get() = %q", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "secrets.enc.json"))
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-token") {
		t.Fatalf("encrypted file contains plaintext secret")
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext file should not exist")
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	if got := reopened.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("reopened Get() = %q", got)
	}
}

func TestLegacyPlaintextSecretsAreMigratedAndRemoved(t *testing.T) {
	dir := t.TempDir()
	legacy := map[string]string{"splunk_hec_token": "legacy-token"}
	raw, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "secrets.json"), raw, 0o600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got := store.Get("splunk_hec_token"); got != "legacy-token" {
		t.Fatalf("migrated token = %q", got)
	}
	status := store.Status()
	if !status.Encrypted || !status.LegacyMigrated {
		t.Fatalf("status = %+v, want encrypted migrated store", status)
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext file should be removed after migration")
	}
	encryptedRaw, err := os.ReadFile(filepath.Join(dir, "secrets.enc.json"))
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	if strings.Contains(string(encryptedRaw), "legacy-token") {
		t.Fatalf("encrypted migrated file contains plaintext secret")
	}
}

func TestDeleteAndMatch(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Set("api_key:test", "client-token"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !store.Match("api_key:test", "client-token") {
		t.Fatalf("Match() should accept stored token")
	}
	if err := store.Delete("api_key:test"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.Match("api_key:test", "client-token") {
		t.Fatalf("Match() should reject deleted token")
	}
}

func TestEnvSecretKey(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("LLAMA_WRANGLER_SECRETS_KEY", base64.RawStdEncoding.EncodeToString(key))
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if status := store.Status(); status.KeySource != "env" {
		t.Fatalf("key source = %q, want env", status.KeySource)
	}
	if status := store.Status(); status.RekeySupported {
		t.Fatalf("env key source should not support local rekey: %+v", status)
	}
	status := store.Status()
	if !containsString(status.BackupRequiredFiles, "secrets.enc.json") || !containsString(status.BackupRequiredFiles, "LLAMA_WRANGLER_SECRETS_KEY") {
		t.Fatalf("env backup required files = %+v", status.BackupRequiredFiles)
	}
	if strings.Contains(status.BackupDescription, base64.RawStdEncoding.EncodeToString(key)) || strings.Contains(status.RestoreDescription, base64.RawStdEncoding.EncodeToString(key)) {
		t.Fatalf("env backup guidance leaked key value: %+v", status)
	}
}

func TestSecretStatusIncludesBackupGuidance(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	status := store.Status()
	if status.KeySource != "file" || !status.RekeySupported {
		t.Fatalf("status = %+v, want file key with local rekey", status)
	}
	for _, required := range []string{"secrets.enc.json", "secrets.key"} {
		if !containsString(status.BackupRequiredFiles, required) {
			t.Fatalf("backup_required_files missing %q: %+v", required, status.BackupRequiredFiles)
		}
	}
	if !strings.Contains(status.BackupDescription, "Back up secrets.enc.json and secrets.key together") {
		t.Fatalf("backup description = %q", status.BackupDescription)
	}
	if !strings.Contains(status.RestoreDescription, "Restore both files together") {
		t.Fatalf("restore description = %q", status.RestoreDescription)
	}
	if len(status.BackupWarnings) == 0 || !strings.Contains(strings.Join(status.BackupWarnings, " "), "Support bundles are diagnostic exports") {
		t.Fatalf("backup warnings = %+v", status.BackupWarnings)
	}
	if status.OSKeychainStatus != "disabled" || status.OSKeychainPlan != "docs/14_os_keychain_plan.md" {
		t.Fatalf("OS keychain plan metadata = %+v", status)
	}
	if !strings.Contains(status.OSKeychainNextStep, "LLAMA_WRANGLER_SECRET_BACKEND=os_keychain") {
		t.Fatalf("OS keychain next step = %q", status.OSKeychainNextStep)
	}
	if status.OSKeychainPlatform == "" || status.OSKeychainRuntime == "" {
		t.Fatalf("OS keychain runtime metadata = %+v", status)
	}
}

func TestOSKeychainBackendMigratesFallbackAndKeepsFallback(t *testing.T) {
	dir := t.TempDir()
	fallback, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(fallback) error = %v", err)
	}
	if err := fallback.Set("admin_token", "super-secret-token"); err != nil {
		t.Fatalf("Set(fallback) error = %v", err)
	}

	fake := newFakeBackend()
	withFakeOSKeychainBackend(t, fake)
	t.Setenv("LLAMA_WRANGLER_SECRET_BACKEND", "os_keychain")
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(keychain) error = %v", err)
	}
	status := store.Status()
	if status.Backend != "os_keychain" || status.OSKeychainStatus != "active" || !status.FallbackAvailable || status.FallbackBackend != "encrypted_file" {
		t.Fatalf("status = %+v", status)
	}
	if status.OSKeychainMigrated != 1 {
		t.Fatalf("OSKeychainMigrated = %d, want 1", status.OSKeychainMigrated)
	}
	if got := fake.values["admin_token"]; got != "super-secret-token" {
		t.Fatalf("fake migrated admin token = %q", got)
	}
	if got := store.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("Get(admin_token) = %q", got)
	}

	if err := store.Set("api_key:local", "client-secret-token"); err != nil {
		t.Fatalf("Set(client) error = %v", err)
	}
	if got := fake.values["api_key:local"]; got != "client-secret-token" {
		t.Fatalf("fake client token = %q", got)
	}
	encryptedRaw, err := os.ReadFile(filepath.Join(dir, "secrets.enc.json"))
	if err != nil {
		t.Fatalf("read encrypted fallback: %v", err)
	}
	if strings.Contains(string(encryptedRaw), "client-secret-token") {
		t.Fatalf("encrypted fallback leaked plaintext client token")
	}

	if err := store.Delete("api_key:local"); err != nil {
		t.Fatalf("Delete(client) error = %v", err)
	}
	if _, ok := fake.values["api_key:local"]; ok {
		t.Fatalf("fake backend retained deleted client token")
	}

	t.Setenv("LLAMA_WRANGLER_SECRET_BACKEND", "")
	reopenedFallback, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(reopened fallback) error = %v", err)
	}
	if got := reopenedFallback.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("fallback admin token after keychain spike = %q", got)
	}
}

func TestOSKeychainUnavailableFallsBackToEncryptedFile(t *testing.T) {
	dir := t.TempDir()
	fallback, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(fallback) error = %v", err)
	}
	if err := fallback.Set("admin_token", "super-secret-token"); err != nil {
		t.Fatalf("Set(fallback) error = %v", err)
	}

	withFakeOSKeychainBackend(t, &fakeBackend{err: errors.New("keychain unavailable"), values: map[string]string{}})
	t.Setenv("LLAMA_WRANGLER_SECRET_BACKEND", "os_keychain")
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(keychain unavailable) error = %v", err)
	}
	status := store.Status()
	if status.Backend != "encrypted_file" || status.OSKeychainStatus != "unavailable" || !status.FallbackAvailable {
		t.Fatalf("status = %+v", status)
	}
	if got := store.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("fallback Get(admin_token) = %q", got)
	}
}

func TestOSKeychainServiceLikeRuntimeWarnsAndKeepsFallback(t *testing.T) {
	withFakeOSKeychainBackend(t, newFakeBackend())
	withFakeOSKeychainRuntime(t, osKeychainRuntimeContext{
		Platform:    "darwin",
		Runtime:     keychainRuntimeServiceLike,
		ServiceMode: true,
		Warning:     "service warning",
	})
	t.Setenv("LLAMA_WRANGLER_SECRET_BACKEND", "os_keychain")
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	status := store.Status()
	if status.Backend != "os_keychain" || status.OSKeychainStatus != "active" || !status.FallbackAvailable {
		t.Fatalf("status = %+v", status)
	}
	if !status.OSKeychainService || status.OSKeychainRuntime != keychainRuntimeServiceLike || status.OSKeychainWarning == "" {
		t.Fatalf("service runtime metadata = %+v", status)
	}
	if !strings.Contains(status.OSKeychainNextStep, "service-like runtime") {
		t.Fatalf("service next step = %q", status.OSKeychainNextStep)
	}
}

func TestRekeyRotatesFileKeyAndPreservesSecrets(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Set("admin_token", "super-secret-token"); err != nil {
		t.Fatalf("Set(admin) error = %v", err)
	}
	if err := store.Set("api_key:local", "client-secret-token"); err != nil {
		t.Fatalf("Set(client) error = %v", err)
	}
	keyPath := filepath.Join(dir, "secrets.key")
	filePath := filepath.Join(dir, "secrets.enc.json")
	oldKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read old key: %v", err)
	}
	oldEncrypted, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read old encrypted file: %v", err)
	}

	result, err := store.Rekey()
	if err != nil {
		t.Fatalf("Rekey() error = %v", err)
	}
	if !result.Rekeyed || !result.Status.RekeySupported || result.Status.KeySource != "file" {
		t.Fatalf("rekey result = %+v", result)
	}
	newKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read new key: %v", err)
	}
	newEncrypted, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read new encrypted file: %v", err)
	}
	if string(oldKey) == string(newKey) {
		t.Fatalf("secrets key file did not change")
	}
	if string(oldEncrypted) == string(newEncrypted) {
		t.Fatalf("encrypted secrets file did not change")
	}
	if strings.Contains(string(newEncrypted), "super-secret-token") || strings.Contains(string(newEncrypted), "client-secret-token") {
		t.Fatalf("encrypted file contains plaintext after rekey")
	}
	if got := store.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("in-memory admin secret after rekey = %q", got)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen after rekey: %v", err)
	}
	if got := reopened.Get("admin_token"); got != "super-secret-token" {
		t.Fatalf("reopened admin secret = %q", got)
	}
	if got := reopened.Get("api_key:local"); got != "client-secret-token" {
		t.Fatalf("reopened client secret = %q", got)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestRekeyRejectsEnvKeySource(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("LLAMA_WRANGLER_SECRETS_KEY", base64.RawStdEncoding.EncodeToString(key))
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := store.Rekey(); err == nil {
		t.Fatalf("Rekey() expected env key source error")
	}
}

func TestOSKeychainBackendPlatformOptIn(t *testing.T) {
	if os.Getenv("LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS") != "1" {
		t.Skip("set LLAMA_WRANGLER_RUN_KEYCHAIN_TESTS=1 to exercise the real OS keychain backend")
	}
	t.Setenv("LLAMA_WRANGLER_SECRET_BACKEND", "os_keychain")
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	key := "test_key:" + strings.ReplaceAll(t.Name(), "/", "_")
	value := "test-secret-value"
	if err := store.Set(key, value); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	status := store.Status()
	if status.Backend != "os_keychain" || status.OSKeychainStatus != "active" || !status.FallbackAvailable || status.OSKeychainPlatform == "" || status.OSKeychainRuntime == "" {
		t.Fatalf("platform keychain status = %+v", status)
	}
	if got := store.Get(key); got != value {
		t.Fatalf("Get() = %q", got)
	}
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestKeychainServiceNameCanBeOverriddenForDisposableChecks(t *testing.T) {
	if got := keychainServiceName(); got != keychainService {
		t.Fatalf("default keychain service = %q", got)
	}
	t.Setenv("LLAMA_WRANGLER_KEYCHAIN_SERVICE", "llama-wrangler-dryrun-test")
	if got := keychainServiceName(); got != "llama-wrangler-dryrun-test" {
		t.Fatalf("overridden keychain service = %q", got)
	}
}

type fakeBackend struct {
	values map[string]string
	err    error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{values: map[string]string{}}
}

func (b *fakeBackend) Get(key string) (string, error) {
	if b.err != nil {
		return "", b.err
	}
	value, ok := b.values[key]
	if !ok {
		return "", errSecretNotFound
	}
	return value, nil
}

func (b *fakeBackend) Set(key, value string) error {
	if b.err != nil {
		return b.err
	}
	b.values[key] = value
	return nil
}

func (b *fakeBackend) Delete(key string) error {
	if b.err != nil {
		return b.err
	}
	if _, ok := b.values[key]; !ok {
		return errSecretNotFound
	}
	delete(b.values, key)
	return nil
}

func withFakeOSKeychainBackend(t *testing.T, backend secretBackend) {
	t.Helper()
	previous := newOSKeychainBackend
	newOSKeychainBackend = func() secretBackend {
		return backend
	}
	t.Cleanup(func() {
		newOSKeychainBackend = previous
	})
}

func withFakeOSKeychainRuntime(t *testing.T, context osKeychainRuntimeContext) {
	t.Helper()
	previous := detectOSKeychainRuntime
	detectOSKeychainRuntime = func() osKeychainRuntimeContext {
		return context
	}
	t.Cleanup(func() {
		detectOSKeychainRuntime = previous
	})
}
