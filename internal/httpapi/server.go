package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
	"llama-wrangler/internal/config"
	"llama-wrangler/internal/detect"
	"llama-wrangler/internal/hec"
	"llama-wrangler/internal/ollama"
	"llama-wrangler/internal/routing"
	"llama-wrangler/internal/secrets"
	"llama-wrangler/internal/session"
	"llama-wrangler/internal/telemetry"
)

type Server struct {
	cfg                config.Config
	store              *appstate.Store
	scanner            detect.Scanner
	tele               *telemetry.Sink
	secrets            *secrets.Store
	recoveryAdminToken string
	queueScheduler     *queueScheduler
	queueMeta          *queueTracker
	authLimiter        *authFailureLimiter
	client             *http.Client
}

func NewServer(cfg config.Config) (*Server, error) {
	store, err := appstate.Open(cfg)
	if err != nil {
		return nil, err
	}
	cfg = store.Snapshot().Config
	secretStore, err := secrets.Open(store.Dir())
	if err != nil {
		return nil, err
	}
	if cfg.Telemetry.SplunkHEC.Token == "" {
		cfg.Telemetry.SplunkHEC.Token = secretStore.Get("splunk_hec_token")
	}
	server := &Server{
		cfg:            cfg,
		store:          store,
		tele:           telemetry.New(cfg, store),
		secrets:        secretStore,
		queueScheduler: newQueueScheduler(max(1, cfg.Routing.QueueMaxDepth), cfg.Routing),
		queueMeta:      newQueueTracker(max(1, cfg.Routing.QueueMaxDepth)),
		authLimiter: newAuthFailureLimiter(
			defaultAuthFailureLimit,
			defaultAuthFailureWindow,
			defaultAuthFailureCooldown,
		),
		client: &http.Client{Timeout: cfg.Routing.Timeout()},
	}
	recoveryToken, err := server.ensureAdminToken()
	if err != nil {
		return nil, err
	}
	server.recoveryAdminToken = recoveryToken
	state := store.Snapshot()
	if state.SetupComplete {
		cfg.UI.RequireAuth = true
		cfg.Auth.Enabled = true
		if err := store.SaveConfig(cfg); err != nil {
			return nil, err
		}
		server.cfg = cfg
		if _, _, err := server.ensureDefaultClientKey(); err != nil {
			return nil, err
		}
	}
	scanner := detect.New(server.cfg)
	server.scanner = scanner
	node := scanner.Local(context.Background())
	_ = store.UpsertNode(node)
	return server, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	s.routes(mux)
	server := &http.Server{Addr: s.cfg.Server.Listen, Handler: withCORS(mux)}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) RecoveryAdminToken() string {
	return s.recoveryAdminToken
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /ui", redirectUI)
	mux.HandleFunc("GET /ui/", s.ui)

	mux.HandleFunc("GET /wrangler/ui/bootstrap", s.bootstrap)
	mux.HandleFunc("GET /wrangler/ui/status", s.bootstrap)
	mux.HandleFunc("POST /wrangler/setup/start", s.requireAdmin(s.setupStart))
	mux.HandleFunc("POST /wrangler/setup/scan-local", s.requireAdmin(s.scanLocal))
	mux.HandleFunc("POST /wrangler/setup/detect-ollama", s.requireAdmin(s.scanLocal))
	mux.HandleFunc("POST /wrangler/setup/discover-peers", s.requireAdmin(s.discoverPeers))
	mux.HandleFunc("POST /wrangler/setup/apply-recommended", s.requireAdmin(s.applyRecommended))
	mux.HandleFunc("POST /wrangler/setup/test-ollama", s.requireAdmin(s.testOllama))
	mux.HandleFunc("POST /wrangler/setup/test-hec", s.requireAdmin(s.testHEC))
	mux.HandleFunc("POST /wrangler/setup/complete", s.requireAdmin(s.setupComplete))
	mux.HandleFunc("GET /wrangler/config", s.requireAdmin(s.getConfig))
	mux.HandleFunc("PUT /wrangler/config", s.requireAdmin(s.putConfig))
	mux.HandleFunc("POST /wrangler/config/export", s.requireAdmin(s.exportConfig))
	mux.HandleFunc("POST /wrangler/support-bundle/export", s.requireAdmin(s.exportSupportBundle))
	mux.HandleFunc("GET /wrangler/nodes", s.requireAdmin(s.nodes))
	mux.HandleFunc("POST /wrangler/nodes/manual-add", s.requireAdmin(s.manualAddNode))
	mux.HandleFunc("POST /wrangler/nodes/", s.requireAdmin(s.nodeAction))
	mux.HandleFunc("GET /wrangler/models", s.requireAdmin(s.models))
	mux.HandleFunc("GET /wrangler/aliases", s.requireAdmin(s.aliases))
	mux.HandleFunc("PUT /wrangler/aliases", s.requireAdmin(s.putAliases))
	mux.HandleFunc("GET /wrangler/routing/policies", s.requireAdmin(s.routingPolicies))
	mux.HandleFunc("PUT /wrangler/routing/policies", s.requireAdmin(s.putRoutingPolicies))
	mux.HandleFunc("GET /wrangler/client-presets", s.requireAdmin(s.clientPresets))
	mux.HandleFunc("GET /wrangler/auth/status", s.requireAdmin(s.authStatus))
	mux.HandleFunc("POST /wrangler/auth/admin-token/rotate", s.requireAdmin(s.rotateAdminTokenHandler))
	mux.HandleFunc("POST /wrangler/auth/api-keys", s.requireAdmin(s.createClientAPIKey))
	mux.HandleFunc("POST /wrangler/auth/api-keys/", s.requireAdmin(s.clientAPIKeyAction))
	mux.HandleFunc("POST /wrangler/secrets/rekey", s.requireAdmin(s.rekeySecretsHandler))
	mux.HandleFunc("GET /wrangler/telemetry/status", s.requireAdmin(s.telemetryStatus))
	mux.HandleFunc("PUT /wrangler/telemetry/splunk-hec", s.requireAdmin(s.putSplunkHEC))
	mux.HandleFunc("POST /wrangler/telemetry/test-hec", s.requireAdmin(s.testHEC))
	mux.HandleFunc("GET /wrangler/audit/recent", s.requireAdmin(s.audit))
	mux.HandleFunc("GET /wrangler/metrics", s.requireAdmin(s.metrics))

	mux.HandleFunc("GET /subscriber/capabilities", s.subscriberCapabilities)
	mux.HandleFunc("POST /subscriber/proxy/", s.subscriberProxy)

	mux.HandleFunc("GET /v1/models", s.requireClientAPIKey(s.openAIModels))
	mux.HandleFunc("POST /v1/chat/completions", s.requireClientAPIKey(s.marshalProxy("/v1/chat/completions", "openai_chat_completions")))
	mux.HandleFunc("POST /v1/completions", s.requireClientAPIKey(s.marshalProxy("/v1/completions", "openai_completions")))
	mux.HandleFunc("POST /v1/embeddings", s.requireClientAPIKey(s.marshalProxy("/v1/embeddings", "openai_embeddings")))
	mux.HandleFunc("GET /api/tags", s.requireClientAPIKey(s.apiTags))
	mux.HandleFunc("POST /api/chat", s.requireClientAPIKey(s.marshalProxy("/api/chat", "ollama_chat")))
	mux.HandleFunc("POST /api/generate", s.requireClientAPIKey(s.marshalProxy("/api/generate", "ollama_generate")))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"mode":   s.cfg.Server.Mode,
		"time":   time.Now().UTC(),
	})
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	state := s.store.Snapshot()
	if state.SetupComplete && state.Config.UI.RequireAuth && !s.validAdminToken(r) {
		token := bearerToken(r)
		if token != "" {
			if blocked, retryAfter := s.authLimiter.registerFailure(authScopeAdmin, r); blocked {
				setRetryAfter(w, retryAfter)
				writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
					"error":               "admin_auth_rate_limited",
					"message":             "Too many invalid admin token attempts. Wait before retrying.",
					"retry_after_seconds": int(retryAfter.Round(time.Second).Seconds()),
					"admin_token_hint":    state.AdminTokenHint,
				})
				return
			}
		}
		body := map[string]interface{}{
			"auth_required":      true,
			"setup_complete":     true,
			"admin_token_hint":   state.AdminTokenHint,
			"recovery_available": s.recoveryAdminToken != "" && isLoopbackRemote(r),
		}
		if s.recoveryAdminToken != "" && isLoopbackRemote(r) {
			body["recovery_admin_token"] = s.recoveryAdminToken
		}
		writeJSON(w, http.StatusUnauthorized, body)
		return
	}
	s.authLimiter.reset(authScopeAdmin, r)
	exposure := lanExposureForListen(state.Config.Server.Listen)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"setup_complete":    state.SetupComplete,
		"admin_token_hint":  state.AdminTokenHint,
		"schema_version":    state.SchemaVersion,
		"config_version":    state.ConfigVersion,
		"migration_history": state.MigrationHistory,
		"secret_storage":    s.secrets.Status(),
		"role":              state.Role,
		"node_id":           state.NodeID,
		"config":            sanitizeConfig(state.Config),
		"nodes":             state.Nodes,
		"sessions":          state.Sessions,
		"client_api_keys":   state.ClientAPIKeys,
		"client_presets":    s.buildClientPresets(r),
		"queue_depth":       s.queueDepth(),
		"queue":             s.queueSnapshot(),
		"operation_stats":   s.operationStats(),
		"auth_rate_limit":   s.authLimiter.metadata(),
		"safe_defaults": map[string]interface{}{
			"frontier_enabled":                 state.Config.Frontier.Enabled,
			"local_only":                       state.Config.Frontier.LocalOnly,
			"telemetry_level":                  state.Config.Telemetry.LoggingLevel,
			"prompt_body_logging":              state.Config.Telemetry.StorePayloads,
			"marshal_listen":                   state.Config.Server.Listen,
			"lan_access_by_default":            false,
			"lan_access_enabled":               exposure.Enabled,
			"lan_access_warning":               exposure.Warning,
			"lan_requires_explicit_enablement": true,
		},
		"telemetry": map[string]interface{}{
			"splunk_hec": s.splunkHECStatus(),
		},
	})
}

