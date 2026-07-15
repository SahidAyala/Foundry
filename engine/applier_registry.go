package engine

import "fmt"

// ApplierRegistry holds named Appliers so an apply Step's declared Target
// can be resolved by name (RFC-0004 §2.6, Piece 4 of
// docs/04-guides/multi-executor-router-implementation-plan.md). It mirrors
// ExecutorRegistry's register-once/look-up-by-name shape.
//
// Unlike ExecutorRegistry paired with Router, there is no "default" concept
// here: ApplyTargetLocal (step.go) — or the empty string every apply Step
// had before Target existed — never goes through this registry at all;
// runContext.resolveApplier (strategy.go) resolves it directly to the
// Engine's single configured Applier. This registry only ever holds
// additional, named targets (ApplyTargetKnowledgeNote,
// ApplyTargetProjectDoc, ...) a Pipeline opts into explicitly.
type ApplierRegistry struct {
	appliers map[string]Applier
}

// NewApplierRegistry returns an empty ApplierRegistry.
func NewApplierRegistry() *ApplierRegistry {
	return &ApplierRegistry{appliers: make(map[string]Applier)}
}

// Register adds a under target. It returns an error, leaving the registry
// unchanged, if an Applier is already registered under that target.
func (r *ApplierRegistry) Register(target string, a Applier) error {
	if _, exists := r.appliers[target]; exists {
		return fmt.Errorf("engine: applier target %q is already registered", target)
	}
	r.appliers[target] = a
	return nil
}

// Get looks up the Applier registered under target. It returns a clear,
// named error if none is registered — a Step whose apply Target can't be
// resolved is a configuration error, never a silent no-op.
func (r *ApplierRegistry) Get(target string) (Applier, error) {
	a, ok := r.appliers[target]
	if !ok {
		return nil, fmt.Errorf("engine: no applier registered for target %q", target)
	}
	return a, nil
}
