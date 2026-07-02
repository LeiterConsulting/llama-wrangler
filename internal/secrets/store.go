package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	backendEncryptedFile        = "encrypted_file"
	backendOSKeychain           = "os_keychain"
	currentFileVersion          = 1
	envSecretBackend            = "LLAMA_WRANGLER_SECRET_BACKEND"
	envKeychainService          = "LLAMA_WRANGLER_KEYCHAIN_SERVICE"
	osKeychainPlanPath          = "docs/14_os_keychain_plan.md"
	osKeychainStatusActive      = "active"
	osKeychainStatusDisabled    = "disabled"
	osKeychainStatusPlanned     = "planned"
	osKeychainStatusUnavailable = "unavailable"
	keychainService             = "llama-wrangler"
	keychainRuntimeInteractive  = "interactive_user"
	keychainRuntimeServiceLike  = "service_like"
)

var ErrRekeyUnsupported = errors.New("local encrypted fallback rekey unsupported")
var errSecretNotFound = errors.New("secret not found")
var newOSKeychainBackend = func() secretBackend {
	return keyringBackend{service: keychainServiceName()}
}
var detectOSKeychainRuntime = defaultOSKeychainRuntimeContext

type secretBackend interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type keyringBackend struct {
	service string
}

func (b keyringBackend) Get(key string) (string, error) {
	return keyring.Get(b.service, key)
}

func (b keyringBackend) Set(key, value string) error {
	return keyring.Set(b.service, key, value)
}

func (b keyringBackend) Delete(key string) error {
	return keyring.Delete(b.service, key)
}

type Store struct {
	mu             sync.RWMutex
	path           string
	legacyPath     string
	keyPath        string
	keySource      string
	data           map[string]string
	key            []byte
	legacyMigrated bool
	activeBackend  string
	osBackend      secretBackend
	osStatus       string
	osMigrated     int
	osRuntime      osKeychainRuntimeContext
}

type Status struct {
	Backend             string   `json:"backend"`
	Encrypted           bool     `json:"encrypted"`
	LegacyMigrated      bool     `json:"legacy_migrated"`
	KeySource           string   `json:"key_source"`
	FallbackBackend     string   `json:"fallback_backend,omitempty"`
	FallbackAvailable   bool     `json:"fallback_available"`
	RekeySupported      bool     `json:"rekey_supported"`
	RekeyDescription    string   `json:"rekey_description,omitempty"`
	BackupRequiredFiles []string `json:"backup_required_files,omitempty"`
	BackupDescription   string   `json:"backup_description,omitempty"`
	RestoreDescription  string   `json:"restore_description,omitempty"`
	BackupWarnings      []string `json:"backup_warnings,omitempty"`
	OSKeychainStatus    string   `json:"os_keychain_status,omitempty"`
	OSKeychainPlan      string   `json:"os_keychain_plan,omitempty"`
	OSKeychainNextStep  string   `json:"os_keychain_next_step,omitempty"`
	OSKeychainMigrated  int      `json:"os_keychain_migrated,omitempty"`
	OSKeychainPlatform  string   `json:"os_keychain_platform,omitempty"`
	OSKeychainRuntime   string   `json:"os_keychain_runtime,omitempty"`
	OSKeychainService   bool     `json:"os_keychain_service_mode"`
	OSKeychainWarning   string   `json:"os_keychain_warning,omitempty"`
}

type RekeyResult struct {
	Status    Status    `json:"status"`
	Rekeyed   bool      `json:"rekeyed"`
	RotatedAt time.Time `json:"rotated_at"`
}

