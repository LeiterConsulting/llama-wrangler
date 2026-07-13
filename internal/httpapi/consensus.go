package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	consensusengine "llama-wrangler/internal/consensus"
	"llama-wrangler/internal/routing"
	"llama-wrangler/internal/telemetry"
)

const consensusMaxResponseBytes int64 = 8 << 20

const (
	consensusFailureMissingProxyURL = "missing_proxy_url"
	consensusFailureConnectionError = "connection_error"
	consensusFailureUpstream4xx     = "upstream_4xx"
	consensusFailureUpstream5xx     = "upstream_5xx"
	consensusFailureBodyRead        = "body_read_failure"
	consensusFailureResponseSize    = "response_size_limit"
	consensusFailureTimeout         = "timeout"
	consensusFailureCancellation    = "cancellation"
)

var (
	errConsensusInsufficientSuccesses = errors.New("insufficient successful consensus participants")
	errConsensusResponseTooLarge      = errors.New("consensus participant response exceeds memory limit")
)

type consensusParticipantOutcome struct {
	NodeID        string
	StatusCode    int
	Header        http.Header
	Body          []byte
	DurationMS    int64
	FailureReason string
	Err           error
}

type ConsensusParticipantFailure struct {
	NodeID     string `json:"node_id"`
	ReasonCode string `json:"reason_code"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type consensusProxyOutcome struct {
	Participants        []string
	SuccessfulNodes     []string
	FailedNodes         []string
	ParticipantFailures []ConsensusParticipantFailure
	FailureReasonCounts map[string]int
	WinnerNode          string
	Required            int
	Limit               int
	AgreementScore      float64
	AgreementCount      int
	ComparisonStrategy  string
	ConsensusReached    bool
	Disagreement        bool
	ValidatorPassed     bool
	BytesWritten        int64
	ResponseCommitted   bool
	ClientCancelled     bool
	TimedOut            bool
	DurationMS          int64
	Err                 error
}

func (s *Server) forwardConsensus(ctx context.Context, w http.ResponseWriter, path, surface string, body []byte, decision routing.Decision, debug bool) consensusProxyOutcome {
	started := time.Now()
	outcome := consensusProxyOutcome{
		Participants:        append([]string(nil), decision.CandidateNodes...),
		Required:            decision.ConsensusRequired,
		Limit:               decision.ConsensusLimit,
		FailureReasonCounts: map[string]int{},
	}
	if outcome.Required < 1 {
		outcome.Required = routing.ConsensusDefaultMinParticipants
	}
	if len(outcome.Participants) < outcome.Required {
		outcome.Err = errConsensusInsufficientSuccesses
		return outcome
	}

	fanoutCtx, cancel := context.WithTimeout(ctx, s.cfg.Routing.Timeout())
	defer cancel()
	results := make(chan consensusParticipantOutcome, len(outcome.Participants))
	for _, nodeID := range outcome.Participants {
		nodeID := nodeID
		go func() {
			results <- s.fetchConsensusParticipant(fanoutCtx, nodeID, path, body, decision.ResolvedModel)
		}()
	}

	responseByNode := map[string]consensusParticipantOutcome{}
	failureByNode := map[string]ConsensusParticipantFailure{}
	completed := 0
collect:
	for completed < len(outcome.Participants) {
		select {
		case response := <-results:
			completed++
			if response.Err != nil {
				failureByNode[response.NodeID] = consensusParticipantFailure(response)
				continue
			}
			responseByNode[response.NodeID] = response
		case <-fanoutCtx.Done():
			if ctx.Err() != nil {
				outcome.ClientCancelled = true
				outcome.Err = ctx.Err()
				break collect
			}
			outcome.TimedOut = true
			outcome.Err = fanoutCtx.Err()
			break collect
		}
	}
	unfinishedReason := ""
	if outcome.ClientCancelled {
		unfinishedReason = consensusFailureCancellation
	} else if outcome.TimedOut {
		unfinishedReason = consensusFailureTimeout
	}
	for _, nodeID := range outcome.Participants {
		if _, ok := responseByNode[nodeID]; ok {
			outcome.SuccessfulNodes = append(outcome.SuccessfulNodes, nodeID)
			continue
		}
		failure, ok := failureByNode[nodeID]
		if !ok {
			failure = ConsensusParticipantFailure{
				NodeID:     nodeID,
				ReasonCode: unfinishedReason,
				DurationMS: time.Since(started).Milliseconds(),
			}
			if failure.ReasonCode == "" {
				failure.ReasonCode = consensusFailureConnectionError
			}
		}
		outcome.FailedNodes = append(outcome.FailedNodes, nodeID)
		outcome.ParticipantFailures = append(outcome.ParticipantFailures, failure)
		outcome.FailureReasonCounts[failure.ReasonCode]++
	}
	outcome.DurationMS = time.Since(started).Milliseconds()
	if outcome.ClientCancelled {
		return outcome
	}
	if len(responseByNode) < outcome.Required {
		outcome.Err = errConsensusInsufficientSuccesses
		return outcome
	}

	candidates := make([]consensusengine.Candidate, 0, len(responseByNode))
	for _, nodeID := range outcome.Participants {
		if response, ok := responseByNode[nodeID]; ok {
			candidates = append(candidates, consensusengine.Candidate{NodeID: nodeID, Body: response.Body})
		}
	}
	result := (consensusengine.Engine{Evaluator: consensusengine.NoopEvaluator{}}).Evaluate(surface, candidates)
	winner, ok := responseByNode[result.Winner.NodeID]
	if !ok {
		outcome.Err = errors.New("consensus winner response missing")
		return outcome
	}
	outcome.WinnerNode = winner.NodeID
	outcome.AgreementScore = result.AgreementScore
	outcome.AgreementCount = result.AgreementCount
	outcome.ComparisonStrategy = result.Strategy
	outcome.ConsensusReached = result.ConsensusReached
	outcome.Disagreement = result.Disagreement
	outcome.ValidatorPassed = result.ValidatorPassed
	outcome.Err = nil
	for key, values := range winner.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if debug {
		w.Header().Set("X-Llama-Wrangler-Consensus", consensusDebugState(outcome))
		w.Header().Set("X-Llama-Wrangler-Consensus-Participants", fmt.Sprintf("%d", len(outcome.SuccessfulNodes)))
		w.Header().Set("X-Llama-Wrangler-Consensus-Agreement", fmt.Sprintf("%.3f", outcome.AgreementScore))
		w.Header().Set("X-Llama-Wrangler-Consensus-Winner", outcome.WinnerNode)
	}
	w.WriteHeader(winner.StatusCode)
	written, err := w.Write(winner.Body)
	flushResponse(w)
	outcome.BytesWritten = int64(written)
	outcome.ResponseCommitted = true
	if err != nil {
		outcome.Err = err
	}
	return outcome
}

func (s *Server) fetchConsensusParticipant(ctx context.Context, nodeID, path string, body []byte, model string) consensusParticipantOutcome {
	started := time.Now()
	outcome := consensusParticipantOutcome{NodeID: nodeID}
	url := s.nodeProxyURL(nodeID, path)
	if url == "" {
		outcome.FailureReason = consensusFailureMissingProxyURL
		outcome.Err = fmt.Errorf("node %s has no proxy URL", nodeID)
		outcome.DurationMS = time.Since(started).Milliseconds()
		return outcome
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rewriteModel(body, model)))
	if err != nil {
		outcome.FailureReason = consensusFailureConnectionError
		outcome.Err = err
		outcome.DurationMS = time.Since(started).Milliseconds()
		return outcome
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		outcome.FailureReason = consensusFailureReasonForContext(ctx, consensusFailureConnectionError)
		outcome.Err = err
		outcome.DurationMS = time.Since(started).Milliseconds()
		return outcome
	}
	defer resp.Body.Close()
	outcome.StatusCode = resp.StatusCode
	outcome.Header = resp.Header.Clone()
	if resp.StatusCode >= http.StatusInternalServerError {
		outcome.FailureReason = consensusFailureUpstream5xx
		outcome.Err = fmt.Errorf("node %s returned %s", nodeID, resp.Status)
		outcome.DurationMS = time.Since(started).Milliseconds()
		return outcome
	}
	if resp.StatusCode >= http.StatusBadRequest {
		outcome.FailureReason = consensusFailureUpstream4xx
		outcome.Err = fmt.Errorf("node %s returned %s", nodeID, resp.Status)
		outcome.DurationMS = time.Since(started).Milliseconds()
		return outcome
	}
	limited := io.LimitReader(resp.Body, consensusMaxResponseBytes+1)
	outcome.Body, err = io.ReadAll(limited)
	outcome.DurationMS = time.Since(started).Milliseconds()
	if err != nil {
		outcome.FailureReason = consensusFailureReasonForContext(ctx, consensusFailureBodyRead)
		outcome.Err = err
		return outcome
	}
	if int64(len(outcome.Body)) > consensusMaxResponseBytes {
		outcome.Body = nil
		outcome.FailureReason = consensusFailureResponseSize
		outcome.Err = errConsensusResponseTooLarge
	}
	return outcome
}

func consensusFailureReasonForContext(ctx context.Context, fallback string) string {
	if ctx == nil {
		return fallback
	}
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return consensusFailureTimeout
	case context.Canceled:
		return consensusFailureCancellation
	default:
		return fallback
	}
}

func consensusParticipantFailure(outcome consensusParticipantOutcome) ConsensusParticipantFailure {
	reason := outcome.FailureReason
	if reason == "" {
		reason = consensusFailureConnectionError
	}
	durationMS := outcome.DurationMS
	if durationMS < 0 {
		durationMS = 0
	}
	return ConsensusParticipantFailure{
		NodeID:     outcome.NodeID,
		ReasonCode: reason,
		StatusCode: outcome.StatusCode,
		DurationMS: durationMS,
	}
}

func consensusInferenceFailure(outcome consensusProxyOutcome) (int, string, string) {
	if len(outcome.SuccessfulNodes) == 0 && len(outcome.ParticipantFailures) > 0 {
		allUpstream4xx := true
		status := 0
		for _, failure := range outcome.ParticipantFailures {
			if failure.ReasonCode != consensusFailureUpstream4xx || failure.StatusCode < 400 || failure.StatusCode >= 500 {
				allUpstream4xx = false
				break
			}
			if status == 0 {
				status = failure.StatusCode
			}
		}
		if allUpstream4xx && status != 0 {
			code, message := normalizedUpstreamError(status)
			return status, code, message
		}
	}
	if outcome.TimedOut || outcome.FailureReasonCounts[consensusFailureTimeout] > 0 {
		return http.StatusGatewayTimeout, errorCodeConsensusInsufficientSuccesses, "Consensus timed out before enough Managed Node responses succeeded."
	}
	return http.StatusBadGateway, errorCodeConsensusInsufficientSuccesses, "Consensus did not receive enough successful Managed Node responses."
}

func consensusDebugRequested(r *http.Request, body []byte) bool {
	if r != nil && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Llama-Wrangler-Debug")), "true") {
		return true
	}
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) == nil {
		debug, _ := payload["debug"].(bool)
		return debug
	}
	return false
}

func consensusDebugState(outcome consensusProxyOutcome) string {
	if outcome.ConsensusReached {
		return "reached"
	}
	return "no_majority"
}

func consensusTelemetry(decision routing.Decision, requestID string, outcome consensusProxyOutcome) telemetry.Event {
	escalationRecommended := strings.Contains(decision.ExecutionMode, "delta") && !outcome.ConsensusReached
	return telemetry.Event{
		"request_id":              requestID,
		"execution_mode":          decision.ExecutionMode,
		"participants":            outcome.Participants,
		"participant_count":       len(outcome.Participants),
		"required_participants":   outcome.Required,
		"max_participants":        outcome.Limit,
		"successful_participants": outcome.SuccessfulNodes,
		"successful_count":        len(outcome.SuccessfulNodes),
		"failed_participants":     outcome.FailedNodes,
		"failed_count":            len(outcome.FailedNodes),
		"participant_failures":    outcome.ParticipantFailures,
		"failure_reason_counts":   outcome.FailureReasonCounts,
		"agreement_score":         outcome.AgreementScore,
		"agreement_count":         outcome.AgreementCount,
		"comparison_strategy":     outcome.ComparisonStrategy,
		"validator_passed":        outcome.ValidatorPassed,
		"winner_node":             outcome.WinnerNode,
		"consensus_reached":       outcome.ConsensusReached,
		"disagreement_detected":   outcome.Disagreement,
		"timed_out":               outcome.TimedOut,
		"client_cancelled":        outcome.ClientCancelled,
		"duration_ms":             outcome.DurationMS,
		"escalation_recommended":  escalationRecommended,
		"escalation_reason":       consensusEscalationReason(outcome, escalationRecommended),
		"frontier_used":           false,
		"content_recorded":        false,
	}
}

func consensusEscalationReason(outcome consensusProxyOutcome, recommended bool) string {
	if !recommended {
		return ""
	}
	if outcome.Err != nil {
		return "insufficient_successful_participants"
	}
	return "no_majority"
}
