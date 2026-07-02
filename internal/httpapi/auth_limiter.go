package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	authScopeAdmin  = "admin"
	authScopeClient = "client"

	defaultAuthFailureLimit    = 5
	defaultAuthFailureWindow   = time.Minute
	defaultAuthFailureCooldown = 30 * time.Second
)

type authFailureLimiter struct {
	mu       sync.Mutex
	attempts map[string]authFailureAttempt
	max      int
	window   time.Duration
	cooldown time.Duration
	now      func() time.Time
}

type authFailureAttempt struct {
	Count        int
	FirstSeen    time.Time
	BlockedUntil time.Time
}

func newAuthFailureLimiter(max int, window, cooldown time.Duration) *authFailureLimiter {
	if max <= 0 {
		max = defaultAuthFailureLimit
	}
	if window <= 0 {
		window = defaultAuthFailureWindow
	}
	if cooldown <= 0 {
		cooldown = defaultAuthFailureCooldown
	}
	return &authFailureLimiter{
		attempts: map[string]authFailureAttempt{},
		max:      max,
		window:   window,
		cooldown: cooldown,
		now:      time.Now,
	}
}

func (l *authFailureLimiter) registerFailure(scope string, r *http.Request) (bool, time.Duration) {
	if l == nil {
		return false, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	key := authFailureKey(scope, r)
	attempt := l.attempts[key]
	if !attempt.BlockedUntil.IsZero() && now.Before(attempt.BlockedUntil) {
		return true, attempt.BlockedUntil.Sub(now)
	}
	if attempt.FirstSeen.IsZero() || now.Sub(attempt.FirstSeen) > l.window {
		attempt = authFailureAttempt{FirstSeen: now}
	}
	attempt.Count++
	if attempt.Count >= l.max {
		attempt.BlockedUntil = now.Add(l.cooldown)
		l.attempts[key] = attempt
		return true, l.cooldown
	}
	l.attempts[key] = attempt
	return false, 0
}

func (l *authFailureLimiter) reset(scope string, r *http.Request) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, authFailureKey(scope, r))
}

func (l *authFailureLimiter) metadata() map[string]interface{} {
	if l == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"max_failures":     l.max,
		"window_seconds":   int(l.window.Seconds()),
		"cooldown_seconds": int(l.cooldown.Seconds()),
	}
}

func authFailureKey(scope string, r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}
	return scope + ":" + host
}

func setRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
