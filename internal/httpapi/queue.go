package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"llama-wrangler/internal/config"
)

const (
	queuePriorityHigh   = "high"
	queuePriorityNormal = "normal"
	queuePriorityLow    = "low"

	queueSchedulingWeightedPriority = "weighted_priority"
	queueSchedulingFIFO             = "fifo"

	queueStatusWaiting   = "waiting"
	queueStatusActive    = "active"
	queueStatusCompleted = "completed"
	queueStatusFailed    = "failed"
	queueStatusCancelled = "cancelled"
	queueStatusRejected  = "rejected"
	queueStatusPartial   = "partial"
)

type queueTracker struct {
	mu       sync.RWMutex
	capacity int
	current  map[string]QueueEntry
	recent   []QueueEntry
}

type queueScheduler struct {
	mu       sync.Mutex
	capacity int
	active   int
	policy   string
	weights  config.QueuePriorityWeights
	schedule []string
	cursor   int
	waiting  []*queueWaiter
}

type queueWaiter struct {
	requestID  string
	priority   string
	ready      chan struct{}
	dispatched bool
}

type QueueEntry struct {
	RequestID    string    `json:"request_id"`
	Priority     string    `json:"priority"`
	Status       string    `json:"status"`
	Surface      string    `json:"surface,omitempty"`
	Model        string    `json:"model,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	Stream       bool      `json:"stream"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	QueueDepth   int       `json:"queue_depth"`
	QueueCap     int       `json:"queue_capacity"`
	RetryAllowed bool      `json:"retry_allowed"`
}

type QueueSnapshot struct {
	MaxDepth   int                   `json:"max_depth"`
	Active     int                   `json:"active"`
	Waiting    int                   `json:"waiting"`
	Available  int                   `json:"available"`
	Current    []QueueEntry          `json:"current"`
	Recent     []QueueEntry          `json:"recent"`
	Priorities []string              `json:"priorities"`
	Scheduling QueueSchedulingStatus `json:"scheduling"`
}

type QueueSchedulingStatus struct {
	Policy  string                      `json:"policy"`
	Weights config.QueuePriorityWeights `json:"weights"`
}

func newQueueTracker(capacity int) *queueTracker {
	if capacity <= 0 {
		capacity = 1
	}
	return &queueTracker{
		capacity: capacity,
		current:  map[string]QueueEntry{},
		recent:   []QueueEntry{},
	}
}

func (q *queueTracker) configure(capacity int) {
	if capacity <= 0 {
		capacity = 1
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.capacity = capacity
	for id, entry := range q.current {
		entry.QueueCap = capacity
		q.current[id] = entry
	}
}

func newQueueScheduler(capacity int, routing config.RoutingConfig) *queueScheduler {
	s := &queueScheduler{}
	s.configure(capacity, routing)
	return s
}

func (s *queueScheduler) configure(capacity int, routing config.RoutingConfig) {
	if capacity <= 0 {
		capacity = 1
	}
	policy := normalizeQueueSchedulingPolicy(routing.QueueSchedulingPolicy)
	weights := normalizeQueuePriorityWeights(routing.QueuePriorityWeights)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capacity = capacity
	s.policy = policy
	s.weights = weights
	s.schedule = buildQueuePrioritySchedule(weights)
	if len(s.schedule) == 0 {
		s.schedule = []string{queuePriorityHigh, queuePriorityNormal, queuePriorityLow}
	}
	if s.cursor >= len(s.schedule) {
		s.cursor = 0
	}
	s.dispatchLocked()
}

func (s *queueScheduler) acquire(ctx context.Context, requestID, priority string) bool {
	priority = normalizeQueuePriority(priority)
	waiter := &queueWaiter{requestID: requestID, priority: priority, ready: make(chan struct{})}
	s.mu.Lock()
	if s.active < s.capacity && len(s.waiting) == 0 {
		s.active++
		s.mu.Unlock()
		return true
	}
	s.waiting = append(s.waiting, waiter)
	s.mu.Unlock()

	select {
	case <-waiter.ready:
		return true
	case <-ctx.Done():
		s.mu.Lock()
		defer s.mu.Unlock()
		if waiter.dispatched {
			return true
		}
		for i, candidate := range s.waiting {
			if candidate == waiter {
				s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
				break
			}
		}
		return false
	}
}

func (s *queueScheduler) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active > 0 {
		s.active--
	}
	s.dispatchLocked()
}

func (s *queueScheduler) activeDepth() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *queueScheduler) status() QueueSchedulingStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return QueueSchedulingStatus{Policy: s.policy, Weights: s.weights}
}

func (s *queueScheduler) dispatchLocked() {
	for s.active < s.capacity && len(s.waiting) > 0 {
		index := s.nextWaiterIndexLocked()
		waiter := s.waiting[index]
		s.waiting = append(s.waiting[:index], s.waiting[index+1:]...)
		waiter.dispatched = true
		s.active++
		close(waiter.ready)
	}
}

