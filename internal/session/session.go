package session

import (
	"llama-wrangler/internal/appstate"
)

func ApplyAffinity(store *appstate.Store, sessionID, affinity string, decisionNode, model, requestID string) string {
	if sessionID == "" || store == nil || affinity == "none" {
		return decisionNode
	}
	existing, ok := store.Session(sessionID)
	if ok && existing.NodeID != "" && (affinity == "soft" || affinity == "strict" || affinity == "task") {
		if affinity == "strict" || affinity == "task" {
			decisionNode = existing.NodeID
		}
	}
	_ = store.UpdateSession(appstate.Session{
		SessionID:     sessionID,
		AffinityMode:  affinity,
		NodeID:        decisionNode,
		Model:         model,
		LastRequestID: requestID,
	})
	return decisionNode
}
