package engine

import (
	"context"
	"fmt"
)

// PipelineRegistry holds named Pipeline definitions so Pipeline uniqueness
// is enforced in one place instead of scattered across every caller that
// needs one (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase
// 3+ groundwork). It has no update or delete: once registered under a
// name, a Pipeline is fixed for the registry's lifetime. Register refuses
// a second registration under the same name, and Get always returns an
// independent copy, so nothing a caller does with a Pipeline it obtained
// can reach back and corrupt what the registry holds.
//
// PipelineRegistry does not discover Pipelines itself — it only registers
// and looks up what a PipelineProvider (provider.go) hands it. Discovery
// and registration are deliberately separate responsibilities: a provider
// can be swapped (built-in, filesystem, embedded, remote) without the
// registry, or anything that looks a Pipeline up by name, changing at all.
type PipelineRegistry struct {
	pipelines map[string]Pipeline
}

// NewPipelineRegistry returns an empty PipelineRegistry.
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{pipelines: make(map[string]Pipeline)}
}

// NewDefaultRegistry returns a PipelineRegistry pre-populated with every
// Pipeline this build of Foundry ships built in, loaded from
// BuiltinProvider — today, only "default" (DefaultPipeline). A future
// built-in Pipeline is added to BuiltinProvider.Load, not here; Engine,
// Strategy, and PipelineRegistry never need to change to see it.
func NewDefaultRegistry() *PipelineRegistry {
	registry := NewPipelineRegistry()
	provider := BuiltinProvider{}
	pipelines, err := provider.Load(context.Background())
	if err != nil {
		// BuiltinProvider.Load has nothing external that can fail.
		panic(fmt.Sprintf("engine: NewDefaultRegistry: %v", err))
	}
	if err := registry.RegisterMany(pipelines...); err != nil {
		// Registering BuiltinProvider's own fixed set into a registry
		// created two lines above can only fail if the built-in set
		// itself declares a duplicate name — a programmer error, not a
		// runtime condition any caller can hit.
		panic(fmt.Sprintf("engine: NewDefaultRegistry: %v", err))
	}
	return registry
}

// Register adds p under p.Name. It returns an error, leaving the registry
// unchanged, if a Pipeline is already registered under that name.
func (r *PipelineRegistry) Register(p Pipeline) error {
	if _, exists := r.pipelines[p.Name]; exists {
		return fmt.Errorf("engine: pipeline %q is already registered", p.Name)
	}
	r.pipelines[p.Name] = clonePipeline(p)
	return nil
}

// RegisterMany registers each of pipelines in order, exactly as a loop of
// Register calls would. It stops at the first duplicate name and returns
// that error; every Pipeline registered before the failing one remains
// registered. This is the shape a PipelineProvider's Load result is meant
// to be registered with: provider.Load(ctx) then registry.RegisterMany(...).
func (r *PipelineRegistry) RegisterMany(pipelines ...Pipeline) error {
	for _, p := range pipelines {
		if err := r.Register(p); err != nil {
			return err
		}
	}
	return nil
}

// Get looks up the Pipeline registered under name. It returns an error if
// no Pipeline is registered under that name.
func (r *PipelineRegistry) Get(name string) (Pipeline, error) {
	p, ok := r.pipelines[name]
	if !ok {
		return Pipeline{}, fmt.Errorf("engine: no pipeline registered as %q", name)
	}
	return clonePipeline(p), nil
}

// MustGet is Get, panicking instead of returning an error if name is not
// registered. It is for composition roots that treat an unresolvable
// built-in Pipeline name as a programmer error to fail fast on, not a
// runtime condition worth propagating as an error value.
func (r *PipelineRegistry) MustGet(name string) Pipeline {
	p, err := r.Get(name)
	if err != nil {
		panic(fmt.Sprintf("engine: MustGet(%q): %v", name, err))
	}
	return p
}

// clonePipeline copies p's Steps slice into a new backing array so the
// returned Pipeline shares no mutable state with p. Register and Get both
// clone — on the way in and on the way out — so a caller can never mutate
// a Pipeline it holds and affect the registry's stored copy, or vice versa.
func clonePipeline(p Pipeline) Pipeline {
	steps := make([]Step, len(p.Steps))
	copy(steps, p.Steps)
	p.Steps = steps
	return p
}
