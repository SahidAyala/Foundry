package model

import (
	"fmt"
	"sync"
	"time"
)

// Status is a model's runtime health state (ADR-0013) — distinct from
// Info's static catalog metadata (Capabilities/Limits/Quality), which
// never changes at runtime. Status is reported by a caller (e.g. an
// Executor observing a real failure or recovery) and read via
// Registry.Health/HealthManager.Get, and — since ADR-0013's
// post-ratification "HealthManager connected" note — by automatic model
// failover, to deprioritize a currently-unavailable candidate before ever
// attempting it; see HealthManager's own doc comment and Health.Unavailable.
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
// engine.Router.ModelHealth reads it, and automatic model failover
// (engine/failover.go's preferHealthyCandidates, ADR-0013's
// post-ratification "HealthManager connected" note) uses it to
// deprioritize — never to hard-exclude — a candidate currently reported
// Unavailable, before ever attempting it, not only after a real Execute
// call to it fails. Safe for concurrent use, since Foundry already
// supports concurrent Acts (worktree isolation) whose Executors could
// plausibly report health at the same time.
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

// Unavailable reports whether h describes a model that should currently
// be treated as down — StatusUnavailable or StatusCooldown, and not yet
// past its own reported RetryAt. StatusAvailable and StatusUnknown are
// never Unavailable. A zero RetryAt means "no retry time given," per this
// struct's own doc comment, not "retry immediately" — so an Unavailable/
// Cooldown report with no RetryAt stays Unavailable indefinitely, until a
// fresh Report says otherwise; only a non-zero RetryAt that has actually
// passed lifts it automatically, without needing a new Report call.
//
// This is what ADR-0013's own "connect HealthManager to failover" follow-up
// (see engine/failover.go's preferHealthyCandidates) reads to decide
// whether a candidate should be deprioritized — a soft signal, since a
// Report can go stale, never a hard exclusion the way Capabilities are.
func (h Health) Unavailable() bool {
	switch h.Status {
	case StatusUnavailable, StatusCooldown:
		return h.RetryAt.IsZero() || time.Now().Before(h.RetryAt)
	default:
		return false
	}
}