func (s *Server) setupStart(w http.ResponseWriter, r *http.Request) {
	s.tele.Emit("setup_event", telemetry.Event{"step": "start"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) scanLocal(w http.ResponseWriter, r *http.Request) {
	node := s.scanner.Local(r.Context())
	_ = s.store.UpsertNode(node)
	s.tele.Emit("node_capability_update", telemetry.Event{"node_id": node.NodeID, "status": node.Status, "ollama_available": node.OllamaAvailable})
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) discoverPeers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers":   []interface{}{},
		"message": "mDNS discovery is reserved for the next pass. Manual peer enrollment is available through node URLs.",
	})
}

func (s *Server) applyRecommended(w http.ResponseWriter, r *http.Request) {
	state := s.store.Snapshot()
	cfg := state.Config
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = config.Default(cfg.Server.Mode).ModelAliases
	}
	cfg.Frontier.Enabled = false
	cfg.Frontier.LocalOnly = true
	cfg.Telemetry.LoggingLevel = "metadata_only"
	cfg.Telemetry.StorePayloads = false
	cfg.Session.DefaultAffinity = "soft"
	_ = s.store.SaveConfig(cfg)
	s.cfg = cfg
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "applied",
		"recommendations": []string{
			"Use localhost binding until LAN access is explicitly enabled.",
			"Keep Frontier Delta disabled until a provider, redaction policy, and approval flow are configured.",
			"Use soft session affinity for IDE and chat clients.",
			"Prefer warm local models when routing latency-sensitive requests.",
		},
	})
}

