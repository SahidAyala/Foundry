package engine

import "context"

// PipelineProvider discovers Pipeline definitions from some source — Go
// code, a filesystem directory, an embedded asset, or (later) a remote
// registry — and returns them for a caller to register into a
// PipelineRegistry. A PipelineProvider only discovers: it never registers a
// Pipeline itself, never de-duplicates by name, and never resolves a name
// to a Pipeline for the Engine to run. Those are PipelineRegistry's job and
// Engine's job respectively; a PipelineProvider does not hold a reference
// to either (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase
// 3+).
type PipelineProvider interface {
	// Load returns every Pipeline this provider knows about. Load must not
	// mutate any PipelineRegistry or retain the caller's use of the
	// returned slice; what happens to the result — whether, and how, it
	// gets registered — is entirely the caller's decision.
	Load(ctx context.Context) ([]Pipeline, error)
}
