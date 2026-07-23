// Package vcs holds Appliers whose apply target reaches outside the
// developer's own machine — publishing to a shared version-control host,
// not merely landing a patch on the developer's own branch (ADR-0010,
// docs/03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md). This is
// deliberately a separate package from workspace: workspace's Appliers
// (GitApplier, KnowledgeNoteApplier, ProjectDocApplier) never leave the
// local git repository; vcs's do.
package vcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"foundry/domain"
	"foundry/engine"
	"foundry/workspace"
)

// defaultRemote is the git remote GitHubPRApplier pushes to. Foundry does
// not support configuring a different remote name; "origin" is the git and
// GitHub CLI convention every developer's clone already has.
const defaultRemote = "origin"

// GitHubPRApplier implements engine.Applier for a Pipeline's apply Step
// declaring Target: engine.ApplyTargetRemotePR (ADR-0010) — Foundry's first
// apply target that leaves the developer's own machine. It commits
// act.Patch to a throwaway branch, pushes it to defaultRemote, and opens a
// pull request via the gh CLI (a subprocess, not an embedded API client,
// mirroring ADR-0001's existing extension-boundary posture for
// executor/claude and git apply) — then cleans up the local throwaway
// branch, leaving the pushed remote branch and opened PR as the durable,
// terminal result (ADR-0010 Decision 5). It never merges, watches, or
// reacts to the PR's later review: that is a separate, GitHub-native
// process outside Foundry's Record.
type GitHubPRApplier struct {
	// TokenEnv names the environment variable GitHubPRApplier reads its
	// GitHub credential from at Apply time — never persisted, logged, or
	// passed through domain.Intent or any recorded Evidence, mirroring
	// ADR-0005 Decision 5's Executor-credential pattern. Required: Apply
	// fails clearly if TokenEnv is empty or names an unset variable.
	TokenEnv string

	// Out receives the gh CLI's own output, including the opened PR's URL
	// (which gh prints on success), so a caller can see it. Nil means
	// os.Stdout — Apply always surfaces the PR URL somewhere, never
	// silently.
	Out io.Writer

	// RequestCopilotReview, when true, asks GitHub Copilot to review the
	// pull request (`gh pr edit --add-reviewer @copilot`, GA since
	// 2026-03-11) immediately after Apply successfully opens it. This
	// requires a paid Copilot plan on the repository/organization (GitHub
	// Docs, "Using GitHub Copilot code review") — Foundry has no way to
	// detect whether that's actually available, so it defaults to false
	// rather than guessing. A failure to request the review is surfaced to
	// Out as a warning, never returned as an Apply error: the pull request
	// — Foundry's actual terminal action (ADR-0010 Decision 5) — has
	// already, successfully, been created by the time this runs.
	RequestCopilotReview bool

	// run is the seam createPullRequest and requestCopilotReview call
	// through instead of shelling out directly, so tests never require a
	// real gh binary, network access, or GitHub credentials. Nil means
	// runGH, the real subprocess implementation.
	run ghRunner
}

// copilotReviewer is the handle `gh pr edit --add-reviewer` expects to
// request a Copilot code review (github.blog/changelog,
// "Request Copilot code review from GitHub CLI", 2026-03-11).
const copilotReviewer = "@copilot"

var _ engine.Applier = GitHubPRApplier{}