func (s *Server) testOllama(w http.ResponseWriter, r *http.Request) {
	err := ollama.New(s.cfg.Ollama.URL).Health(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"status": "failed", "message": "Could not reach Ollama at " + s.cfg.Ollama.URL + ". Start Ollama, then click Retry.", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) testHEC(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Telemetry.SplunkHEC
	if r.Body != nil {
		var body splunkHECRequest
		if json.NewDecoder(r.Body).Decode(&body) == nil && body.URL != "" {
			cfg = body.toConfig(cfg)
		}
	}
	if cfg.Token == "" {
		cfg.Token = s.secrets.Get("splunk_hec_token")
	}
	client := hec.New(cfg)
	err := client.Send("config", map[string]interface{}{"event_type": "config", "status": "test", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"status": "failed", "message": friendlyHECError(err), "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) setupComplete(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot().Config
	cfg.UI.RequireAuth = true
	cfg.Auth.Enabled = true
	_ = s.store.SaveConfig(cfg)
	s.cfg = cfg
	clientKey, clientToken, err := s.ensureDefaultClientKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.SetSetupComplete(true)
	s.tele.Emit("setup_event", telemetry.Event{"step": "complete"})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":              "complete",
		"admin_token":         s.secrets.Get(adminSecretKey),
		"admin_token_hint":    tokenHint(s.secrets.Get(adminSecretKey)),
		"client_api_key":      clientToken,
		"client_api_key_id":   clientKey.ID,
		"client_api_key_hint": clientKey.Hint,
	})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sanitizeConfig(s.store.Snapshot().Config))
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = s.cfg.Server.Listen
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = s.cfg.Server.Mode
	}
	if err := s.store.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.cfg = cfg
	writeJSON(w, http.StatusOK, sanitizeConfig(cfg))
}

func (s *Server) exportConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sanitizeConfig(s.store.Snapshot().Config))
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Nodes)
}

func (s *Server) manualAddNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
		URL    string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.URL = strings.TrimRight(body.URL, "/")
	if body.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subscriber URL is required"})
		return
	}
	node, err := s.fetchSubscriberCapabilities(r.Context(), body.URL)
	if err != nil {
		node = appstate.Node{
			NodeID:      body.NodeID,
			DisplayName: body.NodeID,
			URL:         body.URL,
			Role:        "subscriber",
			Status:      "degraded",
			Enabled:     true,
			Approved:    true,
			LastSeen:    time.Now().UTC(),
			Observed: map[string]interface{}{
				"enrollment_warning": "Capabilities could not be fetched: " + err.Error(),
			},
		}
		if node.NodeID == "" {
			node.NodeID = "manual_" + randomHex(4)
			node.DisplayName = node.NodeID
		}
	} else {
		if body.NodeID != "" {
			node.NodeID = body.NodeID
		}
		node.URL = body.URL
		node.Role = "subscriber"
		node.Enabled = true
		node.Approved = true
		if node.Status == "" {
			node.Status = "healthy"
		}
	}
	if err := s.store.UpsertNode(node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.tele.Emit("node_enrollment", telemetry.Event{"node_id": node.NodeID, "url": body.URL, "status": node.Status})
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) fetchSubscriberCapabilities(ctx context.Context, baseURL string) (appstate.Node, error) {
	var node appstate.Node
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/subscriber/capabilities", nil)
	if err != nil {
		return node, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return node, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return node, fmt.Errorf("subscriber returned %s", resp.Status)
	}
	return node, json.NewDecoder(resp.Body).Decode(&node)
}

