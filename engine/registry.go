package engine

import "fmt"

// PipelineRegistry holds named Pipeline definitions so Pipeline creation is
// centralized in one place instead of scattered across every caller that
// needs one (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase
// 3+ groundwork). It has no update or delete: once registered under a
// name, a Pipeline is fixed for the registry's lifetime. Register refuses
// a second registration under the same name, and Get always returns an
// independent copy, so nothing a caller does with a Pipeline it obtained
// can reach back and corrupt what the registry holds.
type PipelineRegistry struct {
	pipelines map[string]Pipeline
}

// NewPipelineRegistry returns an empty PipelineRegistry.
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{pipelines: make(map[string]Pipeline)}
}

// NewDefaultRegistry returns a PipelineRegistry pre-populated with every
// Pipeline this build of Foundry ships built in — today, only "default"
// (DefaultPipeline). A future built-in Pipeline is added here, as one more
// Register call; Engine and Strategy never need to change to see it.
func NewDefaultRegistry() *PipelineRegistry {
	registry := NewPipelineRegistry()
	if err := registry.Register(DefaultPipeline()); err != nil {
		// Registering a fixed, known-unique name into a registry created
		// two lines above can only fail if the built-in set itself
		// declares a duplicate name — a programmer error, not a runtime
		// condition any caller can hit.
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

// Get looks up the Pipeline registered under name. It returns an error if
// no Pipeline is registered under that name.
func (r *PipelineRegistry) Get(name string) (Pipeline, error) {
	p, ok := r.pipelines[name]
	if !ok {
		return Pipeline{}, fmt.Errorf("engine: no pipeline registered as %q", name)
	}
	return clonePipeline(p), nil
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
