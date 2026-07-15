package engine

import "fmt"

// ExecutorRegistry holds named Executors so a Router (router.go) can resolve
// a Step's explicit executor pin by name (RFC-0004 §2,
// docs/04-guides/multi-executor-router-implementation-plan.md Piece 1). It
// mirrors PipelineRegistry's register-once/look-up-by-name shape — not its
// code — under a name that avoids "Provider": terminology.md retires
// "Provider" for anything touching model access.
//
// ExecutorRegistry does not construct Executors itself and holds no
// reference to an Engine or a Router; wiring a concrete Executor into it
// (today, executor/claude's Claude Code Executor; later, Piece 3's
// additional vendor Executors) is entirely its caller's decision.
type ExecutorRegistry struct {
	executors map[string]Executor
}

// NewExecutorRegistry returns an empty ExecutorRegistry.
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{executors: make(map[string]Executor)}
}

// Register adds e under name. It returns an error, leaving the registry
// unchanged, if an Executor is already registered under that name.
func (r *ExecutorRegistry) Register(name string, e Executor) error {
	if _, exists := r.executors[name]; exists {
		return fmt.Errorf("engine: executor %q is already registered", name)
	}
	r.executors[name] = e
	return nil
}

// Get looks up the Executor registered under name. It returns an error if
// no Executor is registered under that name.
func (r *ExecutorRegistry) Get(name string) (Executor, error) {
	e, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("engine: no executor registered as %q", name)
	}
	return e, nil
}
