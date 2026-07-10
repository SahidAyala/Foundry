package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
	"foundry/record"
)

// foundryPatch is a pure-addition patch creating FOUNDRY.md; it applies
// cleanly to the fixture repository, which lacks that file.
const foundryPatch = "diff --git a/FOUNDRY.md b/FOUNDRY.md\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/FOUNDRY.md\n" +
	"@@ -0,0 +1 @@\n" +
	"+created by test\n"

// scriptedExecutor injects a deterministic fixture Executor so main's tests
// exercise the full command wiring without requiring Claude Code. Production
// injects the real Claude Code executor (see main.go).
func scriptedExecutor(workspace string) engine.Executor {
	return executor.NewScriptedExecutor(foundryPatch)
}

// chdir changes the process's working directory to dir for the duration
// of the test, restoring the original directory on cleanup. Written by
// hand rather than using testing.T.Chdir (Go 1.24+) because this
// module's go.mod declares go 1.21.
func chdir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) failed: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore os.Chdir(%q) failed: %v", original, err)
		}
	})
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

func TestRun_Do_ApprovedAppliesAndRecords(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGitRepo(t)

	var out bytes.Buffer
	code := run([]string{"do", "add a feature", "--repo", repo}, strings.NewReader("y\n"), &out, scriptedExecutor)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; output:\n%s", code, out.String())
	}

	if _, err := os.Stat(filepath.Join(repo, "FOUNDRY.md")); err != nil {
		t.Errorf("FOUNDRY.md was not applied to the repo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".foundry", "acts")); err != nil {
		t.Errorf("no Act was recorded under .foundry/acts: %v", err)
	}
	if !strings.Contains(out.String(), "Applied and recorded") {
		t.Errorf("output missing confirmation, got:\n%s", out.String())
	}
}

func TestRun_Do_Declined(t *testing.T) {
	repo := initGitRepo(t)

	var out bytes.Buffer
	code := run([]string{"do", "add a feature", "--repo", repo}, strings.NewReader("n\n"), &out, scriptedExecutor)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "FOUNDRY.md")); !os.IsNotExist(err) {
		t.Error("declined run applied FOUNDRY.md to the repo")
	}
	if !strings.Contains(out.String(), "Declined") {
		t.Errorf("output missing decline message, got:\n%s", out.String())
	}
}

func TestRun_Do_FailsToApplyOnNonGitRepo(t *testing.T) {
	repo := t.TempDir() // not a git repo

	var out bytes.Buffer
	code := run([]string{"do", "add a feature", "--repo", repo}, strings.NewReader("y\n"), &out, scriptedExecutor)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1; output:\n%s", code, out.String())
	}
}

// recordedActID returns the ID of the single Act recorded in repo.
func recordedActID(t *testing.T, repo string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repo, ".foundry", "acts"))
	if err != nil {
		t.Fatalf("read acts dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("acts dir has %d entries, want 1", len(entries))
	}
	return entries[0].Name()
}

func TestRun_LogAndShowAfterRecordedAct(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGitRepo(t)

	var out bytes.Buffer
	if code := run([]string{"do", "add a feature", "--repo", repo}, strings.NewReader("y\n"), &out, scriptedExecutor); code != 0 {
		t.Fatalf("do exit code = %d, want 0; output:\n%s", code, out.String())
	}
	actID := recordedActID(t, repo)

	var logOut bytes.Buffer
	if code := run([]string{"log", "--repo", repo}, strings.NewReader(""), &logOut, scriptedExecutor); code != 0 {
		t.Fatalf("log exit code = %d, want 0; output:\n%s", code, logOut.String())
	}
	for _, want := range []string{actID, "add a feature", "pass"} {
		if !strings.Contains(logOut.String(), want) {
			t.Errorf("log output missing %q:\n%s", want, logOut.String())
		}
	}

	var showOut bytes.Buffer
	if code := run([]string{"show", actID, "--repo", repo}, strings.NewReader(""), &showOut, scriptedExecutor); code != 0 {
		t.Fatalf("show exit code = %d, want 0; output:\n%s", code, showOut.String())
	}
	for _, want := range []string{"Act:        " + actID, "Intent:     add a feature", "FOUNDRY.md", "Approved:   by tester"} {
		if !strings.Contains(showOut.String(), want) {
			t.Errorf("show output missing %q:\n%s", want, showOut.String())
		}
	}
}

