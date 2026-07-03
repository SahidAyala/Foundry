// Package executor produces Outcomes from Intents.
package executor

import (
	"context"

	"foundry/domain"
)

// ScriptedExecutor returns a fixed patch, deterministically, regardless of
// the Intent or gathered context. It proves the Act lifecycle works without
// any nondeterminism or network.
type ScriptedExecutor struct {
	patch string // hard-coded test patch
}

// NewScriptedExecutor creates a ScriptedExecutor that always returns patch.
func NewScriptedExecutor(patch string) *ScriptedExecutor {
	return &ScriptedExecutor{patch: patch}
}

// Execute always returns the same patch, deterministically.
func (s *ScriptedExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return &domain.Outcome{Patch: s.patch}, nil
}
