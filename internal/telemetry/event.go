package telemetry

import (
	"encoding/json"
	"log"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
	"llama-wrangler/internal/hec"
)

type Event map[string]interface{}

type Sink struct {
	cfg    config.Config
	store  *appstate.Store
	client *hec.Client
}

func New(cfg config.Config, store *appstate.Store) *Sink {
	var client *hec.Client
	if cfg.Telemetry.SplunkHEC.Enabled {
		client = hec.New(cfg.Telemetry.SplunkHEC)
	}
	return &Sink{cfg: cfg, store: store, client: client}
}

func (s *Sink) Emit(eventType string, fields Event) {
	if fields == nil {
		fields = Event{}
	}
	fields["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	fields["event_type"] = eventType
	fields["version"] = s.cfg.Version
	if _, ok := fields["privacy_mode"]; !ok {
		fields["privacy_mode"] = "local_first"
	}
	delete(fields, "prompt")
	delete(fields, "messages")
	delete(fields, "response")

	if s.cfg.Telemetry.JSONLogs {
		if data, err := json.Marshal(fields); err == nil {
			log.Println(string(data))
		}
	}
	if s.store != nil {
		s.store.AddAudit(appstate.AuditEvent{
			Type:      eventType,
			RequestID: stringField(fields, "request_id"),
			Message:   eventType,
			Fields:    map[string]interface{}(fields),
		})
	}
	if s.client != nil {
		if err := s.client.Send(eventType, fields); err != nil && s.store != nil {
			s.store.AddAudit(appstate.AuditEvent{Type: "hec_error", Message: err.Error()})
		}
	}
}

func stringField(fields Event, key string) string {
	if v, ok := fields[key].(string); ok {
		return v
	}
	return ""
}
