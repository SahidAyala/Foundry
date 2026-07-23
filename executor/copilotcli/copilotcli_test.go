package copilotcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

// initGitRepo creates a temporary git repository with one committed file,
// mirroring the fixture shape vcs/github_pr_applier_test.go's own
// initGitRepo already uses — needed here because gitStatusPorcelain shells
// out to a real `git status --porcelain`, unlike the mockable runner
// seam the CLI invocation itself goes through.
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

	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")

	return dir
}

// fakeRunner is an injectable runner that returns canned output and
// captures what it was invoked with, optionally mutating the workspace
// (simulating the CLI editing a file directly instead of only proposing a
// diff) — mirrors executor/claude's and executor/geminicli's own test
// double, extended with that one mutation hook this package's own safety
// check needs covering.
type fakeRunner struct {
	stdout, stderr string
	err            error
	mutateFile     string // if set, Run writes to this path — simulates the CLI editing a file directly

	gotDir, gotName, gotStdin string
	gotArgs                   []string
}

func (f *fakeRunner) Run(ctx context.Context, dir, name string, args []string, stdin string) (string, string, error) {
	f.gotDir, f.gotName, f.gotArgs, f.gotStdin = dir, name, args, stdin
	if f.mutateFile != "" {
		if err := os.WriteFile(f.mutateFile, []byte("mutated by the CLI directly\n"), 0o644); err != nil {
			return "", "", err
		}
	}
	return f.stdout, f.stderr, f.err
}

func newExecutor(workspace string, r runner) *Executor {
	return &Executor{
		workspace:  workspace,
		executable: "copilot",
		timeout:    time.Minute,
		runner:     r,
	}
}

// sampleDiff ends in a newline because git apply rejects a patch whose last
// line is unterminated; executor.ParsePatch guarantees this normalization.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

func TestExecute_Success(t *testing.T) {
	repo := initGitRepo(t)
	r := &fakeRunner{stdout: "```diff\n" + sampleDiff + "```\n"}
	e := newExecutor(repo, r)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "add a comment"}, []string{"main.go contents"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}

	if r.gotDir != repo {
		t.Errorf("runner dir = %q, want %q", r.gotDir, repo)
	}
	if r.gotName != "copilot" {
		t.Errorf("runner name = %q, want %q", r.gotName, "copilot")
	}
	wantArgs := []string{"-s", "--no-ask-user"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
	if !strings.Contains(r.gotStdin, "add a comment") {
		t.Errorf("prompt missing intent, got:\n%s", r.gotStdin)
	}
	if !strings.Contains(r.gotStdin, "main.go contents") {
		t.Errorf("prompt missing gathered context, got:\n%s", r.gotStdin)
	}
}

// TestExecute_NeverGrantsToolAccess is a regression guard on the one thing
// this package's whole safety story depends on: no --allow-tool or
// --allow-all-tools flag is ever passed, for any input.
func TestExecute_NeverGrantsToolAccess(t *testing.T) {
	repo := initGitRepo(t)
	r := &fakeRunner{stdout: "```diff\n" + sampleDiff + "```\n"}
	e := newExecutor(repo, r)
	e.model = "some-model"

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	for _, a := range r.gotArgs {
		if strings.Contains(a, "allow-tool") || strings.Contains(a, "allow-all") {
			t.Errorf("runner args = %v, want no --allow-tool/--allow-all flag ever passed", r.gotArgs)
		}
	}
}

func TestExecute_ModelFlagAppendedWhenSet(t *testing.T) {
	repo := initGitRepo(t)
	r := &fakeRunner{stdout: "```diff\n" + sampleDiff + "```\n"}
	e := newExecutor(repo, r)
	e.model = "gpt-5.1"

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	wantArgs := []string{"-s", "--no-ask-user", "--model", "gpt-5.1"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
}

// TestExecute_DetectsDirectWorkspaceMutation is the test for this
// package's one unique safety behavior: if the CLI edits a file directly
// (git status changes) instead of only proposing a diff as text, Execute
// must refuse rather than silently returning as if nothing unusual
// happened.
func TestExecute_DetectsDirectWorkspaceMutation(t *testing.T) {
	repo := initGitRepo(t)
	r := &fakeRunner{
		stdout:     "```diff\n" + sampleDiff + "```\n",
		mutateFile: filepath.Join(repo, "mutated-by-cli.txt"),
	}
	e := newExecutor(repo, r)

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error despite the workspace being mutated directly")
	}
	if !strings.Contains(err.Error(), "modified the workspace directly") {
		t.Errorf("error = %q, want it to name the direct-mutation safety check", err)
	}
}

func TestExecute_ExecutableMissing(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{err: exec.ErrNotFound})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for missing executable")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err)
	}
}

func TestExecute_Timeout(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{err: context.DeadlineExceeded})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want it to mention 'timed out'", err)
	}
}

func TestExecute_Failure(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{err: errors.New("exit status 1"), stderr: "boom on line 3"})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on failure")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Errorf("error = %q, want it to mention 'execution failed'", err)
	}
	if !strings.Contains(err.Error(), "boom on line 3") {
		t.Errorf("error = %q, want it to include captured stderr", err)
	}
}

func TestExecute_FailureWithEmptyStreams(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{err: errors.New("exit status 1")})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on failure")
	}
	if !strings.Contains(err.Error(), "copilot -s") {
		t.Errorf("error = %q, want a concrete next debugging step when both streams are empty", err)
	}
}

func TestExecute_EmptyOutput(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{stdout: "   \n"})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for empty output")
	}
}

func TestExecute_NoDiffInOutput(t *testing.T) {
	repo := initGitRepo(t)
	e := newExecutor(repo, &fakeRunner{stdout: "I could not make the change.\n"})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for output without a diff")
	}
}

func TestExecute_NoWorkspace(t *testing.T) {
	e := NewExecutor("", "")

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error with no workspace configured")
	}
}
