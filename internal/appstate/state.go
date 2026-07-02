package appstate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"llama-wrangler/internal/config"
)

const CurrentSchemaVersion = 2

const (
	ControlLevelManaged = "managed"
	ControlLevelPassive = "passive"

	TrustLevelLocal         = "local"
	TrustLevelLANTrusted    = "lan_trusted"
	TrustLevelLANUnverified = "lan_unverified"
	TrustLevelExternal      = "external"

	CapabilitySourceSubscriberReported = "subscriber_reported"
	CapabilitySourceMarshalObserved    = "marshal_observed"
	CapabilitySourceManual             = "manual"

	ApprovalStatePending  = "pending"
	ApprovalStateApproved = "approved"
	ApprovalStateRejected = "rejected"
	ApprovalStateRevoked  = "revoked"

	HealthSourceSubscriberReported = "subscriber_reported"
	HealthSourceMarshalObserved    = "marshal_observed"

	ModelInventorySourceSubscriberReported = "subscriber_reported"
	ModelInventorySourceMarshalObserved    = "marshal_observed"
	ModelInventorySourceManual             = "manual"

	BenchmarkSourceSubscriberReported = "subscriber_reported"
	BenchmarkSourceMarshalObserved    = "marshal_observed"
	BenchmarkSourceNone               = "none"

	TelemetryLevelRichMetadata            = "rich_metadata"
	TelemetryLevelMarshalObservedMetadata = "marshal_observed_metadata"
)

type Store struct {
	mu    sync.RWMutex
	dir   string
	path  string
	state State
}

type State struct {
	SchemaVersion    int                 `json:"schema_version"`
	ConfigVersion    int                 `json:"config_version"`
	SetupComplete    bool                `json:"setup_complete"`
	AdminTokenHint   string              `json:"admin_token_hint,omitempty"`
	NodeID           string              `json:"node_id"`
	Role             string              `json:"role"`
	Config           config.Config       `json:"config"`
	Nodes            map[string]Node     `json:"nodes"`
	Sessions         map[string]Session  `json:"sessions"`
	ClientAPIKeys    []ClientAPIKey      `json:"client_api_keys"`
	Audit            []AuditEvent        `json:"audit"`
	EnrollmentQueue  []EnrollmentRequest `json:"enrollment_queue"`
	MigrationHistory []MigrationRecord   `json:"migration_history"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

type MigrationRecord struct {
	FromVersion int       `json:"from_version"`
	ToVersion   int       `json:"to_version"`
	AppliedAt   time.Time `json:"applied_at"`
	Description string    `json:"description"`
}

type Node struct {
	NodeID               string                 `json:"node_id"`
	DisplayName          string                 `json:"display_name"`
	URL                  string                 `json:"url"`
	Role                 string                 `json:"role"`
	ControlLevel         string                 `json:"control_level"`
	TrustLevel           string                 `json:"trust_level"`
	CapabilitySource     string                 `json:"capability_source"`
	ApprovalState        string                 `json:"approval_state"`
	HealthSource         string                 `json:"health_source"`
	ModelInventorySource string                 `json:"model_inventory_source"`
	BenchmarkSource      string                 `json:"benchmark_source"`
	WarmStateSupported   bool                   `json:"warm_state_supported"`
	ManagementSupported  bool                   `json:"management_supported"`
	TelemetryLevel       string                 `json:"telemetry_level"`
	Hostname             string                 `json:"hostname"`
	Platform             string                 `json:"platform"`
	Arch                 string                 `json:"arch"`
	CPU                  string                 `json:"cpu,omitempty"`
	GPU                  string                 `json:"gpu,omitempty"`
	MemoryTotalGB        float64                `json:"memory_total_gb,omitempty"`
	MemoryAvailGB        float64                `json:"memory_available_gb,omitempty"`
	OllamaAvailable      bool                   `json:"ollama_available"`
	OllamaURL            string                 `json:"ollama_url"`
	OllamaVersion        string                 `json:"ollama_version,omitempty"`
	Models               []ModelState           `json:"models"`
	Tags                 []string               `json:"tags"`
	Status               string                 `json:"status"`
	Enabled              bool                   `json:"enabled"`
	Approved             bool                   `json:"approved"`
	ActiveJobs           int                    `json:"active_jobs"`
	MaxJobs              int                    `json:"max_jobs"`
	QueueDepth           int                    `json:"queue_depth"`
	Observed             map[string]interface{} `json:"observed,omitempty"`
	LastSeen             time.Time              `json:"last_seen"`
	LastObservedAt       *time.Time             `json:"last_observed_at,omitempty"`
	LastReportedAt       *time.Time             `json:"last_reported_at,omitempty"`
}

type ModelState struct {
	Name       string  `json:"name"`
	State      string  `json:"state"`
	KeepWarm   bool    `json:"keep_warm"`
	TokensSec  float64 `json:"tokens_per_second,omitempty"`
	LoadTimeMS int     `json:"load_time_ms,omitempty"`
}

type Session struct {
	SessionID     string    `json:"session_id"`
	AffinityMode  string    `json:"affinity_mode"`
	NodeID        string    `json:"node_id,omitempty"`
	Model         string    `json:"model,omitempty"`
	LastRequestID string    `json:"last_request_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ClientAPIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Hint      string    `json:"hint"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
	Enabled   bool      `json:"enabled"`
}