func (s *queueScheduler) nextWaiterIndexLocked() int {
	if s.policy == queueSchedulingFIFO || len(s.schedule) == 0 {
		return 0
	}
	for tries := 0; tries < len(s.schedule); tries++ {
		priority := s.schedule[s.cursor%len(s.schedule)]
		s.cursor = (s.cursor + 1) % len(s.schedule)
		for i, waiter := range s.waiting {
			if waiter.priority == priority {
				return i
			}
		}
	}
	return 0
}

func normalizeQueuePriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case queuePriorityHigh:
		return queuePriorityHigh
	case queuePriorityLow:
		return queuePriorityLow
	default:
		return queuePriorityNormal
	}
}

func normalizeQueueSchedulingPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case queueSchedulingFIFO:
		return queueSchedulingFIFO
	default:
		return queueSchedulingWeightedPriority
	}
}

func normalizeQueuePriorityWeights(weights config.QueuePriorityWeights) config.QueuePriorityWeights {
	if weights.High <= 0 {
		weights.High = 3
	}
	if weights.Normal <= 0 {
		weights.Normal = 2
	}
	if weights.Low <= 0 {
		weights.Low = 1
	}
	return weights
}

func buildQueuePrioritySchedule(weights config.QueuePriorityWeights) []string {
	weights = normalizeQueuePriorityWeights(weights)
	schedule := make([]string, 0, weights.High+weights.Normal+weights.Low)
	for i := 0; i < weights.High; i++ {
		schedule = append(schedule, queuePriorityHigh)
	}
	for i := 0; i < weights.Normal; i++ {
		schedule = append(schedule, queuePriorityNormal)
	}
	for i := 0; i < weights.Low; i++ {
		schedule = append(schedule, queuePriorityLow)
	}
	return schedule
}

func queuePriorityFromHeader(r *http.Request) string {
	return normalizeQueuePriority(r.Header.Get("X-Llama-Wrangler-Priority"))
}

func (q *queueTracker) enqueue(entry QueueEntry) QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry.Priority = normalizeQueuePriority(entry.Priority)
	entry.Status = queueStatusWaiting
	entry.EnqueuedAt = time.Now().UTC()
	entry.QueueCap = q.capacity
	q.current[entry.RequestID] = entry
	return entry
}

func (q *queueTracker) start(requestID string, depth int) QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry := q.current[requestID]
	entry.Status = queueStatusActive
	entry.StartedAt = time.Now().UTC()
	entry.QueueDepth = depth
	entry.QueueCap = q.capacity
	entry.RetryAllowed = true
	q.current[requestID] = entry
	return entry
}

func (q *queueTracker) update(requestID string, fields QueueEntry) QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry := q.current[requestID]
	if fields.Priority != "" {
		entry.Priority = normalizeQueuePriority(fields.Priority)
	}
	if fields.Surface != "" {
		entry.Surface = fields.Surface
	}
	if fields.Model != "" {
		entry.Model = fields.Model
	}
	if fields.SessionID != "" {
		entry.SessionID = fields.SessionID
	}
	entry.Stream = fields.Stream
	q.current[requestID] = entry
	return entry
}

func (q *queueTracker) finish(requestID, status string, depth int) QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry := q.current[requestID]
	if entry.RequestID == "" {
		entry.RequestID = requestID
		entry.Priority = queuePriorityNormal
		entry.EnqueuedAt = time.Now().UTC()
		entry.QueueCap = q.capacity
	}
	entry.Status = status
	entry.CompletedAt = time.Now().UTC()
	entry.QueueDepth = depth
	entry.QueueCap = q.capacity
	delete(q.current, requestID)
	q.recent = append([]QueueEntry{entry}, q.recent...)
	if len(q.recent) > 50 {
		q.recent = q.recent[:50]
	}
	return entry
}

func (q *queueTracker) snapshot(activeDepth int, scheduling QueueSchedulingStatus) QueueSnapshot {
	q.mu.RLock()
	defer q.mu.RUnlock()
	current := make([]QueueEntry, 0, len(q.current))
	waiting := 0
	for _, entry := range q.current {
		current = append(current, entry)
		if entry.Status == queueStatusWaiting {
			waiting++
		}
	}
	recent := make([]QueueEntry, len(q.recent))
	copy(recent, q.recent)
	available := q.capacity - activeDepth
	if available < 0 {
		available = 0
	}
	return QueueSnapshot{
		MaxDepth:  q.capacity,
		Active:    activeDepth,
		Waiting:   waiting,
		Available: available,
		Current:   current,
		Recent:    recent,
		Priorities: []string{
			queuePriorityHigh,
			queuePriorityNormal,
			queuePriorityLow,
		},
		Scheduling: scheduling,
	}
}
