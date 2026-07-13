package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
)

const (
	modelLifecycleSourceSubscriberReported = "subscriber_reported"
	modelLifecycleSourceMarshalObserved    = "marshal_observed"
	modelLifecycleModeRich                 = "rich_lifecycle_metadata"
	modelLifecycleModeInventoryOnly        = "inventory_only"
)

type ModelLifecycleStatus struct {
	Window  string               `json:"window"`
	Summary map[string]int       `json:"summary"`
	Nodes   []ModelLifecycleNode `json:"nodes"`
}

type ModelLifecycleNode struct {
	NodeID               string                `json:"node_id"`
	ControlLevel         string                `json:"control_level"`
	TrustLevel           string                `json:"trust_level"`
	Source               string                `json:"source"`
	Mode                 string                `json:"mode"`
	WarmStateSupported   bool                  `json:"warm_state_supported"`
	ManagementSupported  bool                  `json:"management_supported"`
	ModelInventorySource string                `json:"model_inventory_source"`
	ModelCount           int                   `json:"model_count"`
	WarmCount            int                   `json:"warm_count"`
	LoadedCount          int                   `json:"loaded_count"`
	LoadingCount         int                   `json:"loading_count"`
	InstalledCount       int                   `json:"installed_count"`
	KeepWarmCount        int                   `json:"keep_warm_count"`
	FailedCount          int                   `json:"failed_count"`
	ActionCount          int                   `json:"action_count"`
	PendingActionCount   int                   `json:"pending_action_count"`
	LastActionStatus     string                `json:"last_action_status,omitempty"`
	UpdatedAt            string                `json:"updated_at,omitempty"`
	Message              string                `json:"message"`
	ReasonCodes          []string              `json:"reason_codes"`
	Models               []ModelLifecycleModel `json:"models"`
}

type ModelLifecycleModel struct {
	Name       string  `json:"name"`
	State      string  `json:"state"`
	KeepWarm   bool    `json:"keep_warm"`
	TokensSec  float64 `json:"tokens_per_second,omitempty"`
	LoadTimeMS int     `json:"load_time_ms,omitempty"`
}

func (s *Server) modelLifecycle(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.modelLifecycleStatus())
}

func (s *Server) modelLifecycleStatus() ModelLifecycleStatus {
	return summarizeModelLifecycle(s.store.Snapshot().Nodes)
}

func summarizeModelLifecycle(nodes map[string]appstate.Node) ModelLifecycleStatus {
	status := ModelLifecycleStatus{
		Window:  "current_model_lifecycle",
		Summary: map[string]int{},
		Nodes:   []ModelLifecycleNode{},
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		item := modelLifecycleForNode(nodes[id])
		status.Nodes = append(status.Nodes, item)
		status.Summary["nodes"]++
		status.Summary["models"] += item.ModelCount
		status.Summary["warm_models"] += item.WarmCount
		status.Summary["loaded_models"] += item.LoadedCount
		status.Summary["loading_models"] += item.LoadingCount
		status.Summary["installed_models"] += item.InstalledCount
		status.Summary["keep_warm_models"] += item.KeepWarmCount
		status.Summary["failed_models"] += item.FailedCount
		status.Summary["model_lifecycle_actions"] += item.ActionCount
		status.Summary["pending_model_lifecycle_actions"] += item.PendingActionCount
		if item.ControlLevel == appstate.ControlLevelPassive {
			status.Summary["passive_inventory_only_nodes"]++
		}
		if item.WarmStateSupported {
			status.Summary["warm_state_supported_nodes"]++
		}
	}
	return status
}

