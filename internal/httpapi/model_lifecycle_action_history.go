package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
)

const (
	modelLifecycleActionHistoryDefaultLimit = 20
	modelLifecycleActionHistoryMaxLimit     = 50
)

type ModelLifecycleActionHistoryStatus struct {
	Window            string                            `json:"window"`
	Retention         string                            `json:"retention"`
	MaxActionsPerNode int                               `json:"max_actions_per_node"`
	Filters           ModelLifecycleActionHistoryFilter `json:"filters"`
	Summary           map[string]int                    `json:"summary"`
	TotalMatches      int                               `json:"total_matches"`
	Count             int                               `json:"count"`
	Actions           []ModelLifecycleActionHistoryItem `json:"actions"`
}

type ModelLifecycleActionHistoryFilter struct {
	Status string `json:"status,omitempty"`
	NodeID string `json:"node_id,omitempty"`
	Limit  int    `json:"limit"`
}

type ModelLifecycleActionHistoryItem struct {
	ActionID        string `json:"action_id"`
	ActionType      string `json:"action_type"`
	Policy          string `json:"policy"`
	NodeID          string `json:"node_id"`
	ControlLevel    string `json:"control_level"`
	TrustLevel      string `json:"trust_level"`
	ApprovalState   string `json:"approval_state"`
	Model           string `json:"model"`
	DesiredKeepWarm bool   `json:"desired_keep_warm"`
	Status          string `json:"status"`
	RequestedAt     string `json:"requested_at,omitempty"`
	ClaimedAt       string `json:"claimed_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	CompletedAt     string `json:"completed_at,omitempty"`
	FailedAt        string `json:"failed_at,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
}

func (s *Server) modelLifecycleActionHistory(w http.ResponseWriter, r *http.Request) {
	filter, ok := modelLifecycleActionHistoryFilterFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid model lifecycle action history filter"})
		return
	}
	writeJSON(w, http.StatusOK, s.modelLifecycleActionHistoryStatus(filter))
}

func modelLifecycleActionHistoryFilterFromRequest(r *http.Request) (ModelLifecycleActionHistoryFilter, bool) {
	filter := ModelLifecycleActionHistoryFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		NodeID: safeBenchmarkString(r.URL.Query().Get("node_id")),
		Limit:  modelLifecycleActionHistoryDefaultLimit,
	}
	if filter.Status != "" && safeModelLifecycleActionStatus(filter.Status) == "" {
		return ModelLifecycleActionHistoryFilter{}, false
	}
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > modelLifecycleActionHistoryMaxLimit {
			return ModelLifecycleActionHistoryFilter{}, false
		}
		filter.Limit = limit
	}
	return filter, true
}

func (s *Server) modelLifecycleActionHistoryStatus(filter ModelLifecycleActionHistoryFilter) ModelLifecycleActionHistoryStatus {
	return summarizeModelLifecycleActionHistory(s.store.Snapshot().Nodes, normalizeModelLifecycleActionHistoryFilter(filter))
}

func normalizeModelLifecycleActionHistoryFilter(filter ModelLifecycleActionHistoryFilter) ModelLifecycleActionHistoryFilter {
	filter.Status = safeModelLifecycleActionStatus(filter.Status)
	filter.NodeID = safeBenchmarkString(filter.NodeID)
	if filter.Limit < 1 || filter.Limit > modelLifecycleActionHistoryMaxLimit {
		filter.Limit = modelLifecycleActionHistoryDefaultLimit
	}
	return filter
}

