package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/telemetry"
)

const (
	maxModelLifecycleActions          = 8
	modelLifecycleActionKeepWarm      = "keep_warm"
	modelLifecycleActionPolicyManaged = "managed_subscriber_model_action_v1"
	modelLifecycleActionQueued        = "queued"
	modelLifecycleActionRunning       = "running"
	modelLifecycleActionCompleted     = "completed"
	modelLifecycleActionFailed        = "failed"
	modelLifecycleActionCancelled     = "cancelled"
)

type modelKeepWarmActionRequest struct {
	ModelName string `json:"model_name"`
	Model     string `json:"model"`
	KeepWarm  bool   `json:"keep_warm"`
}

type modelLifecycleActionPolicy struct {
	Eligible      bool     `json:"eligible"`
	ControlLevel  string   `json:"control_level"`
	TrustLevel    string   `json:"trust_level"`
	ApprovalState string   `json:"approval_state"`
	ReasonCodes   []string `json:"reason_codes"`
	Message       string   `json:"message"`
}

type ModelLifecycleActionPolicyStatus struct {
	Window  string                           `json:"window"`
	Summary map[string]int                   `json:"summary"`
	Nodes   []ModelLifecycleActionPolicyNode `json:"nodes"`
}

type ModelLifecycleActionPolicyNode struct {
	NodeID             string   `json:"node_id"`
	ControlLevel       string   `json:"control_level"`
	TrustLevel         string   `json:"trust_level"`
	ApprovalState      string   `json:"approval_state"`
	Source             string   `json:"source"`
	Mode               string   `json:"mode"`
	Eligible           bool     `json:"eligible"`
	SupportedActions   []string `json:"supported_actions"`
	ReasonCodes        []string `json:"reason_codes"`
	Message            string   `json:"message"`
	ModelCount         int      `json:"model_count"`
	ActionCount        int      `json:"action_count"`
	PendingActionCount int      `json:"pending_action_count"`
	LastActionStatus   string   `json:"last_action_status,omitempty"`
}

type subscriberModelActionRequest struct {
	NodeID     string `json:"node_id"`
	ActionID   string `json:"action_id"`
	Status     string `json:"status"`
	ErrorCode  string `json:"error_code"`
	ModelName  string `json:"model_name"`
	Model      string `json:"model"`
	KeepWarm   *bool  `json:"keep_warm"`
	ActionType string `json:"action_type"`
}

func createModelKeepWarmAction(node *appstate.Node, req modelKeepWarmActionRequest, now time.Time) (map[string]interface{}, modelLifecycleActionPolicy, bool) {
	modelName := safeBenchmarkString(defaultString(req.ModelName, req.Model))
	policy := modelLifecycleActionPolicyForNode(*node, modelName)
	if !policy.Eligible {
		return nil, policy, false
	}
	action := map[string]interface{}{
		"action_id":         "model_action_" + randomHex(6),
		"action_type":       modelLifecycleActionKeepWarm,
		"policy":            modelLifecycleActionPolicyManaged,
		"node_id":           node.NodeID,
		"model":             modelName,
		"desired_keep_warm": req.KeepWarm,
		"status":            modelLifecycleActionQueued,
		"requested_at":      now,
		"updated_at":        now,
		"claim_endpoint":    "/subscriber/model-actions/claim",
		"status_endpoint":   "/subscriber/model-actions/status",
	}
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["model_lifecycle_action_status"] = modelLifecycleActionQueued
	node.Observed["model_lifecycle_action_id"] = action["action_id"]
	node.Observed["model_lifecycle_action_updated_at"] = now
	node.Observed["model_lifecycle_actions"] = upsertModelLifecycleAction(node.Observed["model_lifecycle_actions"], action)
	return action, policy, true
}

func modelLifecycleActionPolicyForNode(node appstate.Node, modelName string) modelLifecycleActionPolicy {
	policy := modelLifecycleActionBasePolicyForNode(node)
	modelName = safeBenchmarkString(modelName)
	if modelName == "" {
		addModelLifecycleActionPolicyReason(&policy, "model_name_required", "A safe model name is required.")
	} else if !modelLifecycleNodeHasModel(node, modelName) {
		addModelLifecycleActionPolicyReason(&policy, "model_not_found", "The model must be present in subscriber-reported model inventory before a keep-warm action is queued.")
	}
	finalizeModelLifecycleActionPolicy(&policy)
	return policy
}

