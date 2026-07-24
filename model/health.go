package model

import (
	"fmt"
	"sync"
	"time"
)

// Status is a model's runtime health state (ADR-0013, Proposed, fifth
// increment) — distinct from Info's static catalog metadata
// (Capabilities/Limits/Quality), which never changes at runtime. Status is
// reported by a caller (e.g. an Executor observing a real failure or
// recovery) and only ever read via Registry.Health/HealthManager.Get;
// nothing in Foundry's execution path reads it yet, and no automatic
// failover or routing decision consults it — see HealthManager's own doc
// comment.
type Status string

const (
	StatusAvailable   Status = "AVAILABLE"
	StatusUnavailable Status = "UNAVAILABLE"
	StatusCooldown    Status = "COOLDOWN"
	StatusUnknown     Status = "UNKNOWN"
)

// Health is one model's current runtime health: its Status plus optional
// explanatory metadata. Reason is a free-text explanation (e.g. "rate
// limited", "401 unauthorized") — empty means none was given. RetryAt
// names when a COOLDOWN or UNAVAILABLE status is expected to lift; the
// zero time.Time means "no retry time given," not "retry immediately."
type Health struct {
	Status  Status
	Reason  string
	RetryAt time.Time
}

// HealthManager tracks runtime Health per model ID, in memory, for the
// lifetime of the process. It is deliberately a separate, small component
// from Registry's own static catalog (Info/Capabilities/Limits/Quality):
// health is mutable, observed state; catalog metadata is fixed,
// hand-curated data. Register/Get/List/ByExecutor on Registry are
// entirely unaffected by whether a HealthManager is attached.
//
// This is metadata only, the same "expose now, consume later" shape
// ADR-0013's third increment (Capabilities/Limits/Quality) already
// established: nothing in Foundry's execution path (engine.Router.Resolve,
// session.Session, engine.Engine) reads a HealthManager's state yet, and
// no automatic failover exists — a model reported UNAVAILABLE is not
// skipped, retried elsewhere, or treated any differently by Resolve; it is
// purely observable state a future, separately-decided increment could act
// on. Safe for concurrent use, since Foundry already supports concurrent
// Acts (worktree isolation) whose Executors could plausibly report health
// at the same time.
type HealthManager struct {
	mu     sync.RWMutex
	health map[string]Health
}

// NewHealthManager returns an empty HealthManager. Every model queried
// before its first Report defaults to Health{Status: StatusUnknown} (see
// Get) — an unreported model's health is genuinely unknown, not a lookup
// failure.
func NewHealthManager() *HealthManager {
	return &HealthManager{health: make(map[string]Health)}
}

// Report records health for the model named id, overwriting whatever was
// previously recorded. This is deliberately a plain "set current state"
// operation, not an append-only log or a state machine with enforced
// transitions — nothing stops a caller from reporting StatusCooldown
// straight after StatusAvailable; the caller (e.g. an Executor observing a
// real failure or recovery) owns the meaning of any transition. Refuses an
// empty id, mirroring Registry.Register's own validation discipline.
func (h *HealthManager) Report(id string, health Health) error {
	if id == "" {
		return fmt.Errorf("model: health: model has no ID")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.health[id] = health
	return nil
}

// Get returns id's currently recorded Health, or Health{Status:
// StatusUnknown} if nothing has ever been Report-ed for it — never an
// error, since "no report yet" is itself a legitimate, expected state
// (unknown), not a lookup failure.
func (h *HealthManager) Get(id string) Health {
	h.mu.RLock()
	defer h.mu.RUnlock()
	health, ok := h.health[id]
	if !ok {
		return Health{Status: StatusUnknown}
	}
	return health
}
