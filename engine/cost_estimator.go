package engine

import (
	"context"

	"foundry/domain"
)

// CostEstimator is an optional Executor capability: an Executor that can
// report its own per-call cost estimate implements it (ADR-0005,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md, Decision
// 3 — Proposed, not yet ratified). Per-Step Budget accounting (RFC-0004
// §2.7, Piece 5 of docs/04-guides/multi-executor-router-implementation-plan.md)
// type-asserts an Executor for this interface — see estimateExecuteCostUSD
// below — and falls back to today's flat executeCostEstimateUSD constant
// when it is absent, so no existing Executor — executor/claude.ClaudeExecutor,
// executor.ScriptedExecutor — is required to implement it to keep working
// exactly as it does today.
type CostEstimator interface {
	EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error)
}

// estimateExecuteCostUSD returns executor's own pre-execution cost estimate
// for calling Execute with intent and considered, if executor implements
// CostEstimator, and the flat executeCostEstimateUSD constant otherwise.
// This is the one seam runSteps' generate Step (engine/strategy.go) reads
// from to charge Budget per Step rather than per attempt (RFC-0004 §2.7).
func estimateExecuteCostUSD(ctx context.Context, executor Executor, intent *domain.Intent, considered []string) (float64, error) {
	if ce, ok := executor.(CostEstimator); ok {
		return ce.EstimateCostUSD(ctx, intent, considered)
	}
	return executeCostEstimateUSD, nil
}