func (s *Server) nodeAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/wrangler/nodes/"), "/")
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node action required"})
		return
	}
	state := s.store.Snapshot()
	node, ok := state.Nodes[parts[0]]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	switch parts[1] {
	case "benchmark":
		node.Observed = map[string]interface{}{"benchmark_status": "queued", "updated_at": time.Now().UTC()}
	case "disable":
		node.Enabled = false
		node.Status = "disabled"
	case "enable":
		node.Enabled = true
		node.Status = "healthy"
	case "overrides":
		var patch appstate.Node
		_ = json.NewDecoder(r.Body).Decode(&patch)
		if patch.DisplayName != "" {
			node.DisplayName = patch.DisplayName
		}
		if patch.MaxJobs > 0 {
			node.MaxJobs = patch.MaxJobs
		}
		if patch.Tags != nil {
			node.Tags = patch.Tags
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown node action"})
		return
	}
	_ = s.store.UpsertNode(node)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	models := map[string][]string{}
	for _, node := range s.store.Snapshot().Nodes {
		for _, model := range node.Models {
			models[model.Name] = append(models[model.Name], node.NodeID)
		}
	}
	writeJSON(w, http.StatusOK, models)
}

func (s *Server) aliases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Config.ModelAliases)
}

func (s *Server) putAliases(w http.ResponseWriter, r *http.Request) {
	var aliases map[string]config.ModelAlias
	if err := json.NewDecoder(r.Body).Decode(&aliases); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cfg := s.store.Snapshot().Config
	cfg.ModelAliases = aliases
	_ = s.store.SaveConfig(cfg)
	s.cfg = cfg
	writeJSON(w, http.StatusOK, aliases)
}

func (s *Server) routingPolicies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Config.Routing)
}

func (s *Server) putRoutingPolicies(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Snapshot().Config
	if err := json.NewDecoder(r.Body).Decode(&cfg.Routing); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	config.Normalize(&cfg)
	_ = s.store.SaveConfig(cfg)
	s.cfg = cfg
	s.queueScheduler.configure(max(1, cfg.Routing.QueueMaxDepth), cfg.Routing)
	s.queueMeta.configure(max(1, cfg.Routing.QueueMaxDepth))
	writeJSON(w, http.StatusOK, cfg.Routing)
}

func (s *Server) telemetryStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"json_logs":     s.cfg.Telemetry.JSONLogs,
		"logging_level": s.cfg.Telemetry.LoggingLevel,
		"splunk_hec":    s.splunkHECStatus(),
	})
}

func (s *Server) splunkHECStatus() map[string]interface{} {
	hec := s.cfg.Telemetry.SplunkHEC
	status := map[string]interface{}{
		"enabled":                   hec.Enabled,
		"url":                       hec.URL,
		"index":                     hec.Index,
		"source":                    hec.Source,
		"verify_ssl":                hec.VerifySSL,
		"has_token":                 s.secrets.Get("splunk_hec_token") != "" || hec.Token != "",
		"sourcetype":                hec.Prefix,
		"last_status":               "available_in_audit",
		"tls_verification_disabled": !hec.VerifySSL,
		"tls_warning":               "",
	}
	if !hec.VerifySSL {
		status["tls_warning"] = "TLS certificate verification is disabled. Use this only for trusted self-signed Splunk lab certificates."
	}
	return status
}

func (s *Server) putSplunkHEC(w http.ResponseWriter, r *http.Request) {
	var body splunkHECRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cfg := s.store.Snapshot().Config
	cfg.Telemetry.SplunkHEC = body.toConfig(cfg.Telemetry.SplunkHEC)
	if body.Token != "" && body.Token != "********" {
		if err := s.secrets.Set("splunk_hec_token", body.Token); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		cfg.Telemetry.SplunkHEC.Token = body.Token
	}
	if err := s.store.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.cfg = cfg
	s.tele = telemetry.New(cfg, s.store)
	writeJSON(w, http.StatusOK, sanitizeConfig(cfg).Telemetry.SplunkHEC)
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Audit)
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	state := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":           len(state.Nodes),
		"sessions":        len(state.Sessions),
		"queue_depth":     s.queueDepth(),
		"queue":           s.queueSnapshot(),
		"operation_stats": s.operationStats(),
		"audit_count":     len(state.Audit),
	})
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	state := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"setup_complete":      state.SetupComplete,
		"schema_version":      state.SchemaVersion,
		"config_version":      state.ConfigVersion,
		"secret_storage":      s.secrets.Status(),
		"ui_auth_required":    state.Config.UI.RequireAuth,
		"client_auth_enabled": state.Config.Auth.Enabled,
		"auth_rate_limit":     s.authLimiter.metadata(),
		"admin_token_hint":    state.AdminTokenHint,
		"client_api_keys":     state.ClientAPIKeys,
	})
}

func (s *Server) rotateAdminTokenHandler(w http.ResponseWriter, r *http.Request) {
	token, err := s.rotateAdminToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.tele.Emit("admin_token_rotated", telemetry.Event{"admin_token_hint": tokenHint(token)})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"hint":  tokenHint(token),
	})
}

