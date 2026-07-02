package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	"llama-wrangler/internal/appstate"
)

const (
	adminSecretKey = "admin_token"
	apiKeyPrefix   = "api_key:"
)

func newToken(prefix string) string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return prefix + randomHex(16)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf)
}

func tokenHint(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if token := r.Header.Get("X-Llama-Wrangler-Admin-Token"); token != "" {
		return token
	}
	return ""
}

func (s *Server) ensureAdminToken() (string, error) {
	existing := s.secrets.Get(adminSecretKey)
	if existing != "" {
		if s.store.Snapshot().AdminTokenHint == "" {
			_ = s.store.SetAdminTokenHint(tokenHint(existing))
		}
		return "", nil
	}
	token := newToken("lw_admin_")
	if err := s.secrets.Set(adminSecretKey, token); err != nil {
		return "", err
	}
	_ = s.store.SetAdminTokenHint(tokenHint(token))
	return token, nil
}

func (s *Server) ensureDefaultClientKey() (appstate.ClientAPIKey, string, error) {
	state := s.store.Snapshot()
	for _, key := range state.ClientAPIKeys {
		if key.Name == "local-ide" && key.Enabled && s.secrets.Get(apiKeyPrefix+key.ID) != "" {
			return key, "", nil
		}
	}
	token := newToken("lw_client_")
	key := appstate.ClientAPIKey{
		ID:      "local-ide",
		Name:    "local-ide",
		Hint:    tokenHint(token),
		Enabled: true,
	}
	if err := s.secrets.Set(apiKeyPrefix+key.ID, token); err != nil {
		return key, "", err
	}
	if err := s.store.UpsertClientAPIKey(key); err != nil {
		return key, "", err
	}
	return key, token, nil
}

func (s *Server) rotateAdminToken() (string, error) {
	token := newToken("lw_admin_")
	if err := s.secrets.Set(adminSecretKey, token); err != nil {
		return "", err
	}
	_ = s.store.SetAdminTokenHint(tokenHint(token))
	return token, nil
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}
		state := s.store.Snapshot()
		if !state.SetupComplete || !state.Config.UI.RequireAuth {
			next(w, r)
			return
		}
		if s.validAdminToken(r) {
			s.authLimiter.reset(authScopeAdmin, r)
			next(w, r)
			return
		}
		if bearerToken(r) != "" {
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
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error":            "admin_auth_required",
			"message":          "Enter the local admin token generated during setup.",
			"admin_token_hint": state.AdminTokenHint,
		})
	}
}

func (s *Server) validAdminToken(r *http.Request) bool {
	candidate := bearerToken(r)
	return s.secrets.Match(adminSecretKey, candidate)
}

func (s *Server) requireClientAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := s.store.Snapshot()
		if !state.SetupComplete || !state.Config.Auth.Enabled {
			next(w, r)
			return
		}
		token := bearerToken(r)
		if s.validClientAPIKey(r, state, token) {
			s.authLimiter.reset(authScopeClient, r)
			next(w, r)
			return
		}
		if token != "" {
			if blocked, retryAfter := s.authLimiter.registerFailure(authScopeClient, r); blocked {
				setRetryAfter(w, retryAfter)
				writeInferenceError(w, r, http.StatusTooManyRequests, errorCodeClientAuthRateLimited, "Too many invalid Llama Wrangler client API key attempts. Wait before retrying.")
				return
			}
		}
		writeInferenceError(w, r, http.StatusUnauthorized, errorCodeClientKeyRequired, "Provide a Llama Wrangler client API key in the Authorization header.")
	}
}

func (s *Server) validClientAPIKey(r *http.Request, state appstate.State, token string) bool {
	for _, key := range state.ClientAPIKeys {
		if !key.Enabled {
			continue
		}
		stored := s.secrets.Get(apiKeyPrefix + key.ID)
		if stored != "" && subtle.ConstantTimeCompare([]byte(stored), []byte(token)) == 1 {
			s.store.TouchClientAPIKey(key.ID)
			return true
		}
	}
	for _, key := range state.Config.Auth.APIKeys {
		if key.Key != "" && subtle.ConstantTimeCompare([]byte(key.Key), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func isLoopbackRemote(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip == nil || ip.IsLoopback()
}
