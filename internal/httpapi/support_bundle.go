package httpapi

import (
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
)

const (
	supportBundleSchemaName          = "llama-wrangler.support-bundle"
	supportBundleSchemaVersion       = 1
	supportBundleJSONSchemaPath      = "schemas/support_bundle.schema.json"
	supportBundleDocumentationPath   = "docs/13_support_bundle_schema.md"
	supportBundleCompatibilityPolicy = "additive_backward_compatible"
)

type supportBundle struct {
	GeneratedAt   time.Time              `json:"generated_at"`
	Version       string                 `json:"version"`
	BundleSchema  supportBundleSchema    `json:"bundle_schema"`
	Service       supportServiceMetadata `json:"service"`
	Config        interface{}            `json:"config"`
	Nodes         map[string]interface{} `json:"nodes"`
	Sessions      map[string]interface{} `json:"sessions"`
	Queue         QueueSnapshot          `json:"queue"`
	Audit         []appstate.AuditEvent  `json:"audit"`
	SecretStorage interface{}            `json:"secret_storage"`
	Privacy       supportPrivacyMetadata `json:"privacy"`
}

type supportBundleSchema struct {
	Name          string `json:"name"`
	Version       int    `json:"version"`
	JSONSchema    string `json:"json_schema"`
	Documentation string `json:"documentation"`
	Compatibility string `json:"compatibility"`
}

type supportServiceMetadata struct {
	Role             string                       `json:"role"`
	NodeID           string                       `json:"node_id"`
	SetupComplete    bool                         `json:"setup_complete"`
	SchemaVersion    int                          `json:"schema_version"`
	ConfigVersion    int                          `json:"config_version"`
	MigrationHistory []appstate.MigrationRecord   `json:"migration_history"`
	CreatedAt        time.Time                    `json:"created_at"`
	UpdatedAt        time.Time                    `json:"updated_at"`
	ClientKeys       supportClientKeyMetadata     `json:"client_keys"`
	EnrollmentQueue  []appstate.EnrollmentRequest `json:"enrollment_queue"`
}

type supportClientKeyMetadata struct {
	Count   int `json:"count"`
	Active  int `json:"active"`
	Revoked int `json:"revoked"`
}

type supportPrivacyMetadata struct {
	PromptBodiesIncluded   bool     `json:"prompt_bodies_included"`
	ResponseBodiesIncluded bool     `json:"response_bodies_included"`
	SecretsIncluded        bool     `json:"secrets_included"`
	RedactedFields         []string `json:"redacted_fields"`
}

func (s *Server) exportSupportBundle(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.buildSupportBundle())
}

func (s *Server) buildSupportBundle() supportBundle {
	state := s.store.Snapshot()
	return supportBundle{
		GeneratedAt: time.Now().UTC(),
		Version:     s.cfg.Version,
		BundleSchema: supportBundleSchema{
			Name:          supportBundleSchemaName,
			Version:       supportBundleSchemaVersion,
			JSONSchema:    supportBundleJSONSchemaPath,
			Documentation: supportBundleDocumentationPath,
			Compatibility: supportBundleCompatibilityPolicy,
		},
		Service: supportServiceMetadata{
			Role:             state.Role,
			NodeID:           state.NodeID,
			SetupComplete:    state.SetupComplete,
			SchemaVersion:    state.SchemaVersion,
			ConfigVersion:    state.ConfigVersion,
			MigrationHistory: state.MigrationHistory,
			CreatedAt:        state.CreatedAt,
			UpdatedAt:        state.UpdatedAt,
			ClientKeys:       summarizeClientKeys(state.ClientAPIKeys),
			EnrollmentQueue:  sanitizeEnrollmentQueue(state.EnrollmentQueue),
		},
		Config:        sanitizeForSupport(sanitizeConfig(state.Config)),
		Nodes:         sanitizeNodeMap(state.Nodes),
		Sessions:      sanitizeSessionMap(state.Sessions),
		Queue:         s.queueSnapshot(),
		Audit:         sanitizeAuditEvents(state.Audit),
		SecretStorage: s.secrets.Status(),
		Privacy: supportPrivacyMetadata{
			PromptBodiesIncluded:   false,
			ResponseBodiesIncluded: false,
			SecretsIncluded:        false,
			RedactedFields: []string{
				"authorization",
				"api_key",
				"client_api_key",
				"headers",
				"messages",
				"password",
				"payload",
				"prompt",
				"request_body",
				"response",
				"response_body",
				"secret",
				"token",
			},
		},
	}
}

func summarizeClientKeys(keys []appstate.ClientAPIKey) supportClientKeyMetadata {
	summary := supportClientKeyMetadata{Count: len(keys)}
	for _, key := range keys {
		if key.Enabled {
			summary.Active++
		} else {
			summary.Revoked++
		}
	}
	return summary
}

func sanitizeEnrollmentQueue(queue []appstate.EnrollmentRequest) []appstate.EnrollmentRequest {
	out := make([]appstate.EnrollmentRequest, len(queue))
	copy(out, queue)
	for i := range out {
		out[i].TokenHash = ""
	}
	return out
}

func sanitizeNodeMap(nodes map[string]appstate.Node) map[string]interface{} {
	out := map[string]interface{}{}
	for id, node := range nodes {
		if node.Observed != nil {
			node.Observed = sanitizeForSupport(node.Observed).(map[string]interface{})
		}
		out[id] = sanitizeForSupport(node)
	}
	return out
}

func sanitizeSessionMap(sessions map[string]appstate.Session) map[string]interface{} {
	out := map[string]interface{}{}
	for id, session := range sessions {
		out[id] = sanitizeForSupport(session)
	}
	return out
}

func sanitizeAuditEvents(events []appstate.AuditEvent) []appstate.AuditEvent {
	out := make([]appstate.AuditEvent, 0, len(events))
	for _, event := range events {
		clean := event
		if event.Fields != nil {
			clean.Fields = sanitizeForSupport(event.Fields).(map[string]interface{})
		}
		clean.Message = sanitizeSupportString(clean.Message)
		out = append(out, clean)
	}
	return out
}

func sanitizeForSupport(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, item := range v {
			if supportSensitiveKey(key) {
				continue
			}
			out[key] = sanitizeForSupport(item)
		}
		return out
	case map[string]string:
		out := map[string]interface{}{}
		for key, item := range v {
			if supportSensitiveKey(key) {
				continue
			}
			out[key] = sanitizeSupportString(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeForSupport(item))
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeSupportString(item))
		}
		return out
	case string:
		return sanitizeSupportString(v)
	default:
		return v
	}
}

func supportSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	sensitiveParts := []string{
		"api_key",
		"authorization",
		"bearer",
		"client_key",
		"header",
		"message",
		"password",
		"payload",
		"prompt",
		"request_body",
		"response",
		"secret",
		"token",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func sanitizeSupportString(value string) string {
	if strings.Contains(value, "lw_admin_") || strings.Contains(value, "lw_client_") {
		return "[redacted]"
	}
	return value
}
