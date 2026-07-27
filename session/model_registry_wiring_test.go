package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
	"github.com/SahidAyala/Foundry/project"
	"github.com/SahidAyala/Foundry/session"
)

// TestSession_EngineRoutesModelPinnedStepThroughRegistry proves
// Session.SetModelRegistry's wiring (session.go) end to end (ADR-0013,
// Proposed): a project-local Pipeline pinning a Generate Step's Model
// resolves through the attached model.Registry to the Executor name that
// Model belongs to, then routes exactly as an Executor pin already would —
// never to the Session's default Executor.
func TestSession_EngineRoutesModelPinnedStepThroughRegistry(t *testing.T) {
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

	pinned := &sequencedExecutor{patches: []string{scriptedPatch}}
	construct := func(cfg project.ExecutorConfig, workspace string) (engine.Executor, error) {
		return pinned, nil
	}

	defaultExec := &sequencedExecutor{patches: []string{secondScriptedPatch}}
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{},
		func(string) engine.Executor { return defaultExec }, construct)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	s.SetModelRegistry(models)

	eng, err := s.Engine("modeled")
	if err != nil {
		t.Fatalf(`Engine("modeled") failed: %v`, err)
	}

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if pinned.calls != 1 {
		t.Errorf("model-resolved executor calls = %d, want 1", pinned.calls)
	}
	if defaultExec.calls != 0 {
		t.Errorf("default executor calls = %d, want 0 (the Step names a Model; it must never route to the default)", defaultExec.calls)
	}
}

// TestSession_UnconfiguredModelRegistryLeavesUnpinnedStepsUnaffected
// proves a Session that never calls SetModelRegistry at all (every
// existing caller, before this feature existed) keeps routing unpinned
// Steps to the default Executor exactly as before — backward compatibility
// is not conditional on any new wiring being present.
func TestSession_UnconfiguredModelRegistryLeavesUnpinnedStepsUnaffected(t *testing.T) {
	root := initGitRepo(t)

	defaultExec := &sequencedExecutor{patches: []string{scriptedPatch}}
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{},
		func(string) engine.Executor { return defaultExec })
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	eng, err := s.Engine("default")
	if err != nil {
		t.Fatalf(`Engine("default") failed: %v`, err)
	}
	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if defaultExec.calls != 1 {
		t.Errorf("default executor calls = %d, want 1", defaultExec.calls)
	}
}
