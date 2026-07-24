package engine

import (
	"fmt"

	"foundry/model"
)

// Router resolves the Executor a Step's Generate work runs against
// (RFC-0004 §2, docs/04-guides/multi-executor-router-implementation-plan.md
// Piece 1). Its policy: a Step's Model, if set, resolved through a
// model.Registry to an Executor name (ADR-0013, Proposed); otherwise a
// Step's explicit executor pin; otherwise the Engine's default Executor —
// no capability matching against advertised properties. That is RFC-0002 §7
// layer 2, deliberately out of scope until a real multi-Executor Pipeline in
// production motivates it. Model resolution is additive on top of that same
// policy, not a new one: once a name is decided (from Model or Executor), it
// is looked up in registry exactly as before — no new routing decision.
type Router struct {
	registry *ExecutorRegistry
	def      Executor
	models   *model.Registry
}

// NewRouter returns a Router that resolves a Step's pinned Executor out of
// registry, falling back to def when a Step declares no pin. def is never
// nil in practice — it is always the Engine's own configured Executor,
// preserving the exact behavior every Step had before Router existed. The
// returned Router has no model.Registry configured — WithModels attaches
// one — so every existing caller keeps resolving Executor-only Steps
// exactly as before Model existed.
func NewRouter(registry *ExecutorRegistry, def Executor) Router {
	return Router{registry: registry, def: def}
}

// WithModels returns a copy of r with its model.Registry set to models,
// enabling Step.Model resolution (ADR-0013). models may be nil — the same
// as never calling WithModels at all — for a caller that has no Model
// Registry to offer.
func (r Router) WithModels(models *model.Registry) Router {
	r.models = models
	return r
}

// Resolve returns the Executor step.Kind's Generate work should run
// against.
//
// If step.Model is set, it wins over step.Executor: it is looked up in r's
// model.Registry to find which Executor vendor it belongs to, and that
// name is what gets resolved against r's registry below — a clear, named
// error if step.Model is set but no model.Registry is configured, or the
// model itself is not registered ("unknown model should fail validation";
// no fallback to step.Executor or the default is ever attempted).
//
// Otherwise, step.Executor is resolved: a clear, named error if it is set
// but not registered in r's registry — a pin that can't be honored is
// never silently ignored in favor of the default — or r's default Executor
// if step.Executor is also unset, exactly what every Step meant before
// Router existed.
func (r Router) Resolve(step Step) (Executor, error) {
	name := step.Executor
	if step.Model != "" {
		if r.models == nil {
			return nil, fmt.Errorf("engine: step %q: model %q is set, but no model registry is configured", step.ID, step.Model)
		}
		info, err := r.models.Get(step.Model)
		if err != nil {
			return nil, fmt.Errorf("engine: step %q: %w", step.ID, err)
		}
		name = info.Executor
	}
	if name == "" {
		return r.def, nil
	}
	e, err := r.registry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("engine: step %q: %w", step.ID, err)
	}
	return e, nil
}