func (s *Server) rekeySecretsHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.secrets.Rekey()
	if err != nil {
		status := http.StatusInternalServerError
		code := "secret_rekey_failed"
		message := "Unable to rotate the encrypted local secrets key."
		if errors.Is(err, secrets.ErrRekeyUnsupported) {
			status = http.StatusConflict
			code = "secret_rekey_unsupported"
			message = "Local encrypted fallback rekey is unavailable for the current key source."
		}
		writeJSON(w, status, map[string]interface{}{
			"error":          code,
			"message":        message,
			"secret_storage": result.Status,
		})
		return
	}
	s.tele.Emit("secret_store_rekeyed", telemetry.Event{
		"backend":    result.Status.Backend,
		"key_source": result.Status.KeySource,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createClientAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Name) == "" {
		body.Name = "local-ide"
	}
	token := newToken("lw_client_")
	id := "key_" + randomHex(6)
	key := appstate.ClientAPIKey{
		ID:      id,
		Name:    strings.TrimSpace(body.Name),
		Hint:    tokenHint(token),
		Enabled: true,
	}
	if err := s.secrets.Set(apiKeyPrefix+key.ID, token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.UpsertClientAPIKey(key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg := s.store.Snapshot().Config
	cfg.Auth.Enabled = true
	if err := s.store.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.cfg = cfg
	s.tele.Emit("api_key_created", telemetry.Event{"key_id": key.ID, "key_name": key.Name})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":    key.ID,
		"name":  key.Name,
		"hint":  key.Hint,
		"token": token,
	})
}

func (s *Server) clientAPIKeyAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/wrangler/auth/api-keys/"), "/")
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client API key action required"})
		return
	}
	keyID := parts[0]
	action := parts[1]
	state := s.store.Snapshot()
	var key appstate.ClientAPIKey
	found := false
	for _, existing := range state.ClientAPIKeys {
		if existing.ID == keyID {
			key = existing
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "client API key not found"})
		return
	}
	switch action {
	case "revoke":
		key.Enabled = false
		if err := s.secrets.Delete(apiKeyPrefix + key.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.UpsertClientAPIKey(key); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.tele.Emit("api_key_revoked", telemetry.Event{"key_id": key.ID, "key_name": key.Name})
		writeJSON(w, http.StatusOK, key)
	case "rotate":
		token := newToken("lw_client_")
		key.Hint = tokenHint(token)
		key.Enabled = true
		if err := s.secrets.Set(apiKeyPrefix+key.ID, token); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.UpsertClientAPIKey(key); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.tele.Emit("api_key_rotated", telemetry.Event{"key_id": key.ID, "key_name": key.Name})
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":    key.ID,
			"name":  key.Name,
			"hint":  key.Hint,
			"token": token,
		})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown client API key action"})
	}
}

func (s *Server) subscriberCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.scanner.Local(r.Context()))
}

func (s *Server) subscriberProxy(w http.ResponseWriter, r *http.Request) {
	targetPath := strings.TrimPrefix(r.URL.Path, "/subscriber/proxy")
	if targetPath == "" || targetPath == "/" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "proxy target path required"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	resp, err := ollama.New(s.cfg.Ollama.URL).Proxy(r.Context(), targetPath, body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error(), "message": "Could not reach Ollama at " + s.cfg.Ollama.URL + "."})
		return
	}
	_ = ollama.CopyResponse(w, resp)
}

func (s *Server) openAIModels(w http.ResponseWriter, r *http.Request) {
	data := []map[string]interface{}{}
	for name := range s.cfg.ModelAliases {
		data = append(data, map[string]interface{}{"id": name, "object": "model", "owned_by": "llama-wrangler"})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"object": "list", "data": data})
}

