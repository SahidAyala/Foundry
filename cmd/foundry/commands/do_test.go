package commands

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/executor"
	"github.com/SahidAyala/Foundry/executor/claude"
	"github.com/SahidAyala/Foundry/model"
	"github.com/SahidAyala/Foundry/project"
	"github.com/SahidAyala/Foundry/vcs"
	"github.com/SahidAyala/Foundry/verify"
)

// initGitRepo creates a temporary git repository with one committed file
// — mirroring session_test.go's own helper of the same name — deliberately
// with no go.mod, so verify.DefaultValidators(root) returns zero
// Validators and the Gate trivially passes; these tests care about
// routing, not verification.
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

// TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured proves
// wireEngine's buildApplierRegistry (ADR-0010, Piece 6 of
// docs/04-guides/multi-executor-router-implementation-plan.md) registers
// engine.ApplyTargetRemotePR only when project.Config names a
// RemotePublishTokenEnv, mirroring session.Session's own
// buildApplierRegistry.
func TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured(t *testing.T) {
	registry, err := buildApplierRegistry("/repo", project.Config{RemotePublishTokenEnv: "GITHUB_PR_TOKEN"})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	if _, err := registry.Get(engine.ApplyTargetRemotePR); err != nil {
		t.Errorf("Get(%q) failed, want the remote-pr Applier registered: %v", engine.ApplyTargetRemotePR, err)
	}
}

// TestBuildApplierRegistry_WiresSummarizerWhenPRSummaryModelSet covers
// project.Config.PRSummaryModel: when set alongside RemotePublishTokenEnv,
// the registered remote-pr Applier's Summarizer is a real
// claude.Summarizer, not left nil (Apply's own mechanical default).
func TestBuildApplierRegistry_WiresSummarizerWhenPRSummaryModelSet(t *testing.T) {
	registry, err := buildApplierRegistry("/repo", project.Config{RemotePublishTokenEnv: "GITHUB_PR_TOKEN", PRSummaryModel: "haiku"})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	applier, err := registry.Get(engine.ApplyTargetRemotePR)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", engine.ApplyTargetRemotePR, err)
	}
	prApplier, ok := applier.(vcs.GitHubPRApplier)
	if !ok {
		t.Fatalf("registered Applier = %T, want vcs.GitHubPRApplier", applier)
	}
	if _, ok := prApplier.Summarizer.(*claude.Summarizer); !ok {
		t.Errorf("Summarizer = %T, want *claude.Summarizer", prApplier.Summarizer)
	}
}

// TestBuildValidators_UsesProjectDeclaredValidatorsWhenSet covers
// project.Config.Validators: when non-empty, it replaces
// verify.DefaultValidators' own root-only auto-detection entirely — the
// escape hatch for a project (e.g. a polyglot monorepo) that
// auto-detection can't handle correctly.
func TestBuildValidators_UsesProjectDeclaredValidatorsWhenSet(t *testing.T) {
	cfg := project.Config{Validators: []project.ValidatorConfig{
		{Name: "events-test", Cmd: "cd events && go test ./..."},
		{Name: "ui-test", Cmd: "cd ui && npm test"},
	}}
	got := buildValidators("/repo", cfg)
	if len(got) != 2 {
		t.Fatalf("len(validators) = %d, want 2", len(got))
	}
	if got[0].Name != "events-test" || got[0].Cmd != "cd events && go test ./..." {
		t.Errorf("validators[0] = %+v, want {events-test, cd events && go test ./...}", got[0])
	}
	if got[1].Name != "ui-test" || got[1].Cmd != "cd ui && npm test" {
		t.Errorf("validators[1] = %+v, want {ui-test, cd ui && npm test}", got[1])
	}
}

