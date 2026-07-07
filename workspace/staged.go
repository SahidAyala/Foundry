package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"foundry/domain"
	"foundry/engine"
)

// StagedVerifier decorates an engine.Verifier so that validators check the
// proposed Outcome instead of the developer's checkout: it stages the
// repository's HEAD into a throwaway git worktree, applies the Outcome's
// patch there, and delegates verification to the staged copy. The
// developer's checkout is never touched, and the worktree is removed before
// returning.
//
// A patch that does not apply is a property of the Outcome, not of the
// environment, so it yields a fail Judgment carrying git's findings — food
// for the repair loop — rather than an error.
type StagedVerifier struct {
	verifier engine.Verifier
}

// NewStagedVerifier wraps verifier so it runs against a staged copy of the
// repository with the Outcome's patch applied.
func NewStagedVerifier(verifier engine.Verifier) *StagedVerifier {
	return &StagedVerifier{verifier: verifier}
}

var _ engine.Verifier = (*StagedVerifier)(nil)

// Verify stages workspace's HEAD into a temporary worktree, applies
// outcome's patch, and runs the wrapped verifier there.
func (s *StagedVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	tmp, err := os.MkdirTemp("", "foundry-staged-")
	if err != nil {
		return nil, fmt.Errorf("workspace: create staging dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	staged := filepath.Join(tmp, "worktree")
	if _, err := gitOutput(ctx, workspace, "worktree", "add", "--detach", staged, "HEAD"); err != nil {
		return nil, fmt.Errorf("workspace: stage worktree: %w", err)
	}
	defer func() {
		if _, err := gitOutput(ctx, workspace, "worktree", "remove", "--force", staged); err != nil {
			// The staged directory itself is removed with tmp above; make
			// sure git forgets its registration too.
			gitOutput(ctx, workspace, "worktree", "prune")
		}
	}()

	if outcome.Patch != "" {
		if err := applyIn(ctx, staged, outcome.Patch); err != nil {
			return &domain.Judgment{
				Verdict: "fail",
				Checked: []string{"apply-patch: fail\n" + err.Error()},
			}, nil
		}
	}

	return s.verifier.Verify(ctx, outcome, staged)
}

// applyIn runs `git apply` for patch inside dir, feeding the patch on stdin
// so failure findings stay free of temp-file paths — they end up in the
// Act's recorded Evidence and must be deterministic.
func applyIn(ctx context.Context, dir string, patch string) error {
	cmd := exec.CommandContext(ctx, "git", "apply", "-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}