func (s *Server) apiTags(w http.ResponseWriter, r *http.Request) {
	models := []map[string]interface{}{}
	seen := map[string]bool{}
	for name := range s.cfg.ModelAliases {
		seen[name] = true
		models = append(models, map[string]interface{}{"name": name, "details": map[string]string{"family": "alias"}})
	}
	for _, node := range s.store.Snapshot().Nodes {
		for _, model := range node.Models {
			if !seen[model.Name] {
				seen[model.Name] = true
				models = append(models, map[string]interface{}{"name": model.Name})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": models})
}

func (s *Server) marshalProxy(path, surface string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := "req_" + randomHex(8)
		start := time.Now()
		queueStatus := queueStatusCompleted
		if !s.acquireQueue(w, r, requestID, QueueEntry{RequestID: requestID, Surface: surface, Priority: queuePriorityFromHeader(r)}) {
			return
		}
		defer func() {
			s.queueScheduler.release()
			entry := s.finishQueue(requestID, queueStatus)
			s.tele.Emit("queue_state", telemetry.Event{"request_id": requestID, "status": entry.Status, "queue_depth": s.queueDepth(), "queue_capacity": entry.QueueCap, "priority": entry.Priority, "surface": entry.Surface, "model_requested": entry.Model, "stream": entry.Stream, "scheduling_policy": s.queueSchedulingStatus().Policy})
		}()

		body, _ := io.ReadAll(r.Body)
		model, stream, sessionID := parseRequestMeta(body, r)
		priority := parseRequestPriority(body, r)
		s.updateQueueMetadata(requestID, QueueEntry{Priority: priority, Surface: surface, Model: model, Stream: stream, SessionID: sessionID})
		decision, ok := routing.Select(s.cfg, s.store.Snapshot(), routing.Request{Model: model, Streaming: stream, SessionID: sessionID})
		if !ok {
			queueStatus = queueStatusFailed
			s.tele.Emit("error", telemetry.Event{"request_id": requestID, "error_class": "no_eligible_node", "model_requested": model, "retryable": false})
			writeInferenceError(w, r, http.StatusBadGateway, errorCodeNoEligibleNode, "No eligible Ollama node is available for the requested model.")
			return
		}
		decision.SelectedNode = session.ApplyAffinity(s.store, sessionID, decision.Affinity, decision.SelectedNode, decision.ResolvedModel, requestID)
		s.tele.Emit("request", telemetry.Event{"request_id": requestID, "api_surface": surface, "model_requested": model, "model_alias": decision.ModelAlias, "execution_mode": decision.ExecutionMode, "stream": stream, "session_id": sessionID})
		s.tele.Emit("routing_decision", telemetry.Event{"request_id": requestID, "model_alias": decision.ModelAlias, "resolved_model": decision.ResolvedModel, "selected_node": decision.SelectedNode, "candidate_nodes": decision.CandidateNodes, "fallback_nodes": decision.FallbackNodes, "routing_strategy": decision.Strategy, "routing_reasons": decision.Reasons, "execution_mode": decision.ExecutionMode})

		outcome := s.forwardWithFallback(r.Context(), w, requestID, path, body, decision, stream)
		if outcome.Err != nil {
			queueStatus = queueStatusFailed
			if outcome.ClientCancelled {
				queueStatus = queueStatusCancelled
			} else if outcome.PartialOutput {
				queueStatus = queueStatusPartial
			}
			if outcome.ClientCancelled || outcome.ResponseCommitted {
				return
			}
			s.tele.Emit("error", telemetry.Event{"request_id": requestID, "error_class": "upstream_unavailable", "error_message": outcome.Err.Error(), "retryable": true, "fallback_used": outcome.FallbackUsed})
			writeInferenceError(w, r, http.StatusBadGateway, errorCodeUpstreamUnavailable, "Upstream Ollama node is unavailable.")
			return
		}
		s.tele.Emit("response", telemetry.Event{"request_id": requestID, "status": "success", "selected_node": outcome.SelectedNode, "resolved_model": decision.ResolvedModel, "latency_ms": time.Since(start).Milliseconds(), "fallback_used": outcome.FallbackUsed, "retry_count": outcome.RetryCount, "bytes_written": outcome.BytesWritten, "frontier_used": false})
	}
}

type proxyOutcome struct {
	SelectedNode      string
	FallbackUsed      bool
	RetryCount        int
	BytesWritten      int64
	ResponseCommitted bool
	PartialOutput     bool
	ClientCancelled   bool
	RetryAfterPartial bool
	Err               error
}

type copyOutcome struct {
	BytesWritten      int64
	ResponseCommitted bool
	Err               error
}

func (s *Server) forwardWithFallback(ctx context.Context, w http.ResponseWriter, requestID, path string, body []byte, decision routing.Decision, stream bool) proxyOutcome {
	nodes := append([]string{decision.SelectedNode}, decision.FallbackNodes...)
	var lastErr error
	for i, nodeID := range nodes {
		url := s.nodeProxyURL(nodeID, path)
		if url == "" {
			lastErr = fmt.Errorf("node %s has no proxy URL", nodeID)
			s.emitRetryIfAvailable(requestID, stream, nodes, i, nodeID, "missing_proxy_url")
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rewriteModel(body, decision.ResolvedModel)))
		if err != nil {
			return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, Err: err}
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				s.tele.Emit("request_cancelled", telemetry.Event{"request_id": requestID, "selected_node": nodeID, "reason": "client_cancelled_before_first_token", "stream": stream, "partial_output": false, "retry_allowed": false})
				return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, ClientCancelled: true, Err: ctx.Err()}
			}
			lastErr = err
			s.emitRetryIfAvailable(requestID, stream, nodes, i, nodeID, "upstream_request_error")
			continue
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			code, message := normalizedUpstreamError(resp.StatusCode)
			resp.Body.Close()
			writeInferenceErrorForPath(w, path, resp.StatusCode, code, message)
			return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, RetryCount: i, ResponseCommitted: true, Err: fmt.Errorf("%s returned %s", nodeID, resp.Status)}
		}
		if resp.StatusCode < 400 {
			result := copyUpstreamResponse(w, resp, stream)
			if result.Err == nil {
				return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, RetryCount: i, BytesWritten: result.BytesWritten, ResponseCommitted: result.ResponseCommitted}
			}
			if ctx.Err() != nil {
				reason := "client_cancelled_before_first_token"
				if result.BytesWritten > 0 {
					reason = "client_cancelled_after_partial_output"
				}
				s.tele.Emit("request_cancelled", telemetry.Event{"request_id": requestID, "selected_node": nodeID, "reason": reason, "stream": stream, "bytes_written": result.BytesWritten, "partial_output": result.BytesWritten > 0, "retry_allowed": false})
				return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, RetryCount: i, BytesWritten: result.BytesWritten, ResponseCommitted: result.ResponseCommitted, PartialOutput: result.BytesWritten > 0, ClientCancelled: true, Err: ctx.Err()}
			}
			if result.BytesWritten > 0 || result.ResponseCommitted {
				s.tele.Emit("response_partial", telemetry.Event{"request_id": requestID, "selected_node": nodeID, "stream": stream, "bytes_written": result.BytesWritten, "retry_allowed": false, "retry_phase": "after_partial_output", "error_message": result.Err.Error()})
				return proxyOutcome{SelectedNode: nodeID, FallbackUsed: i > 0, RetryCount: i, BytesWritten: result.BytesWritten, ResponseCommitted: result.ResponseCommitted, PartialOutput: result.BytesWritten > 0, RetryAfterPartial: false, Err: result.Err}
			}
			lastErr = result.Err
			s.emitRetryIfAvailable(requestID, stream, nodes, i, nodeID, "upstream_body_read_error")
			continue
		}
		lastErr = fmt.Errorf("%s returned %s", nodeID, resp.Status)
		resp.Body.Close()
		s.emitRetryIfAvailable(requestID, stream, nodes, i, nodeID, "upstream_status_5xx")
	}
	return proxyOutcome{FallbackUsed: len(nodes) > 1, Err: lastErr}
}