// Apply commits act.Patch to a new branch named for act, pushes it to
// defaultRemote, and opens a pull request against workspaceRoot's
// repository via `gh pr create`.
func (a GitHubPRApplier) Apply(ctx context.Context, workspaceRoot string, act *domain.Act) error {
	if a.TokenEnv == "" {
		return fmt.Errorf("vcs: github-pr: no credential configured (remote_publish_token_env in .foundry/config.json)")
	}
	token := os.Getenv(a.TokenEnv)
	if token == "" {
		return fmt.Errorf("vcs: github-pr: environment variable %q (remote_publish_token_env) is not set", a.TokenEnv)
	}

	ws, err := workspace.NewWorkspace(workspaceRoot, "foundry/act-"+act.ID)
	if err != nil {
		return fmt.Errorf("vcs: github-pr: %w", err)
	}
	if err := ws.Apply(ctx, act.Patch); err != nil {
		return fmt.Errorf("vcs: github-pr: %w", err)
	}
	if err := ws.Commit(ctx, commitMessage(act)); err != nil {
		return fmt.Errorf("vcs: github-pr: %w", err)
	}
	if err := ws.Push(ctx, defaultRemote); err != nil {
		return fmt.Errorf("vcs: github-pr: %w", err)
	}

	out := a.Out
	if out == nil {
		out = os.Stdout
	}
	run := a.run
	if run == nil {
		run = runGH
	}
	if err := createPullRequest(ctx, run, workspaceRoot, ws.BranchName(), act, token, out); err != nil {
		// Push (above) already succeeded by this point — the branch is
		// real and live on defaultRemote even though no PR exists for it.
		// Naming that state explicitly here is the fix: previously this
		// error read identically to any other gh failure, giving no hint
		// that anything had already reached the remote, and Apply's own
		// dangling local worktree/branch (Clean is deliberately not called
		// below on this path) had no comment explaining why either.
		return fmt.Errorf("vcs: github-pr: %w — branch %q was already pushed to %q with no pull request open for it; the pushed branch and the local worktree are left in place for manual recovery (retry, or open the PR yourself with `gh pr create --head %s`)",
			err, ws.BranchName(), defaultRemote, ws.BranchName())
	}

	if a.RequestCopilotReview {
		if err := requestCopilotReview(ctx, run, workspaceRoot, ws.BranchName(), token); err != nil {
			// Best-effort, deliberately: the pull request itself already
			// exists by this point. A repository without Copilot code
			// review actually enabled (no paid plan, or the feature simply
			// off) must not make Foundry treat the whole Act as failed over
			// this supplementary, opt-in nice-to-have.
			fmt.Fprintf(out, "vcs: github-pr: could not request a Copilot review (continuing; the pull request was already opened): %v\n", err)
		}
	}

	if err := ws.Clean(ctx); err != nil {
		return fmt.Errorf("vcs: github-pr: %w", err)
	}
	return nil
}

// commitMessage renders act's Intent as a one-line commit message.
func commitMessage(act *domain.Act) string {
	return strings.TrimSpace(act.Intent)
}

// createPullRequest shells out to `gh pr create` via run, authenticating
// with token through the GH_TOKEN environment variable set only for this
// one subprocess call — never the ambient process environment, never
// persisted, never logged.
func createPullRequest(ctx context.Context, run ghRunner, repoPath, branch string, act *domain.Act, token string, out io.Writer) error {
	title := strings.TrimSpace(act.Intent)
	body := fmt.Sprintf("Opened by Foundry for Act %s.\n\n%s", act.ID, title)

	args := []string{"pr", "create", "--head", branch, "--title", title, "--body", body}
	env := []string{"GH_TOKEN=" + token}
	if err := run(ctx, repoPath, args, env, out); err != nil {
		return fmt.Errorf("gh pr create: %w", err)
	}
	return nil
}

// requestCopilotReview shells out to `gh pr edit <branch> --add-reviewer
// @copilot` via run, adding Copilot as a requested reviewer on the pull
// request already opened for branch. It authenticates the same way
// createPullRequest does: token through GH_TOKEN, set only for this one
// subprocess call. Output is discarded (io.Discard) rather than mixed into
// Out — this is a supplementary, best-effort call the caller already
// wraps in its own warning message on failure, and gh's own confirmation
// text on success adds nothing a caller needs to see.
func requestCopilotReview(ctx context.Context, run ghRunner, repoPath, branch, token string) error {
	args := []string{"pr", "edit", branch, "--add-reviewer", copilotReviewer}
	env := []string{"GH_TOKEN=" + token}
	if err := run(ctx, repoPath, args, env, io.Discard); err != nil {
		return fmt.Errorf("gh pr edit --add-reviewer %s: %w", copilotReviewer, err)
	}
	return nil
}

// ghRunner runs the gh CLI with args in dir, appending env to the
// process's own environment, and writes combined output to out. It is the
// seam createPullRequest calls through — tests substitute a fake so they
// never require a real gh binary, network access, or GitHub credentials.
type ghRunner func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error

// runGH is the production ghRunner: a real gh subprocess.
func runGH(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
