package engine

import (
	"context"
	_ "embed"
	"fmt"

	"foundry/domain"
)

// defaultPipelineDocument is the one built-in Pipeline, authored as a
// declarative document (pipelines/default.json) and embedded into the
// binary at compile time — not discovered, walked, or read from the
// filesystem or network at runtime. A future filesystem or remote
// PipelineProvider reads its own bytes at runtime and calls the same
// DecodePipelineDocument (document.go); BuiltinProvider's only difference
// is that its bytes are fixed at build time.
//
//go:embed pipelines/default.json
var defaultPipelineDocument []byte

// BuiltinProvider is the PipelineProvider for every Pipeline this build of
// Foundry ships compiled into the binary — today, the one document
// embedded as defaultPipelineDocument. It reads nothing from the
// filesystem or the network at runtime; a future filesystem-backed or
// remote PipelineProvider satisfies the same interface alongside it
// without BuiltinProvider or its Pipelines changing
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase 3+).
type BuiltinProvider struct{}

var _ PipelineProvider = BuiltinProvider{}

// Load decodes every built-in Pipeline document. Today that is exactly
// defaultPipelineDocument; a second built-in document is one more
// DecodePipelineDocument call, not an Engine or Strategy change. Decoding
// a fixed, compiled-in document can fail only on a programmer error — the
// embedded JSON itself malformed — which this build's own tests catch
// before it ever reaches a user, but which DecodePipelineDocument still
// surfaces as a normal error rather than hiding.
func (BuiltinProvider) Load(ctx context.Context) ([]Pipeline, error) {
	p, err := DecodePipelineDocument(defaultPipelineDocument)
	if err != nil {
		return nil, fmt.Errorf("engine: BuiltinProvider: %w", err)
	}
	return []Pipeline{p}, nil
}

// DefaultPipeline reproduces the Engine's original fixed lifecycle exactly:
// one Executor call, one verification pass, and — on a failing verdict —
// at most one bounded repair round. It is hand-constructed Go data, kept
// deliberately independent of defaultPipelineDocument: it is the fixed
// reference a test pins BuiltinProvider's decoded Pipeline against
// (builtin_provider_test.go), so a drift between the document and the
// runtime's original shape is a test failure, not a silent behavior
// change. BuiltinProvider.Load no longer calls this function — it decodes
// defaultPipelineDocument instead — though many tests still call
// DefaultPipeline directly to wire an Engine without a Registry.
func DefaultPipeline() Pipeline {
	return Pipeline{
		Name: "default",
		Steps: []Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: RepairPolicy{MaxAttempts: 1},
	}
}
