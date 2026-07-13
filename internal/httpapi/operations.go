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
	Consensus     ConsensusCounter      `json:"consensus"`
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

type ConsensusCounter struct {
	Total                int            `json:"total"`
	Reached              int            `json:"reached"`
	NoMajority           int            `json:"no_majority"`
	Failed               int            `json:"failed"`
	TimedOut             int            `json:"timed_out"`
	Cancelled            int            `json:"cancelled"`
	StreamingRejected    int            `json:"streaming_rejected"`
	LastParticipantCount int            `json:"last_participant_count,omitempty"`
	LastSuccessfulCount  int            `json:"last_successful_count,omitempty"`
	LastAgreementScore   float64        `json:"last_agreement_score,omitempty"`
	LastWinnerNode       string         `json:"last_winner_node,omitempty"`
	FailureReasons       map[string]int `json:"failure_reasons"`
	LastAt               *time.Time     `json:"last_at,omitempty"`
}

func (s *Server) operationStats() OperationStats {
	return summarizeOperations(s.store.Snapshot().Audit)
}

func summarizeOperations(events []appstate.AuditEvent) OperationStats {
	stats := OperationStats{
		Window:      "recent_audit_events",
		AuditEvents: len(events),
		Consensus:   ConsensusCounter{FailureReasons: map[string]int{}},
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
		case "consensus":
			stats.Consensus.Total++
			for reason, count := range consensusFailureReasonCounts(event) {
				stats.Consensus.FailureReasons[reason] += count
			}
			if boolField(event, "consensus_reached") {
				stats.Consensus.Reached++
			} else if boolField(event, "streaming_rejected") {
				stats.Consensus.StreamingRejected++
				stats.Consensus.Failed++
			} else if boolField(event, "client_cancelled") {
				stats.Consensus.Cancelled++
				stats.Consensus.Failed++
			} else if intField(event, "successful_count") < intField(event, "required_participants") {
				stats.Consensus.Failed++
			} else {
				stats.Consensus.NoMajority++
			}
			if boolField(event, "timed_out") {
				stats.Consensus.TimedOut++
			}
			if stats.Consensus.LastAt == nil || event.Timestamp.After(*stats.Consensus.LastAt) {
				stats.Consensus.LastParticipantCount = intField(event, "participant_count")
				stats.Consensus.LastSuccessfulCount = intField(event, "successful_count")
				stats.Consensus.LastAgreementScore = floatField(event, "agreement_score")
				stats.Consensus.LastWinnerNode = stringField(event, "winner_node")
			}
			latestTime(&stats.Consensus.LastAt, event.Timestamp)
		}
	}
	return stats
}

func consensusFailureReasonCounts(event appstate.AuditEvent) map[string]int {
	out := map[string]int{}
	if event.Fields == nil {
		return out
	}
	value := event.Fields["failure_reason_counts"]
	switch counts := value.(type) {
	case map[string]int:
		for reason, count := range counts {
			if isConsensusFailureReason(reason) && count > 0 {
				out[reason] += count
			}
		}
	case map[string]interface{}:
		for reason, count := range counts {
			if isConsensusFailureReason(reason) {
				out[reason] += max(benchmarkJobInt(count, 0), 0)
			}
		}
	}
	return out
}

func isConsensusFailureReason(reason string) bool {
	switch reason {
	case consensusFailureMissingProxyURL,
		consensusFailureConnectionError,
		consensusFailureUpstream4xx,
		consensusFailureUpstream5xx,
		consensusFailureBodyRead,
		consensusFailureResponseSize,
		consensusFailureTimeout,
		consensusFailureCancellation:
		return true
	default:
		return false
	}
}

func boolField(event appstate.AuditEvent, key string) bool {
	if event.Fields == nil {
		return false
	}
	value, _ := event.Fields[key].(bool)
	return value
}

func intField(event appstate.AuditEvent, key string) int {
	if event.Fields == nil {
		return 0
	}
	return benchmarkJobInt(event.Fields[key], 0)
}

func floatField(event appstate.AuditEvent, key string) float64 {
	if event.Fields == nil {
		return 0
	}
	switch value := event.Fields[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
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
