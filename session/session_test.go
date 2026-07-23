package session_test

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
	"foundry/project"
	"foundry/session"
)

// initGitRepo creates a temporary git repository with one committed file,
// mirroring cli_test.go's own helper of the same name — every package
// that needs a real repository for workspace.StagedVerifier defines this
// locally rather than sharing a test-utility package.
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

const scriptedPatch = "diff --git a/session_test_file.txt b/session_test_file.txt\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/session_test_file.txt\n" +
	"@@ -0,0 +1 @@\n" +
	"+created by test\n"

const secondScriptedPatch = "diff --git a/session_test_file_two.txt b/session_test_file_two.txt\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/session_test_file_two.txt\n" +
	"@@ -0,0 +1 @@\n" +
	"+created by test\n"

func newScriptedExecutorFactory(patch string) session.NewExecutor {
	return func(root string) engine.Executor {
		return executor.NewScriptedExecutor(patch)
	}
}

// sequencedExecutor returns one canned patch per Execute call, in order —
// for tests where a Session's single shared Executor must still produce
// distinct Outcomes across more than one Pipeline run.
type sequencedExecutor struct {
	patches []string
	calls   int
}

func (e *sequencedExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	patch := e.patches[e.calls]
	e.calls++
	return &domain.Outcome{Patch: patch}, nil
}

func newSequencedExecutorFactory(patches ...string) session.NewExecutor {
	return func(root string) engine.Executor {
		return &sequencedExecutor{patches: patches}
	}
}

func TestNewSession_ResolvesBuiltinPipelines(t *testing.T) {
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	for _, name := range []string{"default", "review"} {
		if _, err := s.Engine(name); err != nil {
			t.Errorf("Engine(%q) failed: %v", name, err)
		}
	}
}

// TestNewSession_NotInteractiveForNonFileIO confirms the gate ADR-0012's
// rich REPL line editor depends on (Session.Interactive) computes false
// for every non-*os.File input/output — exactly what every other test in
// this package (and every existing test in session_test.go's own
// suite) passes, so this documents why none of them ever exercise
// cli.ReadInteractiveLine.
func TestNewSession_NotInteractiveForNonFileIO(t *testing.T) {
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if s.Interactive {
		t.Error("Interactive = true for a *bytes.Buffer-backed Session, want false")
	}
}

func TestSession_Initialized(t *testing.T) {
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if s.Initialized() {
		t.Error("Initialized() = true before /init has ever run, want false")
	}

	if err := (project.ProjectLoader{}).Scaffold(root); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}
	if !s.Initialized() {
		t.Error("Initialized() = false after Scaffold wrote .foundry/pipelines, want true")
	}
}

func TestSession_Engine_UnknownPipelineNameFailsWithClearError(t *testing.T) {
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = s.Engine("feature")
	if err == nil {
		t.Fatal("Engine(\"feature\") on an uninitialized project returned nil error")
	}
	if !strings.Contains(err.Error(), "feature") {
		t.Errorf("error = %q, want it to name the missing pipeline %q", err.Error(), "feature")
	}
	if !strings.Contains(err.Error(), "init") {
		t.Errorf("error = %q, want it to point at /init", err.Error())
	}
}

// TestSession_RunsDefaultAndReviewIndependentlyWithoutContamination proves
// two different built-in Pipelines run correctly, back-to-back, through
// the same Session — extending engine/review_pipeline_test.go's
// coexistence proof one layer up: a Session's shared Gatherer/Verifier/
// Executor are safe to reuse across Pipelines because engine.NewEngine
// itself carries no state between calls.
func TestSession_RunsDefaultAndReviewIndependentlyWithoutContamination(t *testing.T) {
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	defaultEngine, err := s.Engine("default")
	if err != nil {
		t.Fatalf("Engine(\"default\") failed: %v", err)
	}
	defaultAct, err := defaultEngine.Run(context.Background(), &domain.Intent{Text: "add a feature"})
	if err != nil {
		t.Fatalf("default Run failed: %v", err)
	}
	if defaultAct.JudgmentVerdict != "pass" {
		t.Errorf("default JudgmentVerdict = %q, want %q", defaultAct.JudgmentVerdict, "pass")
	}

	reviewEngine, err := s.Engine("review")
	if err != nil {
		t.Fatalf("Engine(\"review\") failed: %v", err)
	}
	reviewAct, err := reviewEngine.Run(context.Background(), &domain.Intent{Text: "review a change"})
	if err != nil {
		t.Fatalf("review Run failed: %v", err)
	}
	if reviewAct.JudgmentVerdict != "pass" {
		t.Errorf("review JudgmentVerdict = %q, want %q", reviewAct.JudgmentVerdict, "pass")
	}
	if len(reviewAct.Steps) != 3 {
		t.Errorf("review Steps = %+v, want 3 entries (generate, verify, verify-again)", reviewAct.Steps)
	}

	// Running "review" after "default" must not have changed what
	// "default" resolves to.
	defaultEngineAgain, err := s.Engine("default")
	if err != nil {
		t.Fatalf("second Engine(\"default\") failed: %v", err)
	}
	defaultActAgain, err := defaultEngineAgain.Run(context.Background(), &domain.Intent{Text: "add another feature"})
	if err != nil {
		t.Fatalf("second default Run failed: %v", err)
	}
	if defaultActAgain.JudgmentVerdict != "pass" {
		t.Errorf("second default JudgmentVerdict = %q, want %q", defaultActAgain.JudgmentVerdict, "pass")
	}
}
