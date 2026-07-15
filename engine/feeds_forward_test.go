package engine_test

import (
	"context"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// TestPipelineStrategy_FeedsForwardAppendsPrecedingStepOutput proves a Step
// declaring FeedsForward: true has the immediately preceding Step's
// recorded output — here, a Verify Step's Checked findings — appended to
// its own Context, while the Step before it (which declares no
// FeedsForward) sees no such addition.
func TestPipelineStrategy_FeedsForwardAppendsPrecedingStepOutput(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "feeds-forward",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "generate-again", Kind: domain.StepKindGenerate, FeedsForward: true},
		},
	}

	exec := &captureExecutor{patches: []string{"patch-1", "patch-2"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "pass", Checked: []string{"build: pass", "tests: pass"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", pipeline)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if len(exec.calls) != 2 {
		t.Fatalf("Executor called %d times, want 2", len(exec.calls))
	}

	if len(exec.calls[0]) != 1 {
		t.Errorf("generate's considered = %v, want only the gathered files (no feeds_forward)", exec.calls[0])
	}

	found := false
	for _, considered := range exec.calls[1] {
		if strings.Contains(considered, "build: pass") && strings.Contains(considered, "tests: pass") {
			found = true
		}
	}
	if !found {
		t.Errorf("generate-again's considered = %v, want it to include the verify step's checked findings", exec.calls[1])
	}
}

// TestPipelineStrategy_FeedsForwardDoesNotThreadPastItsOwnStep proves the
// augmentation is scoped to the one Step declaring FeedsForward: true —
// RFC-0004 §3's "the one immediately before," applied once — and is not
// carried forward into a later Step that declares no FeedsForward of its
// own.
func TestPipelineStrategy_FeedsForwardDoesNotThreadPastItsOwnStep(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "feeds-forward-once",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "generate-again", Kind: domain.StepKindGenerate, FeedsForward: true},
			{ID: "generate-final", Kind: domain.StepKindGenerate},
		},
	}

	exec := &captureExecutor{patches: []string{"patch-1", "patch-2", "patch-3"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "pass", Checked: []string{"build: pass"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", pipeline)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(exec.calls) != 3 {
		t.Fatalf("Executor called %d times, want 3", len(exec.calls))
	}

	for _, considered := range exec.calls[2] {
		if strings.Contains(considered, "build: pass") {
			t.Errorf("generate-final's considered = %v, want no feeds_forward carried over from an earlier step", exec.calls[2])
		}
	}
}