type encryptedFile struct {
	Version    int       `json:"version"`
	Backend    string    `json:"backend"`
	Nonce      string    `json:"nonce"`
	Ciphertext string    `json:"ciphertext"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type osKeychainRuntimeContext struct {
	Platform    string
	Runtime     string
	ServiceMode bool
	Warning     string
}

func Open(dir string) (*Store, error) {
	key, keySource, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, err
	}
	store := &Store{
		path:          filepath.Join(dir, "secrets.enc.json"),
		legacyPath:    filepath.Join(dir, "secrets.json"),
		keyPath:       filepath.Join(dir, "secrets.key"),
		keySource:     keySource,
		data:          map[string]string{},
		key:           key,
		activeBackend: backendEncryptedFile,
		osStatus:      osKeychainStatusDisabled,
		osRuntime:     detectOSKeychainRuntime(),
	}
	if osKeychainRequested() {
		store.osBackend = newOSKeychainBackend()
		store.osStatus = osKeychainStatusActive
		store.activeBackend = backendOSKeychain
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	if store.osBackend != nil {
		store.migrateFallbackToOSKeychain()
	}
	return store, nil
}

func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Store) Get(key string) string {
	s.mu.RLock()
	backend := s.osBackend
	active := s.activeBackend
	fallback := s.data[key]
	s.mu.RUnlock()
	if backend != nil && active == backendOSKeychain {
		value, err := backend.Get(key)
		if err == nil {
			return value
		}
		if !isSecretNotFound(err) {
			s.markOSKeychainUnavailable()
		}
	}
	return fallback
}

func (s *Store) Set(key, value string) error {
	if value == "" || value == "********" {
		return nil
	}
	s.mu.Lock()
	s.data[key] = value
	err := s.saveLocked()
	backend := s.osBackend
	active := s.activeBackend
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if backend != nil && active == backendOSKeychain {
		if err := backend.Set(key, value); err != nil {
			s.markOSKeychainUnavailable()
		}
	}
	return nil
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	delete(s.data, key)
	err := s.saveLocked()
	backend := s.osBackend
	active := s.activeBackend
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if backend != nil && active == backendOSKeychain {
		if err := backend.Delete(key); err != nil && !isSecretNotFound(err) {
			s.markOSKeychainUnavailable()
		}
	}
	return nil
}

func (s *Store) Match(key, candidate string) bool {
	if candidate == "" {
		return false
	}
	value := s.Get(key)
	if value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(candidate)) == 1
}

func (s *Store) Rekey() (RekeyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.statusLocked()
	if s.keySource != "file" {
		return RekeyResult{Status: status}, fmt.Errorf("%w when key_source is %q", ErrRekeyUnsupported, s.keySource)
	}

	oldKey := append([]byte(nil), s.key...)
	oldKeyRaw, keyErr := os.ReadFile(s.keyPath)
	if keyErr != nil {
		return RekeyResult{Status: status}, keyErr
	}
	oldFileRaw, fileErr := os.ReadFile(s.path)
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return RekeyResult{Status: status}, fileErr
	}

	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return RekeyResult{Status: status}, err
	}
	rotatedAt := time.Now().UTC()
	encryptedRaw, err := s.marshalEncryptedFileLocked(newKey, rotatedAt)
	if err != nil {
		return RekeyResult{Status: status}, err
	}

	keyTmp := s.keyPath + ".tmp"
	fileTmp := s.path + ".tmp"
	if err := os.WriteFile(keyTmp, []byte(base64.RawStdEncoding.EncodeToString(newKey)), 0o600); err != nil {
		return RekeyResult{Status: status}, err
	}
	if err := os.WriteFile(fileTmp, encryptedRaw, 0o600); err != nil {
		_ = os.Remove(keyTmp)
		return RekeyResult{Status: status}, err
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		_ = os.Remove(keyTmp)
		_ = os.Remove(fileTmp)
		_ = os.WriteFile(s.keyPath, oldKeyRaw, 0o600)
		if fileErr == nil {
			_ = os.WriteFile(s.path, oldFileRaw, 0o600)
		}
		s.key = oldKey
	}()
	if err := os.Rename(keyTmp, s.keyPath); err != nil {
		return RekeyResult{Status: status}, err
	}
	if err := os.Rename(fileTmp, s.path); err != nil {
		return RekeyResult{Status: status}, err
	}
	s.key = newKey
	committed = true
	return RekeyResult{Status: s.statusLocked(), Rekeyed: true, RotatedAt: rotatedAt}, nil
}

func (s *Store) load() error {
	if raw, err := os.ReadFile(s.path); err == nil {
		return s.decrypt(raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	raw, err := os.ReadFile(s.legacyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	if s.data == nil {
		s.data = map[string]string{}
	}
	if err := s.saveLocked(); err != nil {
		return err
	}
	if err := os.Remove(s.legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.legacyMigrated = true
	return nil
}

func (s *Store) migrateFallbackToOSKeychain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.osBackend == nil || s.activeBackend != backendOSKeychain {
		return
	}
	for key, value := range s.data {
		if value == "" {
			continue
		}
		if existing, err := s.osBackend.Get(key); err == nil && existing != "" {
			continue
		} else if err != nil && !isSecretNotFound(err) {
			s.markOSKeychainUnavailableLocked()
			return
		}
		if err := s.osBackend.Set(key, value); err != nil {
			s.markOSKeychainUnavailableLocked()
			return
		}
		s.osMigrated++
	}
}

func (s *Store) decrypt(raw []byte) error {
	var file encryptedFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	if file.Version != currentFileVersion {
		return fmt.Errorf("unsupported encrypted secrets version %d", file.Version)
	}
	nonce, err := base64.RawStdEncoding.DecodeString(file.Nonce)
	if err != nil {
		return err
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(file.Ciphertext)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(file.Backend))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(plaintext, &s.data); err != nil {
		return err
	}
	if s.data == nil {
		s.data = map[string]string{}
	}
	return nil
}

func (s *Store) saveLocked() error {
	raw, err := s.marshalEncryptedFileLocked(s.key, time.Now().UTC())
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) marshalEncryptedFileLocked(key []byte, updatedAt time.Time) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	plaintext, err := json.Marshal(s.data)
	if err != nil {
		return nil, err
	}
	backend := backendEncryptedFile
	file := encryptedFile{
		Version:    currentFileVersion,
		Backend:    backend,
		Nonce:      base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(gcm.Seal(nil, nonce, plaintext, []byte(backend))),
		UpdatedAt:  updatedAt,
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Store) statusLocked() Status {
	rekeySupported := s.keySource == "file"
	description := "Local encrypted fallback key can be rotated."
	requiredFiles := []string{"secrets.enc.json", "secrets.key"}
	backupDescription := "Back up secrets.enc.json and secrets.key together from the Llama Wrangler app-data directory."
	restoreDescription := "Restore both files together before starting Llama Wrangler on the target machine."
	warnings := []string{
		"secrets.enc.json cannot be decrypted without its matching key source.",
		"Support bundles are diagnostic exports and are not secret backups.",
	}
	if !rekeySupported {
		description = "Local rekey is unavailable because the secrets key is supplied externally."
		requiredFiles = []string{"secrets.enc.json", "LLAMA_WRANGLER_SECRETS_KEY"}
		backupDescription = "Back up secrets.enc.json and preserve the external LLAMA_WRANGLER_SECRETS_KEY value separately."
		restoreDescription = "Restore secrets.enc.json and provide the same LLAMA_WRANGLER_SECRETS_KEY before starting Llama Wrangler."
	}
	nextStep := "Set LLAMA_WRANGLER_SECRET_BACKEND=os_keychain to opt in to the OS keychain backend; encrypted fallback remains available."
	switch s.osStatus {
	case osKeychainStatusActive:
		nextStep = "Using OS keychain as the active secret backend with encrypted fallback retained."
		if s.osRuntime.ServiceMode {
			nextStep = "OS keychain is active in a service-like runtime; verify the service user and session can access the same keychain items. Encrypted fallback remains available."
		}
	case osKeychainStatusUnavailable:
		nextStep = "Encrypted fallback is active because the OS keychain backend was unavailable."
	case osKeychainStatusPlanned:
		nextStep = "Add a minimal optional OS keychain backend while keeping encrypted fallback available."
	}
	return Status{
		Backend:             s.activeBackend,
		Encrypted:           true,
		LegacyMigrated:      s.legacyMigrated,
		KeySource:           s.keySource,
		FallbackBackend:     backendEncryptedFile,
		FallbackAvailable:   true,
		RekeySupported:      rekeySupported,
		RekeyDescription:    description,
		BackupRequiredFiles: requiredFiles,
		BackupDescription:   backupDescription,
		RestoreDescription:  restoreDescription,
		BackupWarnings:      warnings,
		OSKeychainStatus:    s.osStatus,
		OSKeychainPlan:      osKeychainPlanPath,
		OSKeychainNextStep:  nextStep,
		OSKeychainMigrated:  s.osMigrated,
		OSKeychainPlatform:  s.osRuntime.Platform,
		OSKeychainRuntime:   s.osRuntime.Runtime,
		OSKeychainService:   s.osRuntime.ServiceMode,
		OSKeychainWarning:   s.osRuntime.Warning,
	}
}

func (s *Store) markOSKeychainUnavailable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markOSKeychainUnavailableLocked()
}

func (s *Store) markOSKeychainUnavailableLocked() {
	s.activeBackend = backendEncryptedFile
	s.osStatus = osKeychainStatusUnavailable
}

func osKeychainRequested() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(envSecretBackend)))
	return value == backendOSKeychain || value == "keychain"
}

func keychainServiceName() string {
	if service := strings.TrimSpace(os.Getenv(envKeychainService)); service != "" {
		return service
	}
	return keychainService
}

func isSecretNotFound(err error) bool {
	return errors.Is(err, errSecretNotFound) || errors.Is(err, keyring.ErrNotFound)
}

func defaultOSKeychainRuntimeContext() osKeychainRuntimeContext {
	serviceMode := truthyEnv("LLAMA_WRANGLER_SERVICE_MODE") ||
		os.Getenv("INVOCATION_ID") != "" ||
		os.Getenv("NOTIFY_SOCKET") != "" ||
		os.Getenv("LAUNCHD_JOB") != "" ||
		strings.EqualFold(os.Getenv("LAUNCHD_SESSION_TYPE"), "system")
	mode := keychainRuntimeInteractive
	warning := ""
	if serviceMode {
		mode = keychainRuntimeServiceLike
		warning = "Service-like runtimes can use a different OS user or keychain session; verify keychain access before relying on OS keychain storage."
	}
	return osKeychainRuntimeContext{
		Platform:    runtime.GOOS,
		Runtime:     mode,
		ServiceMode: serviceMode,
		Warning:     warning,
	}
}

func truthyEnv(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func loadOrCreateKey(dir string) ([]byte, string, error) {
	if raw := os.Getenv("LLAMA_WRANGLER_SECRETS_KEY"); raw != "" {
		key, err := decodeKey(raw)
		return key, "env", err
	}
	keyPath := filepath.Join(dir, "secrets.key")
	raw, err := os.ReadFile(keyPath)
	if err == nil {
		key, err := decodeKey(string(raw))
		return key, "file", err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(keyPath, []byte(base64.RawStdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, "", err
	}
	return key, "file", nil
}

func decodeKey(raw string) ([]byte, error) {
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secrets key must decode to 32 bytes")
	}
	return key, nil
}
