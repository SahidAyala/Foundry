package engine

import (
	"context"

	"foundry/domain"
)

// BuiltinProvider is the PipelineProvider for every Pipeline this build of
// Foundry ships compiled into the binary — today, only DefaultPipeline. It
// reads nothing from the filesystem or the network; a future
// filesystem-backed or embedded PipelineProvider satisfies the same
// interface alongside it without BuiltinProvider or its Pipelines changing
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase 3+).
type BuiltinProvider struct{}

var _ PipelineProvider = BuiltinProvider{}

// Load returns every built-in Pipeline. A BuiltinProvider has nothing
// external that can fail, so Load always succeeds; the error return exists
// only to satisfy PipelineProvider, whose other implementations (a
// filesystem read, a remote fetch) genuinely can fail.
func (BuiltinProvider) Load(ctx context.Context) ([]Pipeline, error) {
	return []Pipeline{DefaultPipeline()}, nil
}

// DefaultPipeline reproduces the Engine's original fixed lifecycle exactly:
// one Executor call, one verification pass, and — on a failing verdict —
// at most one bounded repair round. It is loaded by BuiltinProvider and
// registered under the name "default" by NewDefaultRegistry
// (registry.go) — the only Pipeline this build of Foundry ships built in,
// though a PipelineRegistry itself supports registering any number of
// Pipelines under distinct names from any number of PipelineProviders.
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