func (s *Server) emitRetryIfAvailable(requestID string, stream bool, nodes []string, index int, nodeID, reason string) {
	if index+1 >= len(nodes) {
		return
	}
	s.tele.Emit("upstream_retry", telemetry.Event{
		"request_id":     requestID,
		"previous_node":  nodeID,
		"next_node":      nodes[index+1],
		"reason":         reason,
		"retry_phase":    "before_first_token",
		"retry_allowed":  true,
		"partial_output": false,
		"stream":         stream,
	})
}

func copyUpstreamResponse(w http.ResponseWriter, resp *http.Response, stream bool) copyOutcome {
	defer resp.Body.Close()
	if !stream {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return copyOutcome{Err: err}
		}
		copyResponseHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		n, err := w.Write(body)
		flushResponse(w)
		return copyOutcome{BytesWritten: int64(n), ResponseCommitted: true, Err: err}
	}

	buf := make([]byte, 32*1024)
	n, err := resp.Body.Read(buf)
	if n == 0 && err != nil {
		if err == io.EOF {
			copyResponseHeaders(w, resp)
			w.WriteHeader(resp.StatusCode)
			flushResponse(w)
			return copyOutcome{ResponseCommitted: true}
		}
		return copyOutcome{Err: err}
	}

	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	written, writeErr := w.Write(buf[:n])
	bytesWritten := int64(written)
	flushResponse(w)
	if writeErr != nil {
		return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: writeErr}
	}
	if written != n {
		return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: io.ErrShortWrite}
	}
	if err == io.EOF {
		return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true}
	}
	if err != nil {
		return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: err}
	}

	for {
		n, err = resp.Body.Read(buf)
		if n > 0 {
			written, writeErr = w.Write(buf[:n])
			bytesWritten += int64(written)
			flushResponse(w)
			if writeErr != nil {
				return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: writeErr}
			}
			if written != n {
				return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: io.ErrShortWrite}
			}
		}
		if err == io.EOF {
			return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true}
		}
		if err != nil {
			return copyOutcome{BytesWritten: bytesWritten, ResponseCommitted: true, Err: err}
		}
	}
}

func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
}

func flushResponse(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) queueSnapshot() QueueSnapshot {
	return s.queueMeta.snapshot(s.queueDepth(), s.queueSchedulingStatus())
}

func (s *Server) queueDepth() int {
	return s.queueScheduler.activeDepth()
}

func (s *Server) queueSchedulingStatus() QueueSchedulingStatus {
	return s.queueScheduler.status()
}

func (s *Server) nodeProxyURL(nodeID, path string) string {
	state := s.store.Snapshot()
	if node, ok := state.Nodes[nodeID]; ok && node.URL != "" {
		return strings.TrimRight(node.URL, "/") + "/subscriber/proxy" + path
	} else if ok {
		return strings.TrimRight(s.cfg.Ollama.URL, "/") + path
	}
	for _, sub := range s.cfg.Subscribers {
		if sub.NodeID == nodeID {
			return strings.TrimRight(sub.URL, "/") + "/subscriber/proxy" + path
		}
	}
	local := state.Nodes[state.NodeID]
	if nodeID == state.NodeID || nodeID == local.NodeID {
		return s.cfg.Ollama.URL + path
	}
	return ""
}