// TestBuildValidators_FallsBackToAutoDetectionWhenUnset covers the
// backward-compatible default: an empty Validators list (every project
// before this field existed) falls back to verify.DefaultValidators
// exactly as before.
func TestBuildValidators_FallsBackToAutoDetectionWhenUnset(t *testing.T) {
	root := t.TempDir()
	got := buildValidators(root, project.Config{})
	want := verify.DefaultValidators(root)
	if len(got) != len(want) {
		t.Fatalf("len(validators) = %d, want %d (verify.DefaultValidators)", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Cmd != want[i].Cmd {
			t.Errorf("validators[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig verifies a
// project that never sets remote_publish_token_env registers no
// remote-pr Applier at all — exactly as an apply Step with that Target
// would have behaved before ADR-0010 existed.
func TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig(t *testing.T) {
	registry, err := buildApplierRegistry("/repo", project.Config{})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	if _, err := registry.Get(engine.ApplyTargetRemotePR); err == nil {
		t.Error("Get(remote-pr) succeeded, want an error: no Applier should be registered without config")
	}
	// Knowledge-lite capture's own target still registers unconditionally.
	if _, err := registry.Get(engine.ApplyTargetKnowledgeNote); err != nil {
		t.Errorf("Get(knowledge-note) failed: %v", err)
	}
}

// TestWireEngine_ResolvesProjectLocalPipeline covers a real gap: wireEngine
// used to resolve pipelineName from engine.NewDefaultRegistry() alone —
// Foundry's built-in Pipelines only ("default", "review"). `foundry do`
// itself never needed more (it always asks for "default"), but `foundry
// resume` uses this same function with whatever Pipeline name a
// checkpoint happens to name — and an interactive session
// (session.NewSession) runs project-local Pipelines like "feature",
// "bugfix", and "release" (the very starters project.ProjectLoader.Scaffold
// writes) every day. Resuming one of those would fail with "pipeline not
// registered" even with a valid checkpoint present. wireEngine now
// resolves from the project's full registry (built-in plus project-local,
// project.ProjectLoader.LoadRegistry) instead.
func TestWireEngine_ResolvesProjectLocalPipeline(t *testing.T) {
	root := t.TempDir()
	if err := (project.ProjectLoader{}).Scaffold(root); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	newExecutor := func(workspace string) engine.Executor { return executor.NewScriptedExecutor("") }

	for _, name := range []string{"default", "review", "feature", "bugfix", "release", "issue"} {
		eng, _, _, err := wireEngine(context.Background(), root, strings.NewReader(""), &bytes.Buffer{}, newExecutor, nil, name)
		if err != nil {
			t.Errorf("wireEngine(%q) failed: %v", name, err)
			continue
		}
		if eng == nil {
			t.Errorf("wireEngine(%q) returned a nil Engine", name)
		}
	}
}

// TestWireEngine_AIReviewModelRequiresBaseURL covers a real configuration
// gap: ai_review_model names an OpenAI-Chat-Completions-compatible model,
// but there is no single "default" endpoint across vendors (OpenAI,
// Gemini's API, Ollama, Groq, DeepSeek all differ) to fall back to —
// setting the model without the endpoint must be a clear, named
// configuration error, not a nil verifier silently doing nothing.
func TestWireEngine_AIReviewModelRequiresBaseURL(t *testing.T) {
	root := t.TempDir()
	if err := (project.ProjectLoader{}).Scaffold(root); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".foundry", "config.json"), []byte(`{"ai_review_model": "gpt-5.1"}`), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	newExecutor := func(workspace string) engine.Executor { return executor.NewScriptedExecutor("") }
	_, _, _, err := wireEngine(context.Background(), root, strings.NewReader(""), &bytes.Buffer{}, newExecutor, nil, "default")
	if err == nil {
		t.Fatal("wireEngine with ai_review_model but no ai_review_base_url returned nil error")
	}
	if !strings.Contains(err.Error(), "ai_review_base_url") {
		t.Errorf("error = %q, want it to name the missing ai_review_base_url field", err)
	}
}

const wireEngineScriptedPatch = "diff --git a/wire_engine_test_file.txt b/wire_engine_test_file.txt\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/wire_engine_test_file.txt\n" +
	"@@ -0,0 +1 @@\n" +
	"+created by test\n"

const wireEngineDefaultPatch = "diff --git a/wire_engine_test_default.txt b/wire_engine_test_default.txt\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/wire_engine_test_default.txt\n" +
	"@@ -0,0 +1 @@\n" +
	"+created by test\n"

// TestWireEngine_ModelPinnedStepRoutesThroughModelRegistry proves
// wireEngine's models parameter (ADR-0013, Proposed) is actually threaded
// into the Router: a Pipeline Step naming Model resolves through the
// passed model.Registry to its Executor, then routes exactly as an
// Executor pin already would — never to the default Executor.
func TestWireEngine_ModelPinnedStepRoutesThroughModelRegistry(t *testing.T) {
	root := initGitRepo(t)

	pipelinesDir := filepath.Join(root, ".foundry", "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	pipelineDoc := `{
		"name": "modeled",
		"steps": [
			{"id": "generate", "kind": "generate", "model": "gemini-3.5-flash"},
			{"id": "verify", "kind": "verify"}
		]
	}`
	if err := os.WriteFile(filepath.Join(pipelinesDir, "modeled.json"), []byte(pipelineDoc), 0o644); err != nil {
		t.Fatalf("write pipeline document: %v", err)
	}

	executorsDoc := `{"planner": {"vendor": "test", "model": "whatever"}}`
	if err := os.WriteFile(filepath.Join(root, ".foundry", "executors.json"), []byte(executorsDoc), 0o644); err != nil {
		t.Fatalf("write executors config: %v", err)
	}

	construct := func(cfg project.ExecutorConfig, workspace string) (engine.Executor, error) {
		return executor.NewScriptedExecutor(wireEngineScriptedPatch), nil
	}
	newExecutor := func(workspace string) engine.Executor { return executor.NewScriptedExecutor(wireEngineDefaultPatch) }

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	eng, _, _, err := wireEngine(context.Background(), root, strings.NewReader(""), &bytes.Buffer{}, newExecutor, construct, "modeled", models)
	if err != nil {
		t.Fatalf("wireEngine failed: %v", err)
	}

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.Patch != wireEngineScriptedPatch {
		t.Error("Run did not route the Model-pinned Step to the Executor the model.Registry names")
	}
}
