package httpapi

import (
	"sort"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/routing"
)

type RoutingPolicyStatus struct {
	Window   string                 `json:"window"`
	Summary  map[string]int         `json:"summary"`
	Warnings []RoutingPolicyWarning `json:"warnings"`
}

type RoutingPolicyWarning struct {
	NodeID           string `json:"node_id"`
	Severity         string `json:"severity"`
	Scope            string `json:"scope"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	ControlLevel     string `json:"control_level"`
	TrustLevel       string `json:"trust_level"`
	ApprovalState    string `json:"approval_state"`
	CapabilitySource string `json:"capability_source,omitempty"`
}

func (s *Server) routingPolicyStatus() RoutingPolicyStatus {
	return summarizeRoutingPolicy(s.store.Snapshot().Nodes, time.Now().UTC())
}

func summarizeRoutingPolicy(nodes map[string]appstate.Node, now time.Time) RoutingPolicyStatus {
	status := RoutingPolicyStatus{
		Window:   "current_node_metadata",
		Summary:  map[string]int{},
		Warnings: []RoutingPolicyWarning{},
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		node := nodes[id]
		status.Warnings = append(status.Warnings, routingWarningsForNode(node, now)...)
	}
	for _, warning := range status.Warnings {
		status.Summary["warnings"]++
		status.Summary[warning.Severity]++
		status.Summary[warning.Scope]++
		status.Summary[warning.Code]++
	}
	return status
}

func routingWarningsForNode(node appstate.Node, now time.Time) []RoutingPolicyWarning {
	warnings := []RoutingPolicyWarning{}
	add := func(severity, scope, code, message string) {
		warnings = append(warnings, RoutingPolicyWarning{
			NodeID:           node.NodeID,
			Severity:         severity,
			Scope:            scope,
			Code:             code,
			Message:          message,
			ControlLevel:     routingStatusControlLevel(node),
			TrustLevel:       routingStatusTrustLevel(node),
			ApprovalState:    routingStatusApprovalState(node),
			CapabilitySource: node.CapabilitySource,
		})
	}
	if !node.Enabled {
		add("blocked", "routing_and_consensus", "node_disabled", "Node is disabled and excluded from routing and consensus.")
	}
	if !routingStatusApproved(node) {
		add("blocked", "routing_and_consensus", "node_not_approved", "Node is not approved and is excluded from routing and consensus until approved.")
	}
	if node.Status == "disabled" || node.Status == "failed" {
		add("blocked", "routing_and_consensus", "node_status_"+node.Status, "Node health status excludes it from routing and consensus.")
	}
	if routingStatusHeartbeatRequired(node) {
		switch routingStatusHeartbeatFreshness(node, now) {
		case "missing":
			add("blocked", "routing_and_consensus", "heartbeat_missing", "Managed Node requires heartbeat but has not reported yet.")
		case "stale":
			add("blocked", "routing_and_consensus", "heartbeat_stale", "Managed Node heartbeat is stale and routing pauses until a fresh report arrives.")
		}
	}
	if routingStatusTrustLevel(node) == appstate.TrustLevelExternal {
		add("blocked", "routing_and_consensus", "trust_external_excluded", "External trust is excluded by default unless an explicit future policy allows it.")
	}
	if routingStatusControlLevel(node) == appstate.ControlLevelPassive {
		add("limited", "consensus", "passive_consensus_excluded", "Passive Endpoint can be used for approved single-route requests but is excluded from consensus.")
	}
	if routingStatusTrustLevel(node) == appstate.TrustLevelLANUnverified {
		add("limited", "routing", "trust_lan_unverified_deprioritized", "LAN unverified endpoint remains eligible for approved single-route requests but is de-prioritized.")
		add("blocked", "consensus", "trust_lan_unverified_consensus_excluded", "LAN unverified endpoint is excluded from consensus by default.")
	}
	return warnings
}

func routingStatusControlLevel(node appstate.Node) string {
	if node.ControlLevel == appstate.ControlLevelPassive {
		return appstate.ControlLevelPassive
	}
	return appstate.ControlLevelManaged
}

func routingStatusTrustLevel(node appstate.Node) string {
	switch node.TrustLevel {
	case appstate.TrustLevelLANTrusted, appstate.TrustLevelLANUnverified, appstate.TrustLevelExternal:
		return node.TrustLevel
	default:
		return appstate.TrustLevelLocal
	}
}

func routingStatusApprovalState(node appstate.Node) string {
	if node.ApprovalState != "" {
		return node.ApprovalState
	}
	if node.Approved {
		return appstate.ApprovalStateApproved
	}
	return appstate.ApprovalStatePending
}

func routingStatusApproved(node appstate.Node) bool {
	return routingStatusApprovalState(node) == appstate.ApprovalStateApproved && node.Approved
}

func routingStatusHeartbeatRequired(node appstate.Node) bool {
	if routingStatusControlLevel(node) != appstate.ControlLevelManaged || node.Observed == nil {
		return false
	}
	required, ok := node.Observed["heartbeat_required"].(bool)
	return ok && required
}

func routingStatusHeartbeatFreshness(node appstate.Node, now time.Time) string {
	if node.LastReportedAt == nil || node.LastReportedAt.IsZero() {
		return "missing"
	}
	if now.Sub(node.LastReportedAt.UTC()) > routing.ManagedHeartbeatFreshnessWindow {
		return "stale"
	}
	return "fresh"
}
