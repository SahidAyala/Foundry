package workspace

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// initBareRemote creates a bare git repository at a fresh temp directory,
// suitable for use as another repository's "origin" remote in a test —
// Push needs somewhere real to push to, without any network access.
func initBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "--bare", "-b", "main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}
	return dir
}

// initGitRepoWithRemote is initGitRepo plus a "origin" remote pointing at a
// local bare repository, with the initial commit already pushed —
// Workspace.Push needs a remote that already knows the branch it is
// pushing from is a fast-forward.
func initGitRepoWithRemote(t *testing.T) (repo, remote string) {
	t.Helper()
	repo = initGitRepo(t)
	remote = initBareRemote(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("remote", "add", "origin", remote)
	run("push", "origin", "main")

	return repo, remote
}

func TestWorkspace_BranchName_ReturnsCreatedBranch(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "foundry/act-abc123")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}
	defer ws.Clean(context.Background())

	if got := ws.BranchName(); got != "foundry/act-abc123" {
		t.Errorf("BranchName() = %q, want %q", got, "foundry/act-abc123")
	}
}

func TestWorkspace_Commit_CreatesACommitOnTheBranch(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "feature-commit")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}
	defer ws.Clean(context.Background())

	patch := strings.Replace(replacePatch, "%s", "committed change", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if err := ws.Commit(context.Background(), "test: apply the patch"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// repo's own working tree was never touched by Commit at all — the
	// commit happened in ws's isolated worktree.
	status, err := gitOutput(context.Background(), repo, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if status != "" {
		t.Errorf("git status in repo after Commit = %q, want a clean working tree (repo's own checkout was never touched)", status)
	}

	// The commit lives on ws.BranchName(), not repo's own checked-out
	// branch — but since a worktree shares the same refs, it's visible from
	// repo by naming the branch explicitly.
	log, err := gitOutput(context.Background(), repo, "log", "-1", "--pretty=%s", ws.BranchName())
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if log != "test: apply the patch" {
		t.Errorf("last commit message on %q = %q, want %q", ws.BranchName(), log, "test: apply the patch")
	}
}

func TestWorkspace_Push_PushesCommitToRemote(t *testing.T) {
	repo, remote := initGitRepoWithRemote(t)

	ws, err := NewWorkspace(repo, "feature-push")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := strings.Replace(replacePatch, "%s", "pushed change", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if err := ws.Commit(context.Background(), "test: pushed change"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := ws.Push(context.Background(), "origin"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify the remote (bare) repository actually received the branch and
	// commit, without ever touching the network.
	remoteLog, err := gitOutput(context.Background(), remote, "log", "-1", "--pretty=%s", "feature-push")
	if err != nil {
		t.Fatalf("reading remote's feature-push branch failed: %v", err)
	}
	if remoteLog != "test: pushed change" {
		t.Errorf("remote's last commit on feature-push = %q, want %q", remoteLog, "test: pushed change")
	}

	if err := ws.Clean(context.Background()); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Clean only deletes the local branch; the pushed remote branch (and
	// its commit) must survive, exactly as ADR-0010 requires for a
	// remote-pr apply target's terminal result.
	if _, err := gitOutput(context.Background(), remote, "rev-parse", "feature-push"); err != nil {
		t.Errorf("remote branch feature-push no longer exists after local Clean: %v", err)
	}
}

func TestWorkspace_Push_FailsWithoutRemote(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "feature-no-remote")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}
	defer ws.Clean(context.Background())

	patch := strings.Replace(replacePatch, "%s", "no remote configured", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if err := ws.Commit(context.Background(), "test: no remote"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := ws.Push(context.Background(), "origin"); err == nil {
		t.Fatal("Push with no configured remote returned nil error")
	}
}
