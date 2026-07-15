package engine_test

import (
	"context"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

// applyPipeline is a minimal Pipeline with the shape every apply Step
// needs to actually run: a generate Step to produce an Outcome and an
// approve Step to satisfy runSteps' "apply requires an accepted approve
// step first" check, then the apply Step under test.
func applyPipeline(target string) engine.Pipeline {
	return engine.Pipeline{
		Name: "apply-routing",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply, Target: target},
		},
	}
}

// TestEngine_ApplyStepWithNoTargetUsesConfiguredApplier pins the
// backward-compatibility guarantee RFC-0004 §2.6's Target field rests on: an
// apply Step declaring no Target — every Pipeline shipped before Target
// existed — resolves to the Engine's single SetApplier-configured Applier,
// never touching an ApplierRegistry at all (SetApplierRegistry is never
// even called here).
func TestEngine_ApplyStepWithNoTargetUsesConfiguredApplier(t *testing.T) {
	local := &fakeApplier{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, executor.NewScriptedExecutor(scriptedPatch), &fakeVerifier{verdict: "pass"}, "", applyPipeline(""))
	eng.SetAuthority(&fakeAuthority{approved: true})
	eng.SetApplier(local)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if local.calls != 1 {
		t.Errorf("local Applier.Apply called %d times, want 1", local.calls)
	}
}

// TestEngine_ApplyStepWithRegisteredTargetRoutesToNamedApplier proves an
// apply Step declaring a Target other than "local" resolves through the
// Engine's ApplierRegistry instead of its single configured Applier — the
// named Applier is called, the default is not.
func TestEngine_ApplyStepWithRegisteredTargetRoutesToNamedApplier(t *testing.T) {
	local := &fakeApplier{}
	named := &fakeApplier{}

	registry := engine.NewApplierRegistry()
	if err := registry.Register("knowledge-note", named); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, executor.NewScriptedExecutor(scriptedPatch), &fakeVerifier{verdict: "pass"}, "", applyPipeline("knowledge-note"))
	eng.SetAuthority(&fakeAuthority{approved: true})
	eng.SetApplier(local)
	eng.SetApplierRegistry(registry)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if named.calls != 1 {
		t.Errorf("named Applier.Apply called %d times, want 1", named.calls)
	}
	if local.calls != 0 {
		t.Errorf("default Applier.Apply called %d times, want 0 (never the default)", local.calls)
	}
}

// TestEngine_ApplyStepWithUnregisteredTargetFails proves a Target that
// cannot be resolved is a clear, named error — never a silent fallback to
// the default Applier, mirroring Router.Resolve's own unresolved-pin
// behavior for Executor pins.
func TestEngine_ApplyStepWithUnregisteredTargetFails(t *testing.T) {
	local := &fakeApplier{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, executor.NewScriptedExecutor(scriptedPatch), &fakeVerifier{verdict: "pass"}, "", applyPipeline("project-doc"))
	eng.SetAuthority(&fakeAuthority{approved: true})
	eng.SetApplier(local)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unregistered apply Target returned nil error")
	}
	if local.calls != 0 {
		t.Errorf("default Applier.Apply called %d times, want 0 (refused before falling back)", local.calls)
	}
	if !strings.Contains(err.Error(), "project-doc") {
		t.Errorf("error = %q, want it to mention the unresolved target %q", err.Error(), "project-doc")
	}
}