func modelLifecycleActionBasePolicyForNode(node appstate.Node) modelLifecycleActionPolicy {
	control := routingStatusControlLevel(node)
	approval := node.ApprovalState
	if approval == "" {
		if node.Approved {
			approval = appstate.ApprovalStateApproved
		} else {
			approval = appstate.ApprovalStatePending
		}
	}
	policy := modelLifecycleActionPolicy{
		ControlLevel:  control,
		TrustLevel:    node.TrustLevel,
		ApprovalState: approval,
	}
	if control != appstate.ControlLevelManaged {
		addModelLifecycleActionPolicyReason(&policy, "passive_no_model_management_control", "Passive Endpoints are inventory-only and cannot receive model lifecycle actions.")
	}
	if approval != appstate.ApprovalStateApproved || !node.Approved {
		addModelLifecycleActionPolicyReason(&policy, "node_not_approved", "Managed Node must be approved before model lifecycle actions are queued.")
	}
	if !node.Enabled {
		addModelLifecycleActionPolicyReason(&policy, "node_disabled", "Managed Node must be enabled before model lifecycle actions are queued.")
	}
	if node.Status != "" && node.Status != "healthy" {
		addModelLifecycleActionPolicyReason(&policy, "node_not_healthy", "Managed Node must be healthy before model lifecycle actions are queued.")
	}
	if !node.ManagementSupported {
		addModelLifecycleActionPolicyReason(&policy, "model_management_not_reported", "Managed Node must report model-management support through subscriber metadata before actions are queued.")
	}
	if !node.WarmStateSupported {
		addModelLifecycleActionPolicyReason(&policy, "warm_state_not_reported", "Managed Node must report warm-state support through subscriber metadata before keep-warm actions are queued.")
	}
	return policy
}

func addModelLifecycleActionPolicyReason(policy *modelLifecycleActionPolicy, reason, message string) {
	policy.ReasonCodes = append(policy.ReasonCodes, reason)
	if policy.Message == "" {
		policy.Message = message
	}
}

func finalizeModelLifecycleActionPolicy(policy *modelLifecycleActionPolicy) {
	if len(policy.ReasonCodes) == 0 {
		policy.Eligible = true
		policy.ReasonCodes = []string{"managed_subscriber_model_action_allowed"}
		policy.Message = "Managed Node can receive metadata-only subscriber model lifecycle actions."
	}
}

func (s *Server) modelLifecycleActionPolicies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.modelLifecycleActionPolicyStatus())
}

func (s *Server) modelLifecycleActionPolicyStatus() ModelLifecycleActionPolicyStatus {
	return summarizeModelLifecycleActionPolicies(s.store.Snapshot().Nodes)
}

func summarizeModelLifecycleActionPolicies(nodes map[string]appstate.Node) ModelLifecycleActionPolicyStatus {
	status := ModelLifecycleActionPolicyStatus{
		Window:  "current_model_lifecycle_action_policy",
		Summary: map[string]int{},
		Nodes:   []ModelLifecycleActionPolicyNode{},
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		item := modelLifecycleActionPolicyForStatus(nodes[id])
		status.Nodes = append(status.Nodes, item)
		status.Summary["nodes"]++
		status.Summary["models"] += item.ModelCount
		status.Summary["model_lifecycle_actions"] += item.ActionCount
		status.Summary["pending_model_lifecycle_actions"] += item.PendingActionCount
		if item.Eligible {
			status.Summary["eligible_nodes"]++
		} else {
			status.Summary["blocked_nodes"]++
		}
		if item.ControlLevel == appstate.ControlLevelPassive {
			status.Summary["passive_inventory_only_nodes"]++
		}
	}
	return status
}

