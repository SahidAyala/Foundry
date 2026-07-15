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
// is meant to type-assert an Executor for this interface and fall back to
// today's flat executeCostEstimateUSD constant when it is absent, so no
// existing Executor — executor/claude.ClaudeExecutor,
// executor.ScriptedExecutor — is required to implement it to keep working
// exactly as it does today. Nothing in this package type-asserts for it
// yet; Piece 5 is what will.
type CostEstimator interface {
	EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error)
}
