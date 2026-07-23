package vcs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"foundry/domain"
)

// initGitRepo creates a temporary git repository with a single committed
// file, greeting.txt, containing "hello\n" — the same fixture shape
// workspace's own tests use.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(dir+"/greeting.txt", []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")

	return dir
}

func initGitRepoWithRemote(t *testing.T) string {
	t.Helper()
	repo := initGitRepo(t)
	remote := t.TempDir()

	cmd := exec.Command("git", "init", "-q", "--bare", "-b", "main", remote)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

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

	return repo
}

const replacePatch = `diff --git a/greeting.txt b/greeting.txt
index 0000000..1111111 100644
--- a/greeting.txt
+++ b/greeting.txt
@@ -1 +1 @@
-hello
+goodbye
`

func TestGitHubPRApplier_Apply_MissingTokenEnvFieldFails(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	a := GitHubPRApplier{}

	act := &domain.Act{ID: "act-1", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err == nil {
		t.Fatal("Apply with no TokenEnv configured returned nil error")
	}
}

func TestGitHubPRApplier_Apply_UnsetEnvironmentVariableFails(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_UNSET_TOKEN_VAR"}

	os.Unsetenv("FOUNDRY_TEST_UNSET_TOKEN_VAR")
	act := &domain.Act{ID: "act-1", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err == nil {
		t.Fatal("Apply with an unset token env var returned nil error")
	}
}

func TestGitHubPRApplier_Apply_CommitsPushesAndCallsGH(t *testing.T) {
	repo := initGitRepoWithRemote(t)

	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	var capturedArgs []string
	var capturedEnv []string
	var capturedDir string
	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		capturedDir = dir
		capturedArgs = args
		capturedEnv = env
		out.Write([]byte("https://github.com/example/repo/pull/1\n"))
		return nil
	}

	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", run: fakeRun}
	var out bytes.Buffer
	a.Out = &out

	act := &domain.Act{ID: "act-1", Intent: "Fix the greeting", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if capturedDir != repo {
		t.Errorf("gh dir = %q, want %q", capturedDir, repo)
	}
	if len(capturedArgs) == 0 || capturedArgs[0] != "pr" || capturedArgs[1] != "create" {
		t.Errorf("gh args = %v, want it to start with [pr create]", capturedArgs)
	}
	wantHead := "foundry/act-act-1"
	foundHead := false
	for i, a := range capturedArgs {
		if a == "--head" && i+1 < len(capturedArgs) && capturedArgs[i+1] == wantHead {
			foundHead = true
		}
	}
	if !foundHead {
		t.Errorf("gh args = %v, want --head %q", capturedArgs, wantHead)
	}
	if len(capturedEnv) != 1 || capturedEnv[0] != "GH_TOKEN=shh-secret" {
		t.Errorf("gh env = %v, want exactly [GH_TOKEN=shh-secret]", capturedEnv)
	}
	if !strings.Contains(out.String(), "pull/1") {
		t.Errorf("Out = %q, want it to contain the PR URL gh printed", out.String())
	}

	// The local throwaway branch must be cleaned up after a successful
	// Apply (ADR-0010 Decision 5: only the remote branch and opened PR
	// are the durable, terminal result).
	list, err := gitOutputForTest(repo, "branch", "--list", "foundry/act-act-1")
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if list != "" {
		t.Errorf("local branch %q still exists after Apply, want it cleaned up", "foundry/act-act-1")
	}
}

func TestGitHubPRApplier_Apply_GHFailurePropagates(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	errGHFailed := errors.New("gh: pull request already exists")
	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		return errGHFailed
	}

	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", run: fakeRun}
	act := &domain.Act{ID: "act-2", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err == nil {
		t.Fatal("Apply returned nil error when gh pr create failed")
	}
}

// TestGitHubPRApplier_Apply_GHFailureAfterPushNamesTheDanglingBranch covers
// a real gap: Push (which already ran and succeeded by the time gh pr
// create fails) leaves a real branch live on the remote with no PR open
// for it. The returned error must name that branch and the remote
// explicitly — not read identically to any other gh failure — so the
// state is discoverable without a human having to already know to go
// look. The branch itself must actually be on the remote (proving this
// isn't a hypothetical), and Apply must not call Clean on this path, so
// the local worktree survives for retry/manual recovery too.
func TestGitHubPRApplier_Apply_GHFailureAfterPushNamesTheDanglingBranch(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	remote := gitRemoteURL(t, repo)
	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	errGHFailed := errors.New("gh: pull request already exists")
	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		return errGHFailed
	}

	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", run: fakeRun}
	act := &domain.Act{ID: "act-3", Intent: "test", Patch: replacePatch}
	err := a.Apply(context.Background(), repo, act)
	if err == nil {
		t.Fatal("Apply returned nil error when gh pr create failed")
	}

	branch := "foundry/act-" + act.ID
	if !strings.Contains(err.Error(), branch) {
		t.Errorf("error = %q, want it to name the dangling branch %q", err, branch)
	}
	if !strings.Contains(err.Error(), "already pushed") {
		t.Errorf("error = %q, want it to state the branch was already pushed", err)
	}

	branches, gitErr := gitOutputForTest(remote, "branch", "--list", branch)
	if gitErr != nil {
		t.Fatalf("list remote branches: %v", gitErr)
	}
	if !strings.Contains(branches, branch) {
		t.Errorf("branch %q was not actually found on the remote; the error's claim would be false", branch)
	}
}

// recordedCall captures one invocation of ghRunner, for tests that need to
// distinguish `gh pr create` from a later `gh pr edit --add-reviewer` call.
type recordedCall struct {
	args []string
	env  []string
}

func TestGitHubPRApplier_Apply_RequestsCopilotReviewWhenEnabled(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	var calls []recordedCall
	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		calls = append(calls, recordedCall{args: args, env: env})
		if args[0] == "pr" && args[1] == "create" {
			out.Write([]byte("https://github.com/example/repo/pull/1\n"))
		}
		return nil
	}

	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", RequestCopilotReview: true, run: fakeRun}
	var out bytes.Buffer
	a.Out = &out

	act := &domain.Act{ID: "act-copilot", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("gh was called %d times, want 2 (pr create, then pr edit --add-reviewer)", len(calls))
	}
	reviewCall := calls[1]
	wantBranch := "foundry/act-" + act.ID
	wantArgs := []string{"pr", "edit", wantBranch, "--add-reviewer", "@copilot"}
	if len(reviewCall.args) != len(wantArgs) {
		t.Fatalf("second gh call args = %v, want %v", reviewCall.args, wantArgs)
	}
	for i := range wantArgs {
		if reviewCall.args[i] != wantArgs[i] {
			t.Errorf("second gh call args = %v, want %v", reviewCall.args, wantArgs)
		}
	}
	if len(reviewCall.env) != 1 || reviewCall.env[0] != "GH_TOKEN=shh-secret" {
		t.Errorf("second gh call env = %v, want exactly [GH_TOKEN=shh-secret]", reviewCall.env)
	}
}

