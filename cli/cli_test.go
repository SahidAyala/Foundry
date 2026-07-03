package cli_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foundry/cli"
	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
	"foundry/record"
	"foundry/verify"
)

type emptyGatherer struct{}

func (emptyGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	return nil, nil
}

// newFilePatch creates a new file at name containing a single line. As a pure
// addition it applies cleanly to any repository lacking the file.
func newFilePatch(name string) string {
	return "diff --git a/" + name + " b/" + name + "\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/" + name + "\n" +
		"@@ -0,0 +1 @@\n" +
		"+created by test\n"
}

// initGitRepo creates a temporary git repository with one committed file.
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")

	return dir
}

func TestParseArgs_Valid(t *testing.T) {
	intent, repoPath, err := cli.ParseArgs([]string{"add a feature", "--repo", "/tmp/repo"})
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	if intent != "add a feature" {
		t.Errorf("intent = %q, want %q", intent, "add a feature")
	}
	if repoPath != "/tmp/repo" {
		t.Errorf("repoPath = %q, want %q", repoPath, "/tmp/repo")
	}
}

func TestParseArgs_RepoBeforeIntent(t *testing.T) {
	intent, repoPath, err := cli.ParseArgs([]string{"--repo", "/tmp/repo", "add a feature"})
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	if intent != "add a feature" || repoPath != "/tmp/repo" {
		t.Errorf("ParseArgs = (%q, %q), want (%q, %q)", intent, repoPath, "add a feature", "/tmp/repo")
	}
}

func TestParseArgs_RepoEqualsForm(t *testing.T) {
	intent, repoPath, err := cli.ParseArgs([]string{"add a feature", "--repo=/tmp/repo"})
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	if intent != "add a feature" || repoPath != "/tmp/repo" {
		t.Errorf("ParseArgs = (%q, %q), want (%q, %q)", intent, repoPath, "add a feature", "/tmp/repo")
	}
}

func TestParseArgs_MissingRepo(t *testing.T) {
	if _, _, err := cli.ParseArgs([]string{"add a feature"}); err == nil {
		t.Fatal("ParseArgs without --repo returned nil error")
	}
}

func TestParseArgs_MissingIntent(t *testing.T) {
	if _, _, err := cli.ParseArgs([]string{"--repo", "/tmp/repo"}); err == nil {
		t.Fatal("ParseArgs without an intent returned nil error")
	}
}

func TestParseArgs_TooManyPositional(t *testing.T) {
	if _, _, err := cli.ParseArgs([]string{"one", "two", "--repo", "/tmp/repo"}); err == nil {
		t.Fatal("ParseArgs with two positional arguments returned nil error")
	}
}

func TestParseArgs_Help(t *testing.T) {
	if _, _, err := cli.ParseArgs([]string{"--help"}); err != cli.ErrHelp {
		t.Fatalf("ParseArgs(--help) error = %v, want cli.ErrHelp", err)
	}
}

func newCLI(t *testing.T, repoPath, patch, validatorCmd string, in string, out *bytes.Buffer) (*cli.CLI, *record.FileStore) {
	t.Helper()

	store, err := record.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("record.NewFileStore failed: %v", err)
	}
	gate, err := verify.NewGate("all-pass", &verify.Validator{Name: "check", Cmd: validatorCmd})
	if err != nil {
		t.Fatalf("verify.NewGate failed: %v", err)
	}
	eng := engine.NewEngine(emptyGatherer{}, executor.NewScriptedExecutor(patch), gate, repoPath)
	return cli.NewCLI(eng, store, strings.NewReader(in), out), store
}

func TestCLI_Do_ApprovedAppliesAndRecords(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGitRepo(t)

	var out bytes.Buffer
	c, store := newCLI(t, repo, newFilePatch("APPLIED.md"), "exit 0", "y\n", &out)

	if err := c.Do(context.Background(), "add a feature", repo); err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "APPLIED.md")); err != nil {
		t.Errorf("patch was not applied to the repo: %v", err)
	}

	acts, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("store has %d acts, want 1", len(acts))
	}
	if acts[0].ApprovedBy != "tester" {
		t.Errorf("recorded ApprovedBy = %q, want %q", acts[0].ApprovedBy, "tester")
	}
	if acts[0].ApprovedAt == nil {
		t.Error("recorded ApprovedAt is nil, want a timestamp")
	}
	if !strings.Contains(out.String(), "Applied and recorded") {
		t.Errorf("output missing confirmation, got:\n%s", out.String())
	}
}

func TestCLI_Do_DeclinedDoesNothing(t *testing.T) {
	repo := initGitRepo(t)

	var out bytes.Buffer
	c, store := newCLI(t, repo, newFilePatch("APPLIED.md"), "exit 0", "n\n", &out)

	if err := c.Do(context.Background(), "add a feature", repo); err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "APPLIED.md")); !os.IsNotExist(err) {
		t.Error("declined Act applied its patch to the repo")
	}

	acts, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("declined Act was recorded (%d acts), want 0", len(acts))
	}
	if !strings.Contains(out.String(), "Declined") {
		t.Errorf("output missing decline message, got:\n%s", out.String())
	}
}

func TestCLI_Do_ShowsPatchAndVerdict(t *testing.T) {
	repo := initGitRepo(t)

	var out bytes.Buffer
	c, _ := newCLI(t, repo, newFilePatch("APPLIED.md"), "exit 0", "n\n", &out)

	if err := c.Do(context.Background(), "add a feature", repo); err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{"Proposed patch:", "APPLIED.md", "Verdict: pass", "Approve and apply? (y/n)"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q, got:\n%s", want, output)
		}
	}
}

func TestCLI_Do_RepoPathMustExist(t *testing.T) {
	var out bytes.Buffer
	c, _ := newCLI(t, "/nonexistent/repo/path", newFilePatch("APPLIED.md"), "exit 0", "y\n", &out)

	if err := c.Do(context.Background(), "add a feature", "/nonexistent/repo/path"); err == nil {
		t.Fatal("Do with nonexistent repo path returned nil error")
	}
}
