package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	errorCodeAdminAuthRequired              = "admin_auth_required"
	errorCodeClientKeyRequired              = "client_api_key_required"
	errorCodeClientAuthRateLimited          = "client_auth_rate_limited"
	errorCodeNoEligibleNode                 = "no_eligible_node"
	errorCodeQueueFull                      = "queue_full"
	errorCodeUpstreamUnavailable            = "upstream_unavailable"
	errorCodeConsensusStreamingUnsupported  = "consensus_streaming_not_supported"
	errorCodeConsensusInsufficientSuccesses = "consensus_insufficient_successful_participants"
	errorCodeProxyTargetRequired            = "proxy_target_required"
)

type openAIErrorResponse struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   interface{} `json:"param"`
	Code    string      `json:"code"`
}

func writeInferenceError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	path := ""
	if r != nil && r.URL != nil {
		path = r.URL.Path
	}
	writeInferenceErrorForPath(w, path, status, code, message)
}

func writeInferenceErrorForPath(w http.ResponseWriter, path string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	switch inferenceCompatibilityPath(path) {
	case "openai":
		_ = json.NewEncoder(w).Encode(openAIErrorResponse{Error: openAIError{
			Message: message,
			Type:    openAIErrorType(status),
			Param:   nil,
			Code:    code,
		}})
	case "ollama":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": message,
			"type":  ollamaErrorType(status),
			"code":  code,
		})
	default:
		_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
	}
}

func inferenceCompatibility(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return inferenceCompatibilityPath(r.URL.Path)
}

func inferenceCompatibilityPath(path string) string {
	if strings.HasPrefix(path, "/v1/") {
		return "openai"
	}
	if strings.HasPrefix(path, "/api/") {
		return "ollama"
	}
	return ""
}

func normalizedUpstreamError(status int) (string, string) {
	switch status {
	case http.StatusBadRequest:
		return "upstream_invalid_request", "Upstream Ollama rejected the request."
	case http.StatusUnauthorized, http.StatusForbidden:
		return "upstream_auth_failed", "Upstream Ollama authorization failed."
	case http.StatusNotFound:
		return "upstream_not_found", "Requested model or upstream resource was not found."
	case http.StatusTooManyRequests:
		return "upstream_rate_limited", "Upstream Ollama is rate limited."
	default:
		return "upstream_request_failed", "Upstream Ollama returned an error."
	}
}

func openAIErrorType(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusBadRequest, http.StatusNotFound:
		return "invalid_request_error"
	default:
		if status >= 500 {
			return "server_error"
		}
		return "api_error"
	}
}

func ollamaErrorType(status int) string {
	if status >= 500 {
		return "server_error"
	}
	if status == http.StatusTooManyRequests {
		return "rate_limit_error"
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return "authentication_error"
	}
	return "invalid_request_error"
}
