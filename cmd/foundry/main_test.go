package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foundry/engine"
	"foundry/executor"
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

func TestRun_NoArgs(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, scriptedExecutor); code != 2 {
		t.Errorf("run(nil) exit code = %d, want 2", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"bogus"}, strings.NewReader(""), &out, scriptedExecutor); code != 2 {
		t.Errorf("run([\"bogus\"]) exit code = %d, want 2", code)
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
