package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// A successfully applied Act must never leave the developer's repo on
	// a throwaway branch, nor leave that branch lying around.
	branch := gitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "main" {
		t.Errorf("repo left on branch %q, want %q", branch, "main")
	}
	if list := gitOutput(t, repo, "branch", "--list", "foundry/act-*"); list != "" {
		t.Errorf("throwaway branch left behind: %q", list)
	}
}

// gitOutput runs git with args in dir and returns its trimmed output,
// failing the test on error.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
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
	for _, want := range []string{"Proposed patch:", "APPLIED.md", "Verdict: ✓ pass", "Approve and apply? (y/n)"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q, got:\n%s", want, output)
		}
	}
}

// newHistoryStore returns a FileStore preloaded with count Acts created an
// hour apart, IDs act-0 (oldest) through act-<count-1> (newest).
func newHistoryStore(t *testing.T, count int) *record.FileStore {
	t.Helper()

	store, err := record.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("record.NewFileStore failed: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		act := &domain.Act{
			ID:              fmt.Sprintf("act-%d", i),
			Intent:          fmt.Sprintf("intent %d", i),
			CreatedAt:       base.Add(time.Duration(i) * time.Hour),
			JudgmentVerdict: "pass",
		}
		if err := store.Write(context.Background(), act); err != nil {
			t.Fatalf("store.Write failed: %v", err)
		}
	}
	return store
}

func TestCLI_Log_NewestFirstWithLimit(t *testing.T) {
	var out bytes.Buffer
	c := cli.NewCLI(nil, newHistoryStore(t, 3), strings.NewReader(""), &out)

	if err := c.Log(context.Background(), 2); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("Log printed %d lines, want 2:\n%s", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "act-2") || !strings.HasPrefix(lines[1], "act-1") {
		t.Errorf("Log order wrong, want act-2 then act-1:\n%s", out.String())
	}
	if !strings.Contains(lines[0], "intent 2") || !strings.Contains(lines[0], "pass") {
		t.Errorf("Log line missing intent or verdict: %q", lines[0])
	}
}

func TestCLI_Log_EmptyStore(t *testing.T) {
	var out bytes.Buffer
	c := cli.NewCLI(nil, newHistoryStore(t, 0), strings.NewReader(""), &out)

	if err := c.Log(context.Background(), 10); err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if !strings.Contains(out.String(), "No acts recorded.") {
		t.Errorf("Log on empty store printed %q", out.String())
	}
}

func TestCLI_Log_RejectsNonPositiveLimit(t *testing.T) {
	c := cli.NewCLI(nil, newHistoryStore(t, 1), strings.NewReader(""), &bytes.Buffer{})

	if err := c.Log(context.Background(), 0); err == nil {
		t.Fatal("Log with limit 0 returned nil error")
	}
}

func TestCLI_Show_PrintsFullAct(t *testing.T) {
	var out bytes.Buffer
	c := cli.NewCLI(nil, newHistoryStore(t, 2), strings.NewReader(""), &out)

	if err := c.Show(context.Background(), "act-1"); err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	for _, want := range []string{"Act:        act-1", "Intent:     intent 1", "Verdict:    pass", "Patch:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Show output missing %q:\n%s", want, out.String())
		}
	}
}

func TestCLI_Show_UnknownID(t *testing.T) {
	c := cli.NewCLI(nil, newHistoryStore(t, 1), strings.NewReader(""), &bytes.Buffer{})

	if err := c.Show(context.Background(), "no-such-act"); err == nil {
		t.Fatal("Show with unknown ID returned nil error")
	}
}

func TestCLI_Do_RepoPathMustExist(t *testing.T) {
	var out bytes.Buffer
	c, _ := newCLI(t, "/nonexistent/repo/path", newFilePatch("APPLIED.md"), "exit 0", "y\n", &out)

	if err := c.Do(context.Background(), "add a feature", "/nonexistent/repo/path"); err == nil {
		t.Fatal("Do with nonexistent repo path returned nil error")
	}
}
