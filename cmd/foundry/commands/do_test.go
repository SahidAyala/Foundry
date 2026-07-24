package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/engine"
	"foundry/executor"
	"foundry/project"
)

// TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured proves
// wireEngine's buildApplierRegistry (ADR-0010, Piece 6 of
// docs/04-guides/multi-executor-router-implementation-plan.md) registers
// engine.ApplyTargetRemotePR only when project.Config names a
// RemotePublishTokenEnv, mirroring session.Session's own
// buildApplierRegistry.
func TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured(t *testing.T) {
	registry, err := buildApplierRegistry(project.Config{RemotePublishTokenEnv: "GITHUB_PR_TOKEN"})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	if _, err := registry.Get(engine.ApplyTargetRemotePR); err != nil {
		t.Errorf("Get(%q) failed, want the remote-pr Applier registered: %v", engine.ApplyTargetRemotePR, err)
	}
}

// TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig verifies a
// project that never sets remote_publish_token_env registers no
// remote-pr Applier at all — exactly as an apply Step with that Target
// would have behaved before ADR-0010 existed.
func TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig(t *testing.T) {
	registry, err := buildApplierRegistry(project.Config{})
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