func TestRun_LogWithoutHistory(t *testing.T) {
	repo := initGitRepo(t)

	var out bytes.Buffer
	if code := run([]string{"log", "--repo", repo}, strings.NewReader(""), &out, scriptedExecutor); code != 0 {
		t.Fatalf("log exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "No acts recorded.") {
		t.Errorf("log output = %q, want 'No acts recorded.'", out.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".foundry")); !os.IsNotExist(err) {
		t.Error("reading history created .foundry; inspection must not write")
	}
}

func TestRun_ShowUnknownAct(t *testing.T) {
	repo := initGitRepo(t)

	var out bytes.Buffer
	if code := run([]string{"show", "deadbeef", "--repo", repo}, strings.NewReader(""), &out, scriptedExecutor); code != 1 {
		t.Fatalf("show exit code = %d, want 1; output:\n%s", code, out.String())
	}
}

func TestRun_LogHelp(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"log", "--help"}, strings.NewReader(""), &out, scriptedExecutor); code != 0 {
		t.Errorf("log --help exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: foundry log") {
		t.Errorf("log help missing usage, got:\n%s", out.String())
	}
}

// TestRun_NoArgs_StartsInteractiveSession pins the product-shape change
// docs/01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md
// proposes: `foundry` with no subcommand at all starts an interactive
// session rooted at the current directory, instead of printing usage
// and exiting 2. chdir is used (not --repo) because the interactive
// surface has no --repo flag — the project is always the process's
// working directory.
func TestRun_NoArgs_StartsInteractiveSession(t *testing.T) {
	repo := initGitRepo(t)
	chdir(t, repo)

	var out bytes.Buffer
	code := run(nil, strings.NewReader("/exit\n"), &out, scriptedExecutor)
	if code != 0 {
		t.Fatalf("run(nil) exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "interactive session") {
		t.Errorf("output missing the session banner, got:\n%s", out.String())
	}
}

// TestRun_NoArgs_InitThenFeature exercises the full command wiring for
// the interactive path the way TestRun_Do_ApprovedAppliesAndRecords
// already does for the one-shot `do` subcommand: /init scaffolds the
// project, /feature runs a Pipeline through to approval and recording,
// /exit ends the session.
func TestRun_NoArgs_InitThenFeature(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGitRepo(t)
	chdir(t, repo)

	var out bytes.Buffer
	code := run(nil, strings.NewReader("/init\n/feature add x\ny\n/exit\n"), &out, scriptedExecutor)
	if code != 0 {
		t.Fatalf("run(nil) exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "Applied and recorded") {
		t.Errorf("output missing confirmation, got:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".foundry", "pipelines", "feature.json")); err != nil {
		t.Errorf("/init did not scaffold feature.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "FOUNDRY.md")); err != nil {
		t.Errorf("/feature did not apply the patch to the repo: %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"bogus"}, strings.NewReader(""), &out, scriptedExecutor); code != 2 {
		t.Errorf("run([\"bogus\"]) exit code = %d, want 2", code)
	}
	if !strings.Contains(out.String(), `unknown command "bogus"`) {
		t.Errorf("unknown-command output missing the bad command name, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Usage: foundry <command>") {
		t.Errorf("unknown-command output missing top-level usage, got:\n%s", out.String())
	}
}

// TestRun_TopLevelHelp guards a real first-run trap: before this, `foundry
// --help` fell through to the "unknown command" branch instead of showing
// usage — the first thing anyone discovering the tool is likely to type.
func TestRun_TopLevelHelp(t *testing.T) {
	for _, flag := range []string{"-h", "--help", "help"} {
		var out bytes.Buffer
		code := run([]string{flag}, strings.NewReader(""), &out, scriptedExecutor)
		if code != 0 {
			t.Errorf("run([%q]) exit code = %d, want 0", flag, code)
		}
		for _, want := range []string{"Usage: foundry <command>", "do ", "log ", "show "} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("run([%q]) output missing %q, got:\n%s", flag, want, out.String())
			}
		}
	}
}

func TestRun_DoHelp(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"do", "--help"}, strings.NewReader(""), &out, scriptedExecutor)
	if code != 0 {
		t.Errorf("run([\"do\", \"--help\"]) exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: foundry do") {
		t.Errorf("help output missing usage text, got:\n%s", out.String())
	}
}

// sequencedExecutor returns its patches in order across Execute calls,
// repeating the last one; it deterministically scripts a first attempt and
// its repair.
type sequencedExecutor struct {
	patches []string
	calls   int
}

func (s *sequencedExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	i := s.calls
	if i >= len(s.patches) {
		i = len(s.patches) - 1
	}
	s.calls++
	return &domain.Outcome{Patch: s.patches[i]}, nil
}

// initGoRepo creates a committed Go module whose user.go defines User; the
// module builds and tests green at HEAD.
func initGoRepo(t *testing.T) string {
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

	files := map[string]string{
		"go.mod":  "module demo\n\ngo 1.21\n",
		"user.go": "package demo\n\ntype User struct {\n\tName string\n}\n\nfunc NewUser(name string) *User {\n\treturn &User{Name: name}\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture file %s: %v", name, err)
		}
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")

	return dir
}

// partialRenamePatch renames only the type declaration, leaving NewUser
// referencing the removed User type: it applies cleanly but cannot build.
const partialRenamePatch = "diff --git a/user.go b/user.go\n" +
	"--- a/user.go\n" +
	"+++ b/user.go\n" +
	"@@ -1,5 +1,5 @@\n" +
	" package demo\n" +
	" \n" +
	"-type User struct {\n" +
	"+type Account struct {\n" +
	" \tName string\n" +
	" }\n"

// fullRenamePatch renames the type and every use; the module builds again.
const fullRenamePatch = "diff --git a/user.go b/user.go\n" +
	"--- a/user.go\n" +
	"+++ b/user.go\n" +
	"@@ -1,9 +1,9 @@\n" +
	" package demo\n" +
	" \n" +
	"-type User struct {\n" +
	"+type Account struct {\n" +
	" \tName string\n" +
	" }\n" +
	" \n" +
	"-func NewUser(name string) *User {\n" +
	"-\treturn &User{Name: name}\n" +
	"+func NewAccount(name string) *Account {\n" +
	"+\treturn &Account{Name: name}\n" +
	" }\n"

// TestRun_Do_DemoRenameWithRepair is the end-to-end demo golden test: the
// first proposed patch breaks the build, verification of the staged patch
// catches it, one bounded repair produces the complete rename, the human
// approves, the patch lands, and the recorded Act shows both iterations.
func TestRun_Do_DemoRenameWithRepair(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGoRepo(t)

	seq := &sequencedExecutor{patches: []string{partialRenamePatch, fullRenamePatch}}
	newExecutor := func(workspace string) engine.Executor { return seq }

	var out bytes.Buffer
	code := run([]string{"do", "rename User to Account", "--repo", repo}, strings.NewReader("y\n"), &out, newExecutor)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if seq.calls != 2 {
		t.Fatalf("Executor called %d times, want 2 (first attempt + one repair)", seq.calls)
	}

	source, err := os.ReadFile(filepath.Join(repo, "user.go"))
	if err != nil {
		t.Fatalf("read patched user.go: %v", err)
	}
	if !strings.Contains(string(source), "type Account struct") || strings.Contains(string(source), "User") {
		t.Errorf("repo does not carry the completed rename:\n%s", source)
	}

	store, err := record.NewFileStore(filepath.Join(repo, ".foundry", "acts"))
	if err != nil {
		t.Fatalf("open record store: %v", err)
	}
	acts, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("store has %d acts, want 1", len(acts))
	}
	act := acts[0]

	if act.JudgmentVerdict != "pass" {
		t.Errorf("recorded JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.Iterations != 2 {
		t.Errorf("recorded Iterations = %d, want 2", act.Iterations)
	}
	if act.Patch != fullRenamePatch {
		t.Errorf("recorded Patch is not the repaired patch:\n%s", act.Patch)
	}

	// The Evidence carries the failed first verification the repair saw.
	if len(act.ConsideredFiles) == 0 {
		t.Fatal("recorded Act has no considered Evidence")
	}
	last := act.ConsideredFiles[len(act.ConsideredFiles)-1]
	if !strings.Contains(last, "failed previous attempt") || !strings.Contains(last, "go-build: fail") {
		t.Errorf("recorded Evidence missing the build findings the repair used, got %q", last)
	}

	// The final, passing round's checked Evidence is recorded on the Act
	// too — `foundry show` must be able to display why a pass verdict
	// held, not only that it did.
	if len(act.CheckedFindings) == 0 {
		t.Fatal("recorded Act has no checked Evidence")
	}
	for _, prefix := range []string{"go-build: pass", "go-test: pass"} {
		found := false
		for _, f := range act.CheckedFindings {
			if strings.HasPrefix(f, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("recorded CheckedFindings = %v, want an entry starting with %q", act.CheckedFindings, prefix)
		}
	}
}
