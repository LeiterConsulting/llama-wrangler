package httpapi

import (
	"time"

	"llama-wrangler/internal/appstate"
)

type OperationStats struct {
	Window        string                `json:"window"`
	AuditEvents   int                   `json:"audit_events"`
	Retries       OperationEventCounter `json:"retries"`
	Partials      OperationEventCounter `json:"partials"`
	Cancellations CancellationCounter   `json:"cancellations"`
}

type OperationEventCounter struct {
	Total            int        `json:"total"`
	BeforeFirstToken int        `json:"before_first_token,omitempty"`
	AfterPartial     int        `json:"after_partial,omitempty"`
	LastAt           *time.Time `json:"last_at,omitempty"`
}

type CancellationCounter struct {
	Total              int        `json:"total"`
	BeforeFirstToken   int        `json:"before_first_token,omitempty"`
	AfterPartialOutput int        `json:"after_partial_output,omitempty"`
	BeforeQueue        int        `json:"before_queue,omitempty"`
	LastAt             *time.Time `json:"last_at,omitempty"`
}

func (s *Server) operationStats() OperationStats {
	return summarizeOperations(s.store.Snapshot().Audit)
}

func summarizeOperations(events []appstate.AuditEvent) OperationStats {
	stats := OperationStats{
		Window:      "recent_audit_events",
		AuditEvents: len(events),
	}
	for _, event := range events {
		switch event.Type {
		case "upstream_retry":
			stats.Retries.Total++
			if stringField(event, "retry_phase") == "before_first_token" {
				stats.Retries.BeforeFirstToken++
			}
			latestTime(&stats.Retries.LastAt, event.Timestamp)
		case "response_partial":
			stats.Partials.Total++
			if stringField(event, "retry_phase") == "after_partial_output" {
				stats.Partials.AfterPartial++
			}
			latestTime(&stats.Partials.LastAt, event.Timestamp)
		case "request_cancelled":
			stats.Cancellations.Total++
			switch stringField(event, "reason") {
			case "client_cancelled_before_first_token":
				stats.Cancellations.BeforeFirstToken++
			case "client_cancelled_after_partial_output":
				stats.Cancellations.AfterPartialOutput++
			case "client_disconnect_before_queue":
				stats.Cancellations.BeforeQueue++
			}
			latestTime(&stats.Cancellations.LastAt, event.Timestamp)
		}
	}
	return stats
}

func stringField(event appstate.AuditEvent, key string) string {
	if event.Fields == nil {
		return ""
	}
	value, _ := event.Fields[key].(string)
	return value
}

func latestTime(current **time.Time, candidate time.Time) {
	if candidate.IsZero() {
		return
	}
	if *current == nil || candidate.After(**current) {
		t := candidate
		*current = &t
	}
}
