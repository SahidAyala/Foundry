package engine

import "fmt"

// Router resolves the Executor a Step's Generate work runs against
// (RFC-0004 §2, docs/04-guides/multi-executor-router-implementation-plan.md
// Piece 1). It has exactly one policy: a Step's explicit executor pin, or
// the Engine's default Executor — no capability matching against advertised
// properties. That is RFC-0002 §7 layer 2, deliberately out of scope until a
// real multi-Executor Pipeline in production motivates it.
type Router struct {
	registry *ExecutorRegistry
	def      Executor
}

// NewRouter returns a Router that resolves a Step's pinned Executor out of
// registry, falling back to def when a Step declares no pin. def is never
// nil in practice — it is always the Engine's own configured Executor,
// preserving the exact behavior every Step had before Router existed.
func NewRouter(registry *ExecutorRegistry, def Executor) Router {
	return Router{registry: registry, def: def}
}

// Resolve returns the Executor step.Kind's Generate work should run against:
// step's pinned Executor (step.Executor), if set and registered in r's
// registry; a clear, named error if step.Executor is set but not
// registered — a pin that can't be honored is never silently ignored in
// favor of the default; or r's default Executor if step.Executor is unset,
// exactly what every Step meant before Router existed.
func (r Router) Resolve(step Step) (Executor, error) {
	if step.Executor == "" {
		return r.def, nil
	}
	e, err := r.registry.Get(step.Executor)
	if err != nil {
		return nil, fmt.Errorf("engine: step %q: %w", step.ID, err)
	}
	return e, nil
}
