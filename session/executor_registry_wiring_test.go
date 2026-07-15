package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/project"
	"foundry/session"
)

// TestSession_EngineRoutesPinnedStepToNamedExecutor proves Session.Engine's
// Router wiring (session.go) end to end: a project-local Pipeline pinning a
// Generate Step's executor to a name declared in .foundry/executors.json
// actually routes that Step to the Executor NewSession's newNamedExecutor
// constructed for it — never to the Session's default Executor.
func TestSession_EngineRoutesPinnedStepToNamedExecutor(t *testing.T) {
	root := initGitRepo(t)

	pipelinesDir := filepath.Join(root, ".foundry", "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	pipelineDoc := `{
		"name": "routed",
		"steps": [
			{"id": "generate", "kind": "generate", "executor": "pinned-vendor"},
			{"id": "verify", "kind": "verify"}
		]
	}`
	if err := os.WriteFile(filepath.Join(pipelinesDir, "routed.json"), []byte(pipelineDoc), 0o644); err != nil {
		t.Fatalf("write pipeline document: %v", err)
	}

	executorsDoc := `{"pinned-vendor": {"vendor": "test", "model": "whatever"}}`
	if err := os.WriteFile(filepath.Join(root, ".foundry", "executors.json"), []byte(executorsDoc), 0o644); err != nil {
		t.Fatalf("write executors config: %v", err)
	}

	pinned := &sequencedExecutor{patches: []string{scriptedPatch}}
	construct := func(cfg project.ExecutorConfig) (engine.Executor, error) {
		if cfg.Vendor != "test" {
			t.Fatalf("construct received vendor %q, want %q", cfg.Vendor, "test")
		}
		return pinned, nil
	}

	defaultExec := &sequencedExecutor{patches: []string{secondScriptedPatch}}
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{},
		func(string) engine.Executor { return defaultExec }, construct)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	eng, err := s.Engine("routed")
	if err != nil {
		t.Fatalf(`Engine("routed") failed: %v`, err)
	}

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if pinned.calls != 1 {
		t.Errorf("pinned executor calls = %d, want 1", pinned.calls)
	}
	if defaultExec.calls != 0 {
		t.Errorf("default executor calls = %d, want 0 (the Step is pinned; it must never route to the default)", defaultExec.calls)
	}
}

// TestSession_EngineRoutesUnpinnedStepToDefault pins the
// backward-compatibility guarantee: a project that configures
// .foundry/executors.json but runs a Pipeline whose Steps declare no pin
// (every built-in Pipeline) still routes to the Session's default Executor,
// unaffected by the named Executors' existence.
func TestSession_EngineRoutesUnpinnedStepToDefault(t *testing.T) {
	root := initGitRepo(t)

	executorsDoc := `{"pinned-vendor": {"vendor": "test", "model": "whatever"}}`
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".foundry", "executors.json"), []byte(executorsDoc), 0o644); err != nil {
		t.Fatalf("write executors config: %v", err)
	}

	construct := func(cfg project.ExecutorConfig) (engine.Executor, error) {
		return &sequencedExecutor{patches: []string{scriptedPatch}}, nil
	}

	defaultExec := &sequencedExecutor{patches: []string{scriptedPatch}}
	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{},
		func(string) engine.Executor { return defaultExec }, construct)
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