type EnrollmentRequest struct {
	NodeID           string    `json:"node_id"`
	URL              string    `json:"url"`
	Hostname         string    `json:"hostname"`
	ControlLevel     string    `json:"control_level,omitempty"`
	TrustLevel       string    `json:"trust_level,omitempty"`
	CapabilitySource string    `json:"capability_source,omitempty"`
	ApprovalState    string    `json:"approval_state,omitempty"`
	TokenHash        string    `json:"token_hash,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type AuditEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	RequestID string                 `json:"request_id,omitempty"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func Open(cfg config.Config) (*Store, error) {
	dir, err := AppDataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	store := &Store{dir: dir, path: filepath.Join(dir, "state.json")}
	if err := store.load(cfg); err != nil {
		return nil, err
	}
	return store, nil
}

func AppDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Llama Wrangler"), nil
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Llama Wrangler"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "Llama Wrangler"), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "llama-wrangler"), nil
		}
		return filepath.Join(home, ".config", "llama-wrangler"), nil
	}
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) load(cfg config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.Normalize(&cfg)

	data, err := os.ReadFile(s.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		nodeID := cfg.Node.NodeID
		if nodeID == "" || nodeID == "local" {
			nodeID = "node_" + randomHex(4)
		}
		s.state = State{
			SchemaVersion:    CurrentSchemaVersion,
			ConfigVersion:    1,
			NodeID:           nodeID,
			Role:             cfg.Server.Mode,
			Config:           cfg,
			Nodes:            map[string]Node{},
			Sessions:         map[string]Session{},
			ClientAPIKeys:    []ClientAPIKey{},
			Audit:            []AuditEvent{},
			EnrollmentQueue:  []EnrollmentRequest{},
			MigrationHistory: []MigrationRecord{},
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		return s.saveLocked()
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return err
	}
	migrated, err := s.migrateLocked()
	if err != nil {
		return err
	}
	s.state.Config = mergeRuntimeConfig(s.state.Config, cfg)
	config.Normalize(&s.state.Config)
	if s.state.ConfigVersion == 0 {
		s.state.ConfigVersion = 1
		migrated = true
	}
	if s.state.Role == "" {
		s.state.Role = s.state.Config.Server.Mode
		migrated = true
	}
	if migrated {
		return s.saveLocked()
	}
	return nil
}

func (s *Store) migrateLocked() (bool, error) {
	if s.state.SchemaVersion > CurrentSchemaVersion {
		return false, fmt.Errorf("state schema version %d is newer than supported version %d", s.state.SchemaVersion, CurrentSchemaVersion)
	}
	migrated := false
	if s.state.SchemaVersion == 0 {
		s.state.Nodes = ensureNodeMap(s.state.Nodes)
		s.state.Sessions = ensureSessionMap(s.state.Sessions)
		s.state.ClientAPIKeys = ensureClientKeys(s.state.ClientAPIKeys)
		s.state.Audit = ensureAudit(s.state.Audit)
		s.state.EnrollmentQueue = ensureEnrollmentQueue(s.state.EnrollmentQueue)
		if s.state.CreatedAt.IsZero() {
			s.state.CreatedAt = time.Now().UTC()
		}
		s.state.SchemaVersion = 1
		s.state.MigrationHistory = append(s.state.MigrationHistory, MigrationRecord{
			FromVersion: 0,
			ToVersion:   1,
			AppliedAt:   time.Now().UTC(),
			Description: "Initialized schema metadata, collection defaults, and migration history.",
		})
		migrated = true
	}
	if s.state.SchemaVersion == 1 {
		s.state.Nodes = normalizeNodeMap(s.state.Nodes)
		s.state.EnrollmentQueue = normalizeEnrollmentQueue(s.state.EnrollmentQueue)
		s.state.SchemaVersion = 2
		s.state.MigrationHistory = append(s.state.MigrationHistory, MigrationRecord{
			FromVersion: 1,
			ToVersion:   2,
			AppliedAt:   time.Now().UTC(),
			Description: "Added Managed Node and Passive Endpoint control, trust, approval, capability source, and freshness metadata.",
		})
		migrated = true
	}
	if s.state.SchemaVersion < CurrentSchemaVersion {
		return migrated, fmt.Errorf("state schema version %d has no migration path to %d", s.state.SchemaVersion, CurrentSchemaVersion)
	}
	return migrated, nil
}

func mergeRuntimeConfig(stored config.Config, runtimeCfg config.Config) config.Config {
	if runtimeCfg.Server.Mode != "" {
		stored.Server.Mode = runtimeCfg.Server.Mode
	}
	if runtimeCfg.Server.Listen != "" {
		stored.Server.Listen = runtimeCfg.Server.Listen
	}
	if runtimeCfg.Ollama.URL != "" {
		stored.Ollama.URL = runtimeCfg.Ollama.URL
	}
	return stored
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.state)
}

