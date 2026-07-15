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

func gitOutputForTest(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
