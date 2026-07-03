// Package workspace isolates patch application inside a throwaway git branch
// so that a repository is never mutated outside an explicit Clean.
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// branchNamePattern allows only characters that are safe to pass to git and
// cannot be misread as a command-line flag or path traversal.
var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]*$`)

// Workspace represents an isolated copy of a project, realized as a
// throwaway git branch inside an existing repository checkout.
type Workspace struct {
	repoPath    string
	branchName  string
	patchPath   string // unified diff file
	originalRef string // branch to restore on Clean
}

// NewWorkspace checks that repoPath is a git repository and branchName is a
// safe git ref, then creates and checks out branchName from the current HEAD.
func NewWorkspace(repoPath string, branchName string) (*Workspace, error) {
	if err := validateBranchName(branchName); err != nil {
		return nil, err
	}

	ctx := context.Background()

	if _, err := gitOutput(ctx, repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("workspace: %q is not a git repository: %w", repoPath, err)
	}

	originalRef, err := gitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("workspace: determine current branch: %w", err)
	}

	if _, err := gitOutput(ctx, repoPath, "checkout", "-b", branchName); err != nil {
		return nil, fmt.Errorf("workspace: create branch %q: %w", branchName, err)
	}

	return &Workspace{
		repoPath:    repoPath,
		branchName:  branchName,
		originalRef: originalRef,
	}, nil
}

// Apply writes patch to a temp file and runs `git apply` against it in the
// workspace's branch, reporting conflicts as errors.
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

	if _, err := gitOutput(ctx, w.repoPath, "apply", w.patchPath); err != nil {
		return fmt.Errorf("workspace: apply patch: %w", err)
	}
	return nil
}

// Clean discards any changes made in the workspace's branch, checks the
// repository back out to the branch it was on before NewWorkspace, and
// deletes the throwaway branch.
func (w *Workspace) Clean(ctx context.Context) error {
	if _, err := gitOutput(ctx, w.repoPath, "reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("workspace: reset changes: %w", err)
	}
	if _, err := gitOutput(ctx, w.repoPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("workspace: remove untracked files: %w", err)
	}
	if _, err := gitOutput(ctx, w.repoPath, "checkout", w.originalRef); err != nil {
		return fmt.Errorf("workspace: checkout %q: %w", w.originalRef, err)
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