func TestGitHubPRApplier_Apply_DoesNotRequestReviewByDefault(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	var calls []recordedCall
	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		calls = append(calls, recordedCall{args: args, env: env})
		out.Write([]byte("https://github.com/example/repo/pull/1\n"))
		return nil
	}

	// RequestCopilotReview left at its zero value (false).
	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", run: fakeRun}
	act := &domain.Act{ID: "act-no-review", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("gh was called %d times, want exactly 1 (pr create only) when RequestCopilotReview is false", len(calls))
	}
}

// TestGitHubPRApplier_Apply_CopilotReviewFailureDoesNotFailApply covers the
// deliberate best-effort design: a repository without Copilot code review
// actually enabled (no paid plan, or the feature simply off) must not make
// Foundry treat the whole Act as failed — the pull request itself already
// exists by the time the review request is attempted.
func TestGitHubPRApplier_Apply_CopilotReviewFailureDoesNotFailApply(t *testing.T) {
	repo := initGitRepoWithRemote(t)
	t.Setenv("FOUNDRY_TEST_TOKEN", "shh-secret")

	fakeRun := func(ctx context.Context, dir string, args []string, env []string, out io.Writer) error {
		if args[0] == "pr" && args[1] == "create" {
			out.Write([]byte("https://github.com/example/repo/pull/1\n"))
			return nil
		}
		return errors.New("Copilot code review is not available for this repository")
	}

	a := GitHubPRApplier{TokenEnv: "FOUNDRY_TEST_TOKEN", RequestCopilotReview: true, run: fakeRun}
	var out bytes.Buffer
	a.Out = &out

	act := &domain.Act{ID: "act-review-fails", Intent: "test", Patch: replacePatch}
	if err := a.Apply(context.Background(), repo, act); err != nil {
		t.Fatalf("Apply failed: %v, want it to succeed despite the review-request failure", err)
	}
	if !strings.Contains(out.String(), "could not request a Copilot review") {
		t.Errorf("Out = %q, want it to warn about the failed review request", out.String())
	}
	if !strings.Contains(out.String(), "Copilot code review is not available") {
		t.Errorf("Out = %q, want it to include gh's own underlying error", out.String())
	}

	// The pull request's own branch must still be cleaned up locally --
	// the review-request failure must not leave Apply behaving as if the
	// whole thing failed.
	list, err := gitOutputForTest(repo, "branch", "--list", "foundry/act-"+act.ID)
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if list != "" {
		t.Errorf("local branch still exists after Apply, want it cleaned up despite the review-request failure")
	}
}

// gitRemoteURL returns repo's configured "origin" remote path, so a test
// can inspect the bare remote repository directly.
func gitRemoteURL(t *testing.T, repo string) string {
	t.Helper()
	url, err := gitOutputForTest(repo, "remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("get remote url: %v", err)
	}
	return strings.TrimSpace(url)
}

func gitOutputForTest(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
