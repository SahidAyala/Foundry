package engine

import (
	"fmt"

	"github.com/SahidAyala/Foundry/model"
)

// Router resolves the Executor a Step's Generate work runs against
// (RFC-0004 §2, docs/04-guides/multi-executor-router-implementation-plan.md
// Piece 1). Its policy, in precedence order: a Step's Preferred list's
// first entry, if set; otherwise its Model, if set — either resolved
// through a model.Registry to an Executor name (ADR-0013, Proposed);
// otherwise a Step's explicit executor pin; otherwise the Engine's default
// Executor — no capability matching against advertised properties. That is
// RFC-0002 §7 layer 2, deliberately out of scope until a real
// multi-Executor Pipeline in production motivates it. Model/Preferred
// resolution is additive on top of that same policy, not a new one: once a
// name is decided (from Preferred, Model, or Executor), it is looked up in
// registry exactly as before — no new routing decision.
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
// enabling Step.Model/Step.Preferred resolution (ADR-0013). models may be
// nil — the same as never calling WithModels at all — for a caller that
// has no Model Registry to offer.
func (r Router) WithModels(models *model.Registry) Router {
	r.models = models
	return r
}

// Resolve returns the Executor step.Kind's Generate work should run
// against.
//
// If step.Preferred is non-empty, its first entry wins over
// everything else — Resolve picks Preferred[0] with no availability check
// of any kind (it never probes whether that model is actually reachable,
// never falls back to a later Preferred entry, and never retries): the
// rest of the list is inert today, reserved for a future,
// separately-decided availability-aware increment.
//
// Otherwise, if step.Model is set, it wins over step.Executor.
//
// Either way, the chosen Model ID is looked up in r's model.Registry to
// find which Executor vendor it belongs to, and that name is what gets
// resolved against r's registry below — a clear, named error if a Model ID
// is chosen but no model.Registry is configured, or the model itself is
// not registered ("unknown model should fail validation"; no fallback to
// a later Preferred entry, step.Executor, or the default is ever
// attempted).
//
// Otherwise, step.Executor is resolved: a clear, named error if it is set
// but not registered in r's registry — a pin that can't be honored is
// never silently ignored in favor of the default — or r's default Executor
// if step.Executor is also unset, exactly what every Step meant before
// Router existed.
func (r Router) Resolve(step Step) (Executor, error) {
	name := step.Executor

	modelID, source := step.Model, "model"
	if len(step.Preferred) > 0 {
		modelID, source = step.Preferred[0], "preferred[0]"
	}

	if modelID != "" {
		if r.models == nil {
			return nil, fmt.Errorf("engine: step %q: %s %q is set, but no model registry is configured", step.ID, source, modelID)
		}
		info, err := r.models.Get(modelID)
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

// ResolveModel resolves a specific Model ID to an Executor — the same
// name-lookup Resolve performs internally for Model/Preferred[0], exposed
// standalone so a caller (automatic model failover, ADR-0013 Proposed
// sixth increment — see executeGenerateStep in strategy.go) can resolve
// each subsequent Preferred entry in turn after the first one Resolve
// already tried. A clear, named error if no model.Registry is attached,
// id is not registered in it, or the Executor it names is not registered
// in r's ExecutorRegistry — the same failure shapes Resolve already
// produces for Model/Preferred[0], just without step-specific wrapping
// (the caller adds that context).
func (r Router) ResolveModel(id string) (Executor, error) {
	if r.models == nil {
		return nil, fmt.Errorf("model %q is set, but no model registry is configured", id)
	}
	info, err := r.models.Get(id)
	if err != nil {
		return nil, err
	}
	return r.registry.Get(info.Executor)
}

// ModelInfo returns id's catalog model.Info from r's attached
// model.Registry, without resolving it to an Executor — used by
// capability-aware model resolution (ADR-0013, Proposed, seventh
// increment) to inspect a candidate's Capabilities before deciding
// whether it may be selected at all, distinct from actually resolving
// and running it. A clear, named error if no model.Registry is attached
// or id is not registered in it — the same failure shape Resolve/
// ResolveModel already produce for an unknown model.
func (r Router) ModelInfo(id string) (model.Info, error) {
	if r.models == nil {
		return model.Info{}, fmt.Errorf("model %q is set, but no model registry is configured", id)
	}
	return r.models.Get(id)
}
