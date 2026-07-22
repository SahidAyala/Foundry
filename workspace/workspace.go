// Package workspace isolates patch application inside a throwaway git
// worktree so that a repository's own checkout is never mutated outside an
// explicit Land.
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// branchNamePattern allows only characters that are safe to pass to git and
// cannot be misread as a command-line flag or path traversal.
var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]*$`)

// Workspace represents an isolated copy of a project, realized as a
// throwaway branch checked out into its own git worktree — a separate
// directory sharing the repository's object database, refs, and remotes.
// repoPath's own checked-out branch is never switched for any part of an
// Act's lifetime: Apply, Commit, and Push all run inside the worktree; Land
// is the only operation that ever touches repoPath itself, and only via a
// single `git apply`, never a checkout.
type Workspace struct {
	repoPath    string
	branchName  string
	worktreeDir string // the isolated directory Apply/Commit/Push run in
	tmpDir      string // parent of worktreeDir, removed wholesale on cleanup
	patchPath   string // unified diff file, reused for both Apply and Land
	startingRef string // repoPath's branch at NewWorkspace time; Land refuses to run if this has since changed
}

// NewWorkspace checks that repoPath is a git repository and branchName is a
// safe git ref, then creates branchName as a new branch checked out into a
// fresh, separate git worktree — mirroring StagedVerifier's own proven
// worktree-staging pattern (staged.go), but for the actual apply path
// rather than verification. repoPath's own checked-out branch is never
// touched.
func NewWorkspace(repoPath string, branchName string) (*Workspace, error) {
	if err := validateBranchName(branchName); err != nil {
		return nil, err
	}

	ctx := context.Background()

	if _, err := gitOutput(ctx, repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("workspace: %q is not a git repository: %w", repoPath, err)
	}

	startingRef, err := gitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("workspace: determine current branch: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "foundry-workspace-")
	if err != nil {
		return nil, fmt.Errorf("workspace: create worktree dir: %w", err)
	}

	// The worktree directory's leaf name is derived from branchName (unique
	// per Act) rather than a constant: git derives its own internal
	// .git/worktrees/<name> administrative directory from this path's
	// basename, so two concurrent NewWorkspace calls against the same
	// repoPath must not contend for the same name.
	worktreeDir := filepath.Join(tmpDir, worktreeDirName(branchName))
	if _, err := gitOutput(ctx, repoPath, "worktree", "add", "-b", branchName, worktreeDir, "HEAD"); err != nil {
		// `worktree add` can fail after partially registering
		// .git/worktrees/<name> (e.g. a disk-full error partway through
		// populating the worktree) — removing tmpDir alone leaves that
		// registration behind, exactly the scenario cleanup()'s own
		// worktree-prune fallback already documents (a killed process can
		// leave .git/worktrees/<name> registered after its directory is
		// gone). `prune` only ever removes a registration whose directory
		// no longer exists — it is always safe to attempt, best-effort,
		// even when nothing was actually left behind (e.g. `worktree add`
		// failed for an unrelated reason, such as branchName already
		// existing). Deliberately not also deleting branchName here: unlike
		// the worktree registration, a pre-existing branch of that name was
		// not created by this failed call and must not be destroyed by its
		// error path.
		os.RemoveAll(tmpDir)
		gitOutput(ctx, repoPath, "worktree", "prune")
		return nil, fmt.Errorf("workspace: create worktree for branch %q: %w", branchName, err)
	}

	return &Workspace{
		repoPath:    repoPath,
		branchName:  branchName,
		worktreeDir: worktreeDir,
		tmpDir:      tmpDir,
		startingRef: startingRef,
	}, nil
}

// worktreeDirName returns a filesystem-safe directory leaf name unique to
// branchName (branchName already includes the Act ID, e.g.
// "foundry/act-<id>").
func worktreeDirName(branchName string) string {
	return strings.ReplaceAll(branchName, "/", "-")
}

// Apply writes patch to a temp file and runs `git apply` against it inside
// w's isolated worktree, reporting conflicts as errors. repoPath is not
// touched.
func (w *Workspace) Apply(ctx context.Context, patch string) error {
	f, err := os.CreateTemp("", "workspace-*.patch")
	if err != nil {
		return fmt.Errorf("workspace: create patch file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(patch); err != nil {
		return fmt.Errorf("workspace: write patch file: %w", err)
	}
	w.patchPath = f.Name()

	if _, err := gitOutput(ctx, w.worktreeDir, "apply", w.patchPath); err != nil {
		return fmt.Errorf("workspace: apply patch: %w", err)
	}
	return nil
}

// Land applies the same patch bytes Apply already validated in w's
// worktree directly onto repoPath — the developer's real, never-switched
// checkout — so what was verified/approved and what lands are provably
// byte-identical, then removes the worktree and its throwaway branch. Land
// is the accept path once an Authority has approved the Outcome; Clean
// remains the discard path and must never be called after Land.
//
// Land refuses to run, without touching repoPath or cleaning up anything,
// if repoPath's checked-out branch has changed since NewWorkspace: landing
// a patch onto a branch the developer switched to mid-Act would be silently
// wrong, not merely surprising. On any failure — the safety check or the
// repoPath apply itself — the worktree, branch, and patch file are left in
// place for manual recovery rather than destroyed.
func (w *Workspace) Land(ctx context.Context) error {
	currentRef, err := gitOutput(ctx, w.repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("workspace: determine current branch: %w", err)
	}
	if currentRef != w.startingRef {
		return fmt.Errorf("workspace: %q was switched from branch %q to %q since this Act started; refusing to land — worktree %q and branch %q left in place for manual recovery",
			w.repoPath, w.startingRef, currentRef, w.worktreeDir, w.branchName)
	}

	if _, err := gitOutput(ctx, w.repoPath, "apply", w.patchPath); err != nil {
		return fmt.Errorf("workspace: apply patch to %q: %w — worktree %q and branch %q left in place for manual recovery",
			w.repoPath, err, w.worktreeDir, w.branchName)
	}

	return w.cleanup(ctx)
}

// Clean discards w's worktree and its throwaway branch, leaving repoPath
// entirely untouched — including any of the developer's own pre-existing
// uncommitted changes, which Clean never had any reason to see in the
// first place.
func (w *Workspace) Clean(ctx context.Context) error {
	return w.cleanup(ctx)
}

// cleanup removes w's worktree — falling back to `git worktree prune` if a
// direct removal fails, mirroring StagedVerifier's own cleanup, since a
// killed process can leave .git/worktrees/<name> registered after its
// directory is gone — deletes the local throwaway branch, and removes the
// temp directory and patch file. The worktree must be removed before the
// branch can be deleted: git refuses to delete a branch checked out in any
// worktree.
func (w *Workspace) cleanup(ctx context.Context) error {
	if _, err := gitOutput(ctx, w.repoPath, "worktree", "remove", "--force", w.worktreeDir); err != nil {
		gitOutput(ctx, w.repoPath, "worktree", "prune")
	}
	if err := os.RemoveAll(w.tmpDir); err != nil {
		return fmt.Errorf("workspace: remove worktree dir: %w", err)
	}
	if _, err := gitOutput(ctx, w.repoPath, "branch", "-D", w.branchName); err != nil {
		return fmt.Errorf("workspace: delete branch %q: %w", w.branchName, err)
	}
	if w.patchPath != "" {
		if err := os.Remove(w.patchPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("workspace: remove patch file: %w", err)
		}
	}
	return nil
}

// BranchName returns the throwaway branch NewWorkspace created — the name
// a caller pushing this workspace to a remote (Push) needs to name as the
// pushed ref (ADR-0010, docs/03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md).
func (w *Workspace) BranchName() string {
	return w.branchName
}

// Commit stages every change in w's worktree (from a prior Apply) and
// commits it with message. Unlike Land, which never commits and instead
// applies the same patch directly onto repoPath, a remote apply target
// (vcs.GitHubPRApplier) needs a real commit to push — nothing to push a
// diff to otherwise.
func (w *Workspace) Commit(ctx context.Context, message string) error {
	if _, err := gitOutput(ctx, w.worktreeDir, "add", "-A"); err != nil {
		return fmt.Errorf("workspace: stage changes: %w", err)
	}
	if _, err := gitOutput(ctx, w.worktreeDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("workspace: commit: %w", err)
	}
	return nil
}

// Push pushes w's worktree's branch to remote, creating the matching
// remote-tracking branch. It requires the repository's own remote and
// credentials to already be configured for pushing (e.g. an SSH key, or a
// credential helper `gh auth login` sets up) — Foundry does not manage git
// push authentication separately from whatever the developer's checkout
// already has configured.
func (w *Workspace) Push(ctx context.Context, remote string) error {
	if _, err := gitOutput(ctx, w.worktreeDir, "push", "-u", remote, w.branchName); err != nil {
		return fmt.Errorf("workspace: push %q to %q: %w", w.branchName, remote, err)
	}
	return nil
}

func validateBranchName(name string) error {
	if name == "" {
		return errors.New("workspace: branch name must not be empty")
	}
	if !branchNamePattern.MatchString(name) {
		return fmt.Errorf("workspace: unsafe branch name %q", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("workspace: unsafe branch name %q", name)
	}
	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("workspace: unsafe branch name %q", name)
	}
	return nil
}

// gitOutput runs git with args in dir and returns its trimmed combined output.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return strings.TrimSpace(out.String()), nil
}