func (s *Server) acquireQueue(w http.ResponseWriter, r *http.Request, requestID string, entry QueueEntry) bool {
	entry.RequestID = requestID
	entry = s.queueMeta.enqueue(entry)
	scheduling := s.queueSchedulingStatus()
	s.tele.Emit("queue_state", telemetry.Event{"request_id": requestID, "status": queueStatusWaiting, "queue_depth": s.queueDepth(), "queue_capacity": entry.QueueCap, "priority": entry.Priority, "surface": entry.Surface, "scheduling_policy": scheduling.Policy})
	waitCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.queueScheduler.acquire(waitCtx, requestID, entry.Priority) {
		entry = s.queueMeta.start(requestID, s.queueDepth())
		s.tele.Emit("queue_state", telemetry.Event{"request_id": requestID, "queue_depth": s.queueDepth(), "queue_capacity": entry.QueueCap, "status": queueStatusActive, "priority": entry.Priority, "surface": entry.Surface, "scheduling_policy": scheduling.Policy})
		return true
	}
	if r.Context().Err() != nil {
		entry = s.queueMeta.finish(requestID, queueStatusCancelled, s.queueDepth())
		s.tele.Emit("request_cancelled", telemetry.Event{"request_id": requestID, "reason": "client_disconnect_before_queue", "queue_status": entry.Status, "priority": entry.Priority, "scheduling_policy": scheduling.Policy})
		return false
	}
	entry = s.queueMeta.finish(requestID, queueStatusRejected, s.queueDepth())
	writeInferenceError(w, r, http.StatusTooManyRequests, errorCodeQueueFull, "Request queue is full. Try again when active jobs complete.")
	s.tele.Emit("error", telemetry.Event{"request_id": requestID, "error_class": "queue_full", "queue_depth": s.queueDepth(), "queue_capacity": entry.QueueCap, "priority": entry.Priority, "retryable": true, "scheduling_policy": scheduling.Policy})
	return false
}

func (s *Server) updateQueueMetadata(requestID string, entry QueueEntry) QueueEntry {
	return s.queueMeta.update(requestID, entry)
}

func (s *Server) finishQueue(requestID, status string) QueueEntry {
	return s.queueMeta.finish(requestID, status, s.queueDepth())
}

func parseRequestMeta(body []byte, r *http.Request) (string, bool, string) {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)
	model, _ := payload["model"].(string)
	stream, _ := payload["stream"].(bool)
	sessionID := r.Header.Get("X-Llama-Wrangler-Session")
	if sessionID == "" {
		if v, ok := payload["session_id"].(string); ok {
			sessionID = v
		}
	}
	return model, stream, sessionID
}

func parseRequestPriority(body []byte, r *http.Request) string {
	if priority := r.Header.Get("X-Llama-Wrangler-Priority"); priority != "" {
		return normalizeQueuePriority(priority)
	}
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) == nil {
		if priority, ok := payload["priority"].(string); ok {
			return normalizeQueuePriority(priority)
		}
		if priority, ok := payload["queue_priority"].(string); ok {
			return normalizeQueuePriority(priority)
		}
	}
	return queuePriorityNormal
}

func rewriteModel(body []byte, model string) []byte {
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil || model == "" {
		return body
	}
	payload["model"] = model
	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func sanitizeConfig(cfg config.Config) config.Config {
	for i := range cfg.Auth.APIKeys {
		if cfg.Auth.APIKeys[i].Key != "" {
			cfg.Auth.APIKeys[i].Key = "********"
		}
	}
	if cfg.Telemetry.SplunkHEC.Token != "" {
		cfg.Telemetry.SplunkHEC.Token = "********"
	}
	return cfg
}

type splunkHECRequest struct {
	Enabled   bool   `json:"enabled"`
	URL       string `json:"url"`
	Token     string `json:"token"`
	TokenEnv  string `json:"token_env"`
	Index     string `json:"index"`
	Source    string `json:"source"`
	VerifySSL bool   `json:"verify_ssl"`
	Prefix    string `json:"sourcetype_prefix"`
}

func (r splunkHECRequest) toConfig(base config.SplunkHECConfig) config.SplunkHECConfig {
	base.Enabled = r.Enabled
	base.URL = r.URL
	if r.Token != "" && r.Token != "********" {
		base.Token = r.Token
	}
	base.TokenEnv = r.TokenEnv
	base.Index = r.Index
	base.Source = r.Source
	base.VerifySSL = r.VerifySSL
	base.Prefix = r.Prefix
	if base.Index == "" {
		base.Index = "llama_wrangler"
	}
	if base.Source == "" {
		base.Source = "llama-wrangler"
	}
	if base.Prefix == "" {
		base.Prefix = "llama_wrangler"
	}
	return base
}

func friendlyHECError(err error) string {
	text := err.Error()
	if strings.Contains(text, "403") {
		return "Splunk rejected the HEC token. Check that the token is enabled and allowed to write to the selected index."
	}
	if strings.Contains(text, "connection refused") {
		return "Could not reach Splunk HEC. Check the URL and confirm the HEC receiver is enabled."
	}
	return "Splunk HEC test failed. Check the URL, token, SSL settings, and index permissions."
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