func modelLifecycleForNode(node appstate.Node) ModelLifecycleNode {
	control := routingStatusControlLevel(node)
	source := modelLifecycleSourceSubscriberReported
	mode := modelLifecycleModeRich
	message := "Managed Node reports model lifecycle and warm-state metadata through subscriber heartbeat."
	reasons := []string{"managed_lifecycle_subscriber_reported"}
	if control == appstate.ControlLevelPassive {
		source = modelLifecycleSourceMarshalObserved
		mode = modelLifecycleModeInventoryOnly
		message = "Passive Endpoint model inventory is marshal-observed from /api/tags; warm-state and local model control are unavailable."
		reasons = []string{"passive_inventory_only", "warm_state_unavailable", "model_management_unavailable"}
	}
	models := sanitizeModelStates(node.Models)
	summary := modelLifecycleSummaryForModels(models)
	actionCount, pendingActionCount, lastActionStatus := modelLifecycleActionSummary(node)
	item := ModelLifecycleNode{
		NodeID:               node.NodeID,
		ControlLevel:         control,
		TrustLevel:           node.TrustLevel,
		Source:               source,
		Mode:                 mode,
		WarmStateSupported:   node.WarmStateSupported && control != appstate.ControlLevelPassive,
		ManagementSupported:  node.ManagementSupported && control != appstate.ControlLevelPassive,
		ModelInventorySource: node.ModelInventorySource,
		ModelCount:           len(models),
		WarmCount:            summary["warm"],
		LoadedCount:          summary["loaded"],
		LoadingCount:         summary["loading"],
		InstalledCount:       summary["installed"],
		KeepWarmCount:        summary["keep_warm"],
		FailedCount:          summary["failed"],
		ActionCount:          actionCount,
		PendingActionCount:   pendingActionCount,
		LastActionStatus:     lastActionStatus,
		Message:              message,
		ReasonCodes:          reasons,
		Models:               modelLifecycleModels(models),
	}
	if control == appstate.ControlLevelPassive {
		item.WarmCount = 0
		item.LoadingCount = 0
		item.KeepWarmCount = 0
	}
	if node.LastReportedAt != nil && control != appstate.ControlLevelPassive {
		item.UpdatedAt = node.LastReportedAt.UTC().Format(time.RFC3339)
	} else if node.LastObservedAt != nil {
		item.UpdatedAt = node.LastObservedAt.UTC().Format(time.RFC3339)
	}
	return item
}

func modelLifecycleActionSummary(node appstate.Node) (int, int, string) {
	if node.Observed == nil {
		return 0, 0, ""
	}
	actions := modelLifecycleActions(node.Observed["model_lifecycle_actions"])
	pending := 0
	lastStatus := ""
	for i, action := range actions {
		status := benchmarkJobString(action["status"])
		if i == 0 {
			lastStatus = status
		}
		if status == modelLifecycleActionQueued || status == modelLifecycleActionRunning {
			pending++
		}
	}
	return len(actions), pending, lastStatus
}

func modelLifecycleModels(models []appstate.ModelState) []ModelLifecycleModel {
	out := make([]ModelLifecycleModel, 0, len(models))
	for _, model := range models {
		out = append(out, ModelLifecycleModel{
			Name:       model.Name,
			State:      model.State,
			KeepWarm:   model.KeepWarm,
			TokensSec:  model.TokensSec,
			LoadTimeMS: model.LoadTimeMS,
		})
	}
	return out
}

func sanitizeModelStates(models []appstate.ModelState) []appstate.ModelState {
	if models == nil {
		return nil
	}
	out := make([]appstate.ModelState, 0, len(models))
	seen := map[string]bool{}
	for _, model := range models {
		name := safeBenchmarkString(model.Name)
		if name == "" {
			continue
		}
		state := normalizeModelLifecycleState(model.State)
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		clean := appstate.ModelState{
			Name:     name,
			State:    state,
			KeepWarm: model.KeepWarm,
		}
		if model.TokensSec > 0 && model.TokensSec <= 1000000 {
			clean.TokensSec = model.TokensSec
		}
		if model.LoadTimeMS > 0 && model.LoadTimeMS <= 3600000 {
			clean.LoadTimeMS = model.LoadTimeMS
		}
		out = append(out, clean)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func normalizeModelLifecycleState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "installed", "loading", "loaded", "warm", "busy", "unloaded", "evicted", "failed":
		return strings.ToLower(strings.TrimSpace(state))
	case "":
		return "installed"
	default:
		return "unknown"
	}
}

func modelLifecycleSummaryForModels(models []appstate.ModelState) map[string]int {
	summary := map[string]int{}
	for _, model := range models {
		state := normalizeModelLifecycleState(model.State)
		summary[state]++
		if state == "warm" || state == "loaded" || state == "busy" {
			summary["loaded"]++
		}
		if model.KeepWarm {
			summary["keep_warm"]++
		}
	}
	return summary
}

func applyModelLifecycleObserved(node *appstate.Node, source string, mode string, now time.Time) {
	if node.Observed == nil {
		node.Observed = map[string]interface{}{}
	}
	summary := modelLifecycleSummaryForModels(node.Models)
	node.Observed["model_lifecycle_source"] = source
	node.Observed["model_lifecycle_mode"] = mode
	node.Observed["model_lifecycle_updated_at"] = now
	node.Observed["model_lifecycle_summary"] = summary
	node.Observed["model_count"] = len(node.Models)
	node.Observed["warm_model_count"] = summary["warm"]
	node.Observed["keep_warm_count"] = summary["keep_warm"]
	node.Observed["warm_state_supported"] = node.WarmStateSupported
}