func modelLifecycleActionPolicyForStatus(node appstate.Node) ModelLifecycleActionPolicyNode {
	policy := modelLifecycleActionBasePolicyForNode(node)
	models := sanitizeModelStates(node.Models)
	if len(models) == 0 && len(policy.ReasonCodes) == 0 {
		addModelLifecycleActionPolicyReason(&policy, "model_inventory_empty", "Managed Node must report at least one safe model before keep-warm actions can be queued.")
	}
	finalizeModelLifecycleActionPolicy(&policy)
	actionCount, pendingActionCount, lastActionStatus := modelLifecycleActionSummary(node)
	mode := "subscriber_claimed_model_actions"
	source := modelLifecycleSourceSubscriberReported
	supportedActions := []string{}
	if policy.Eligible {
		supportedActions = []string{modelLifecycleActionKeepWarm}
	}
	if policy.ControlLevel == appstate.ControlLevelPassive {
		mode = modelLifecycleModeInventoryOnly
		source = modelLifecycleSourceMarshalObserved
	}
	return ModelLifecycleActionPolicyNode{
		NodeID:             node.NodeID,
		ControlLevel:       policy.ControlLevel,
		TrustLevel:         policy.TrustLevel,
		ApprovalState:      policy.ApprovalState,
		Source:             source,
		Mode:               mode,
		Eligible:           policy.Eligible,
		SupportedActions:   supportedActions,
		ReasonCodes:        append([]string(nil), policy.ReasonCodes...),
		Message:            policy.Message,
		ModelCount:         len(models),
		ActionCount:        actionCount,
		PendingActionCount: pendingActionCount,
		LastActionStatus:   lastActionStatus,
	}
}

func modelLifecycleNodeHasModel(node appstate.Node, modelName string) bool {
	modelName = strings.ToLower(safeBenchmarkString(modelName))
	for _, model := range sanitizeModelStates(node.Models) {
		if strings.ToLower(model.Name) == modelName {
			return true
		}
	}
	return false
}

func (s *Server) subscriberModelActionClaim(w http.ResponseWriter, r *http.Request) {
	var body subscriberModelActionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.NodeID = strings.TrimSpace(body.NodeID)
	if body.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
		return
	}
	state := s.store.Snapshot()
	node, ok := state.Nodes[body.NodeID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model lifecycle actions are only supported for managed nodes"})
		return
	}
	if !s.verifySubscriberHeartbeatCredential(w, r, node) {
		return
	}
	action, ok := nextQueuedModelLifecycleAction(node)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "no_action", "action": nil})
		return
	}
	now := time.Now().UTC()
	action["status"] = modelLifecycleActionRunning
	action["claimed_at"] = now
	action["updated_at"] = now
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["model_lifecycle_action_status"] = modelLifecycleActionRunning
	node.Observed["model_lifecycle_action_id"] = action["action_id"]
	node.Observed["model_lifecycle_action_updated_at"] = now
	node.Observed["model_lifecycle_actions"] = upsertModelLifecycleAction(node.Observed["model_lifecycle_actions"], action)
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.tele.Emit("model_lifecycle_action_claimed", telemetry.Event{
		"node_id":        node.NodeID,
		"control_level":  node.ControlLevel,
		"trust_level":    node.TrustLevel,
		"approval_state": node.ApprovalState,
		"action_id":      action["action_id"],
		"action_type":    action["action_type"],
		"model":          action["model"],
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "action_claimed", "action": action})
}

