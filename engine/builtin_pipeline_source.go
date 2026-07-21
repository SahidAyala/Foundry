package engine

import (
	"context"
	_ "embed"
	"fmt"

	"foundry/domain"
)

// defaultPipelineDocument and reviewPipelineDocument are the built-in
// Pipelines, each authored as a declarative document (pipelines/*.json)
// and embedded into the binary at compile time — not discovered, walked,
// or read from the filesystem or network at runtime. A future filesystem
// or remote PipelineSource reads its own bytes at runtime and calls the
// same DecodePipelineDocument (document.go); BuiltinPipelineSource's only
// difference is that its bytes are fixed at build time.
//
// reviewPipelineDocument authors "review" — generate, then two
// independent verify Steps, with no bounded repair (repair.max_attempts:
// 0) — a genuinely different execution shape from "default" (one verify,
// one repair round). It is the same shape already proven to execute
// correctly through an unmodified PipelineStrategy and Engine in
// strategy_test.go's TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges,
// now shipped as declarative data instead of only a hand-built test
// fixture. Adding it required no change to Engine, Strategy, Pipeline,
// PipelineSource, or PipelineRegistry — only this file's embed list and
// builtinDocuments below grew by one entry.
//
//go:embed pipelines/default.json
var defaultPipelineDocument []byte

//go:embed pipelines/review.json
var reviewPipelineDocument []byte

// builtinDocuments lists every Pipeline document this build embeds, in a
// fixed order (defaultPipelineDocument first, matching today's sole
// caller expectations of "default" as index 0). A third built-in Pipeline
// is a third pipelines/*.json file, a third //go:embed line, and a third
// entry here — nothing else in this package changes.
var builtinDocuments = [][]byte{
	defaultPipelineDocument,
	reviewPipelineDocument,
}

// BuiltinPipelineSource is the PipelineSource for every Pipeline this build of
// Foundry ships compiled into the binary — today, the documents listed in
// builtinDocuments. It reads nothing from the filesystem or the network at
// runtime; a future filesystem-backed or remote PipelineSource satisfies
// the same interface alongside it without BuiltinPipelineSource or its
// Pipelines changing
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase 3+).
type BuiltinPipelineSource struct{}

var _ PipelineSource = BuiltinPipelineSource{}

// Load decodes every built-in Pipeline document. Decoding a fixed,
// compiled-in document can fail only on a programmer error — the embedded
// JSON itself malformed — which this build's own tests catch before it
// ever reaches a user, but which DecodePipelineDocument still surfaces as
// a normal error rather than hiding.
func (BuiltinPipelineSource) Load(ctx context.Context) ([]Pipeline, error) {
	pipelines := make([]Pipeline, 0, len(builtinDocuments))
	for _, document := range builtinDocuments {
		p, err := DecodePipelineDocument(document)
		if err != nil {
			return nil, fmt.Errorf("engine: BuiltinPipelineSource: %w", err)
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}

// DefaultPipeline reproduces the Engine's original fixed lifecycle exactly:
// one Executor call, one verification pass, and — on a failing verdict —
// at most one bounded repair round. It is hand-constructed Go data, kept
// deliberately independent of defaultPipelineDocument: it is the fixed
// reference a test pins BuiltinPipelineSource's decoded Pipeline against
// (builtin_pipeline_source_test.go), so a drift between the document and the
// runtime's original shape is a test failure, not a silent behavior
// change. BuiltinPipelineSource.Load no longer calls this function — it decodes
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