func (s *Store) SaveConfig(cfg config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.Normalize(&cfg)
	s.state.Config = cfg
	if s.state.ConfigVersion == 0 {
		s.state.ConfigVersion = 1
	} else {
		s.state.ConfigVersion++
	}
	s.state.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) UpsertNode(node Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if node.NodeID == "" {
		node.NodeID = "node_" + randomHex(4)
	}
	if node.Status == "" {
		node.Status = "unknown"
	}
	if !node.LastSeen.IsZero() {
		node.LastSeen = node.LastSeen.UTC()
	} else {
		node.LastSeen = time.Now().UTC()
	}
	if node.Models == nil {
		node.Models = []ModelState{}
	}
	node = normalizeNode(node)
	s.state.Nodes[node.NodeID] = node
	s.state.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) SetSetupComplete(complete bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.SetupComplete = complete
	s.state.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) SetAdminTokenHint(hint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AdminTokenHint = hint
	s.state.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) UpsertClientAPIKey(key ClientAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key.ID == "" {
		key.ID = "key_" + randomHex(4)
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}
	key.Enabled = key.Enabled || !containsClientKey(s.state.ClientAPIKeys, key.ID)
	for i := range s.state.ClientAPIKeys {
		if s.state.ClientAPIKeys[i].ID == key.ID {
			s.state.ClientAPIKeys[i] = key
			s.state.UpdatedAt = time.Now().UTC()
			return s.saveLocked()
		}
	}
	s.state.ClientAPIKeys = append(s.state.ClientAPIKeys, key)
	s.state.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *Store) TouchClientAPIKey(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.ClientAPIKeys {
		if s.state.ClientAPIKeys[i].ID == id {
			s.state.ClientAPIKeys[i].LastUsed = time.Now().UTC()
			_ = s.saveLocked()
			return
		}
	}
}

func (s *Store) UpdateSession(session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session.SessionID == "" {
		return nil
	}
	session.UpdatedAt = time.Now().UTC()
	s.state.Sessions[session.SessionID] = session
	s.state.UpdatedAt = session.UpdatedAt
	return s.saveLocked()
}

func (s *Store) Session(id string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.state.Sessions[id]
	return session, ok
}

func (s *Store) AddAudit(event AuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.Timestamp = time.Now().UTC()
	s.state.Audit = append([]AuditEvent{event}, s.state.Audit...)
	if len(s.state.Audit) > 250 {
		s.state.Audit = s.state.Audit[:250]
	}
	_ = s.saveLocked()
}

func (s *Store) saveLocked() error {
	s.state.SchemaVersion = CurrentSchemaVersion
	if s.state.ConfigVersion == 0 {
		s.state.ConfigVersion = 1
	}
	s.state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func cloneState(state State) State {
	data, _ := json.Marshal(state)
	var out State
	_ = json.Unmarshal(data, &out)
	return out
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "local"
	}
	return hex.EncodeToString(buf)
}

func containsClientKey(keys []ClientAPIKey, id string) bool {
	for _, key := range keys {
		if key.ID == id {
			return true
		}
	}
	return false
}

func ensureNodeMap(nodes map[string]Node) map[string]Node {
	if nodes == nil {
		return map[string]Node{}
	}
	return nodes
}

func normalizeNodeMap(nodes map[string]Node) map[string]Node {
	nodes = ensureNodeMap(nodes)
	for id, node := range nodes {
		nodes[id] = normalizeNode(node)
	}
	return nodes
}

func ensureSessionMap(sessions map[string]Session) map[string]Session {
	if sessions == nil {
		return map[string]Session{}
	}
	return sessions
}

func ensureClientKeys(keys []ClientAPIKey) []ClientAPIKey {
	if keys == nil {
		return []ClientAPIKey{}
	}
	return keys
}

func ensureAudit(events []AuditEvent) []AuditEvent {
	if events == nil {
		return []AuditEvent{}
	}
	return events
}

