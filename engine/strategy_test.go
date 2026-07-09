package engine_test

import (
	"context"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

// TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges pins RFC-0002
// §9 Phase 3's exit criterion: a Pipeline shaped differently from
// DefaultPipeline (here, two verify Steps run back-to-back against the same
// Outcome) executes correctly through the unmodified Engine and
// PipelineStrategy — no Engine or Strategy code was written for this
// specific shape.
func TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges(t *testing.T) {
	custom := engine.Pipeline{
		Name: "review",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "verify-again", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 0},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", custom)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	// One generate Step and two verify Steps were recorded, in order.
	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindVerify}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestPipelineStrategy_UnrecognizedStepKindFails verifies a Pipeline
// referencing a Step Kind PipelineStrategy does not recognize fails with a
// clear, named error instead of silently skipping the Step.
func TestPipelineStrategy_UnrecognizedStepKindFails(t *testing.T) {
	malformed := engine.Pipeline{
		Name: "malformed",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "mystery", Kind: "transmute"},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", malformed)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unrecognized step kind returned nil error")
	}
	if !strings.Contains(err.Error(), "transmute") || !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error = %q, want it to name the unrecognized kind %q and step %q", err.Error(), "transmute", "mystery")
	}
}

// TestPipelineStrategy_VerifyWithoutPrecedingGenerateFails verifies a
// verify Step with no generate Step before it fails with a clear error
// instead of handing a nil Outcome to the Verifier port.
func TestPipelineStrategy_VerifyWithoutPrecedingGenerateFails(t *testing.T) {
	verifyOnly := engine.Pipeline{
		Name:  "verify-only",
		Steps: []engine.Step{{ID: "verify", Kind: domain.StepKindVerify}},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", verifyOnly)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with a verify-only pipeline returned nil error")
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Errorf("error = %q, want it to mention the verify step", err.Error())
	}
}

// TestPipelineStrategy_NoVerifyStepFails verifies a Pipeline with no verify
// Step at all — which can never produce a Judgment — fails with a clear
// error instead of a nil-pointer panic.
func TestPipelineStrategy_NoVerifyStepFails(t *testing.T) {
	generateOnly := engine.Pipeline{
		Name:  "generate-only",
		Steps: []engine.Step{{ID: "generate", Kind: domain.StepKindGenerate}},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", generateOnly)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with a generate-only pipeline returned nil error")
	}
	if !strings.Contains(err.Error(), "generate-only") {
		t.Errorf("error = %q, want it to name the pipeline %q", err.Error(), "generate-only")
	}
}
