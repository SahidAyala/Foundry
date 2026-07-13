package workspace

import (
	"context"

	"foundry/domain"
)

// ApplyAct applies act's patch to repoPath on an isolated branch named for
// the Act, then lands it back on the branch the developer was on: a
// throwaway `foundry/act-<id>` branch must never be left behind for a
// successfully applied Act. This is the one mechanism both cli.CLI.Do (for
// a Pipeline with no apply Step) and GitApplier (for a Pipeline that
// declares one, RFC-0002 §9 Phase 4) share — neither reimplements it.
func ApplyAct(ctx context.Context, repoPath string, act *domain.Act) error {
	ws, err := NewWorkspace(repoPath, "foundry/act-"+act.ID)
	if err != nil {
		return err
	}
	if err := ws.Apply(ctx, act.Patch); err != nil {
		return err
	}
	return ws.Land(ctx)
}