func ensureEnrollmentQueue(queue []EnrollmentRequest) []EnrollmentRequest {
	if queue == nil {
		return []EnrollmentRequest{}
	}
	return queue
}

func normalizeEnrollmentQueue(queue []EnrollmentRequest) []EnrollmentRequest {
	queue = ensureEnrollmentQueue(queue)
	for i := range queue {
		if queue[i].ControlLevel == "" {
			queue[i].ControlLevel = ControlLevelManaged
		}
		if queue[i].TrustLevel == "" {
			queue[i].TrustLevel = trustLevelForEndpoint(queue[i].URL, false)
		}
		if queue[i].CapabilitySource == "" {
			queue[i].CapabilitySource = CapabilitySourceSubscriberReported
		}
		if queue[i].ApprovalState == "" {
			queue[i].ApprovalState = ApprovalStatePending
		}
	}
	return queue
}

func normalizeNode(node Node) Node {
	if node.ControlLevel == "" {
		if strings.EqualFold(node.Role, ControlLevelPassive) {
			node.ControlLevel = ControlLevelPassive
		} else {
			node.ControlLevel = ControlLevelManaged
		}
	}
	if node.ControlLevel != ControlLevelPassive && node.ControlLevel != ControlLevelManaged {
		node.ControlLevel = ControlLevelManaged
	}
	if node.TrustLevel == "" {
		node.TrustLevel = trustLevelForEndpoint(primaryNodeEndpoint(node), node.URL == "" && node.OllamaURL == "")
	}
	if node.ApprovalState == "" {
		if node.Approved {
			node.ApprovalState = ApprovalStateApproved
		} else {
			node.ApprovalState = ApprovalStatePending
		}
	}
	if node.ApprovalState == ApprovalStateApproved {
		node.Approved = true
	}
	if node.ControlLevel == ControlLevelPassive {
		if node.CapabilitySource == "" {
			node.CapabilitySource = CapabilitySourceMarshalObserved
		}
		if node.HealthSource == "" {
			node.HealthSource = HealthSourceMarshalObserved
		}
		if node.ModelInventorySource == "" {
			node.ModelInventorySource = ModelInventorySourceMarshalObserved
		}
		if node.BenchmarkSource == "" {
			node.BenchmarkSource = BenchmarkSourceNone
		}
		if node.TelemetryLevel == "" {
			node.TelemetryLevel = TelemetryLevelMarshalObservedMetadata
		}
		node.WarmStateSupported = false
		node.ManagementSupported = false
	} else {
		if node.CapabilitySource == "" {
			node.CapabilitySource = CapabilitySourceSubscriberReported
		}
		if node.HealthSource == "" {
			node.HealthSource = HealthSourceSubscriberReported
		}
		if node.ModelInventorySource == "" {
			node.ModelInventorySource = ModelInventorySourceSubscriberReported
		}
		if node.BenchmarkSource == "" {
			node.BenchmarkSource = BenchmarkSourceNone
		}
		if node.TelemetryLevel == "" {
			node.TelemetryLevel = TelemetryLevelRichMetadata
		}
		node.ManagementSupported = true
	}
	if node.LastSeen.IsZero() {
		node.LastSeen = time.Now().UTC()
	} else {
		node.LastSeen = node.LastSeen.UTC()
	}
	if node.LastObservedAt == nil || node.LastObservedAt.IsZero() {
		node.LastObservedAt = timePtr(node.LastSeen)
	} else {
		observed := node.LastObservedAt.UTC()
		node.LastObservedAt = &observed
	}
	if node.ControlLevel == ControlLevelManaged {
		if node.LastReportedAt == nil || node.LastReportedAt.IsZero() {
			node.LastReportedAt = timePtr(node.LastSeen)
		} else {
			reported := node.LastReportedAt.UTC()
			node.LastReportedAt = &reported
		}
	} else {
		node.LastReportedAt = nil
	}
	return node
}

func timePtr(t time.Time) *time.Time {
	t = t.UTC()
	return &t
}

func primaryNodeEndpoint(node Node) string {
	if node.URL != "" {
		return node.URL
	}
	return node.OllamaURL
}

func trustLevelForEndpoint(raw string, localDefault bool) string {
	if raw == "" {
		if localDefault {
			return TrustLevelLocal
		}
		return TrustLevelLANUnverified
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return TrustLevelLANUnverified
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return TrustLevelLocal
	}
	ip := net.ParseIP(host)
	if ip == nil {
		if strings.HasSuffix(host, ".local") {
			return TrustLevelLANUnverified
		}
		return TrustLevelExternal
	}
	if ip.IsLoopback() {
		return TrustLevelLocal
	}
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return TrustLevelLANUnverified
	}
	return TrustLevelExternal
}
