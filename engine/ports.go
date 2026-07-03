package engine

import (
	"context"

	"foundry/domain"
)

// Executor is the port for executing work. Production wires the Claude Code
// executor (executor/claude); tests wire the deterministic ScriptedExecutor.
type Executor interface {
	Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error)
}

// Verifier runs checks against an Outcome and renders a verdict.
type Verifier interface {
	Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error)
}

// Gatherer assembles the context an Executor needs to act on an Intent.
type Gatherer interface {
	Gather(ctx context.Context, intent *domain.Intent) ([]string, error)
}