func summarizeModelLifecycleActionHistory(nodes map[string]appstate.Node, filter ModelLifecycleActionHistoryFilter) ModelLifecycleActionHistoryStatus {
	status := ModelLifecycleActionHistoryStatus{
		Window:            "recent_model_lifecycle_actions",
		Retention:         "bounded_node_state_recent_actions",
		MaxActionsPerNode: maxModelLifecycleActions,
		Filters:           filter,
		Summary:           map[string]int{},
		Actions:           []ModelLifecycleActionHistoryItem{},
	}
	for _, node := range nodes {
		if routingStatusControlLevel(node) != appstate.ControlLevelManaged {
			continue
		}
		if filter.NodeID != "" && node.NodeID != filter.NodeID {
			continue
		}
		for _, action := range modelLifecycleActions(node.Observed["model_lifecycle_actions"]) {
			item := modelLifecycleActionHistoryItem(node, action)
			if filter.Status != "" && item.Status != filter.Status {
				continue
			}
			status.Actions = append(status.Actions, item)
			status.Summary["actions"]++
			status.Summary[item.Status]++
			if item.ErrorCode != "" {
				status.Summary["with_error_code"]++
			}
		}
	}
	status.TotalMatches = len(status.Actions)
	sort.Slice(status.Actions, func(i, j int) bool {
		left := modelLifecycleActionSortTime(status.Actions[i])
		right := modelLifecycleActionSortTime(status.Actions[j])
		if left.Equal(right) {
			return status.Actions[i].ActionID > status.Actions[j].ActionID
		}
		return left.After(right)
	})
	if len(status.Actions) > filter.Limit {
		status.Actions = status.Actions[:filter.Limit]
	}
	status.Count = len(status.Actions)
	return status
}

func modelLifecycleActionHistoryItem(node appstate.Node, action map[string]interface{}) ModelLifecycleActionHistoryItem {
	status := safeModelLifecycleActionStatus(benchmarkJobString(action["status"]))
	if status == "" {
		status = "unknown"
	}
	actionType := benchmarkJobString(action["action_type"])
	if actionType != modelLifecycleActionKeepWarm {
		actionType = "unknown"
	}
	policy := benchmarkJobString(action["policy"])
	if policy != modelLifecycleActionPolicyManaged {
		policy = "legacy_managed_model_action"
	}
	desired, _ := action["desired_keep_warm"].(bool)
	return ModelLifecycleActionHistoryItem{
		ActionID:        safeBenchmarkString(benchmarkJobString(action["action_id"])),
		ActionType:      actionType,
		Policy:          policy,
		NodeID:          safeBenchmarkString(node.NodeID),
		ControlLevel:    appstate.ControlLevelManaged,
		TrustLevel:      node.TrustLevel,
		ApprovalState:   modelLifecycleActionApprovalState(node),
		Model:           safeBenchmarkString(benchmarkJobString(action["model"])),
		DesiredKeepWarm: desired,
		Status:          status,
		RequestedAt:     modelLifecycleActionTimeString(action["requested_at"]),
		ClaimedAt:       modelLifecycleActionTimeString(action["claimed_at"]),
		UpdatedAt:       modelLifecycleActionTimeString(action["updated_at"]),
		CompletedAt:     modelLifecycleActionTimeString(action["completed_at"]),
		FailedAt:        modelLifecycleActionTimeString(action["failed_at"]),
		ErrorCode:       safeModelLifecycleActionErrorCode(benchmarkJobString(action["error_code"])),
	}
}

func modelLifecycleActionApprovalState(node appstate.Node) string {
	if node.ApprovalState != "" {
		return node.ApprovalState
	}
	if node.Approved {
		return appstate.ApprovalStateApproved
	}
	return appstate.ApprovalStatePending
}

func modelLifecycleActionTimeString(value interface{}) string {
	parsed, ok := benchmarkJobTime(value)
	if !ok {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func modelLifecycleActionSortTime(item ModelLifecycleActionHistoryItem) time.Time {
	for _, value := range []string{item.UpdatedAt, item.CompletedAt, item.FailedAt, item.ClaimedAt, item.RequestedAt} {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func safeModelLifecycleActionErrorCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(safeBenchmarkString(value)))
	if value == "" {
		return ""
	}
	if value == "[redacted]" {
		return "redacted_error_code"
	}
	if len(value) > 64 {
		return "invalid_error_code"
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			continue
		}
		return "invalid_error_code"
	}
	return value
}