func (s *Server) subscriberModelActionStatus(w http.ResponseWriter, r *http.Request) {
	var body subscriberModelActionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.NodeID = strings.TrimSpace(body.NodeID)
	body.ActionID = safeBenchmarkString(body.ActionID)
	status := safeModelLifecycleActionStatus(body.Status)
	if body.NodeID == "" || body.ActionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and action_id are required"})
		return
	}
	if status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid model action status is required"})
		return
	}
	state := s.store.Snapshot()
	node, ok := state.Nodes[body.NodeID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if node.ControlLevel != "" && node.ControlLevel != appstate.ControlLevelManaged {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model lifecycle actions are only supported for managed nodes"})
		return
	}
	if !s.verifySubscriberHeartbeatCredential(w, r, node) {
		return
	}
	action, ok := findModelLifecycleAction(node, body.ActionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model lifecycle action not found"})
		return
	}
	now := time.Now().UTC()
	action["status"] = status
	action["updated_at"] = now
	if body.ErrorCode != "" {
		action["error_code"] = safeModelLifecycleActionErrorCode(body.ErrorCode)
	}
	switch status {
	case modelLifecycleActionCompleted:
		action["completed_at"] = now
		applyCompletedModelLifecycleAction(&node, action, now)
	case modelLifecycleActionFailed:
		action["failed_at"] = now
		if body.ErrorCode == "" {
			action["error_code"] = "subscriber_reported_failed"
		}
	case modelLifecycleActionCancelled:
		action["completed_at"] = now
	}
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	node.Observed["model_lifecycle_action_status"] = status
	node.Observed["model_lifecycle_action_id"] = body.ActionID
	node.Observed["model_lifecycle_action_updated_at"] = now
	node.Observed["model_lifecycle_actions"] = upsertModelLifecycleAction(node.Observed["model_lifecycle_actions"], action)
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stored := s.store.Snapshot().Nodes[node.NodeID]
	s.tele.Emit("model_lifecycle_action_status", telemetry.Event{
		"node_id":        stored.NodeID,
		"control_level":  stored.ControlLevel,
		"trust_level":    stored.TrustLevel,
		"approval_state": stored.ApprovalState,
		"action_id":      body.ActionID,
		"action_type":    action["action_type"],
		"model":          action["model"],
		"status":         status,
		"error_code":     action["error_code"],
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "node": stored, "action": action})
}

func applyCompletedModelLifecycleAction(node *appstate.Node, action map[string]interface{}, now time.Time) {
	actionType := benchmarkJobString(action["action_type"])
	if actionType != modelLifecycleActionKeepWarm {
		return
	}
	modelName := safeBenchmarkString(benchmarkJobString(action["model"]))
	desired, ok := action["desired_keep_warm"].(bool)
	if !ok || modelName == "" {
		return
	}
	for i := range node.Models {
		if strings.EqualFold(node.Models[i].Name, modelName) {
			node.Models[i].KeepWarm = desired
			if desired && node.Models[i].State == "installed" {
				node.Models[i].State = "warm"
			}
			break
		}
	}
	applyModelLifecycleObserved(node, modelLifecycleSourceSubscriberReported, modelLifecycleModeRich, now)
}

func nextQueuedModelLifecycleAction(node appstate.Node) (map[string]interface{}, bool) {
	for _, action := range modelLifecycleActions(node.Observed["model_lifecycle_actions"]) {
		if benchmarkJobString(action["status"]) == modelLifecycleActionQueued {
			return cloneBenchmarkJob(action), true
		}
	}
	return nil, false
}

func findModelLifecycleAction(node appstate.Node, actionID string) (map[string]interface{}, bool) {
	for _, action := range modelLifecycleActions(node.Observed["model_lifecycle_actions"]) {
		if benchmarkJobString(action["action_id"]) == actionID {
			return cloneBenchmarkJob(action), true
		}
	}
	return nil, false
}

func upsertModelLifecycleAction(existing interface{}, action map[string]interface{}) []map[string]interface{} {
	action = cloneBenchmarkJob(action)
	id := benchmarkJobString(action["action_id"])
	out := []map[string]interface{}{action}
	for _, current := range modelLifecycleActions(existing) {
		if id != "" && benchmarkJobString(current["action_id"]) == id {
			continue
		}
		out = append(out, current)
	}
	if len(out) > maxModelLifecycleActions {
		return out[:maxModelLifecycleActions]
	}
	return out
}

func modelLifecycleActions(existing interface{}) []map[string]interface{} {
	return benchmarkJobs(existing)
}

func safeModelLifecycleActionStatus(status string) string {
	switch strings.TrimSpace(status) {
	case modelLifecycleActionQueued, modelLifecycleActionRunning, modelLifecycleActionCompleted, modelLifecycleActionFailed, modelLifecycleActionCancelled:
		return strings.TrimSpace(status)
	default:
		return ""
	}
}
