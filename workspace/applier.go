package workspace

import (
	"context"

	"foundry/domain"
	"foundry/engine"
)

// GitApplier implements engine.Applier by calling ApplyAct — an apply
// Step's concrete mechanism for a Pipeline that mutates a git repository.
type GitApplier struct{}

// Apply applies act's patch to repoPath (the Engine's configured workspace
// directory, always a repository path today).
func (GitApplier) Apply(ctx context.Context, repoPath string, act *domain.Act) error {
	return ApplyAct(ctx, repoPath, act)
}

var _ engine.Applier = GitApplier{}
