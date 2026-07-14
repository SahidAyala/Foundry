package replay_test

import (
	"context"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/replay"
)

// seqVerifier returns one canned Judgment per Verify call, in order.
type seqVerifier struct {
	judgments []*domain.Judgment
	calls     int
}

func (v *seqVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	j := v.judgments[v.calls]
	v.calls++
	return j, nil
}

func actWithSteps(steps ...domain.StepRecord) *domain.Act {
	return &domain.Act{ID: "act-1", Steps: steps}
}

func TestVerify_ReproducedMatchesRecordedJudgment(t *testing.T) {
	act := actWithSteps(
		domain.StepRecord{StepID: "1", Kind: domain.StepKindGenerate, Produced: []string{"diff-1"}},
		domain.StepRecord{StepID: "2", Kind: domain.StepKindVerify, JudgmentVerdict: "pass", Checked: []string{"build: pass"}},
	)
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "pass", Checked: []string{"build: pass"}}}}

	result, err := replay.Verify(context.Background(), act, verifier, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !result.Reproduced() {
		t.Errorf("Reproduced() = false, want true: %+v", result.Steps)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("Steps = %+v, want 1 entry", result.Steps)
	}
	if result.Steps[0].ReplayedVerdict != "pass" {
		t.Errorf("ReplayedVerdict = %q, want %q", result.Steps[0].ReplayedVerdict, "pass")
	}
}

func TestVerify_DivergedVerdictIsNotReproduced(t *testing.T) {
	act := actWithSteps(
		domain.StepRecord{StepID: "1", Kind: domain.StepKindGenerate, Produced: []string{"diff-1"}},
		domain.StepRecord{StepID: "2", Kind: domain.StepKindVerify, JudgmentVerdict: "pass", Checked: []string{"build: pass"}},
	)
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "fail", Checked: []string{"build: fail"}}}}

	result, err := replay.Verify(context.Background(), act, verifier, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.Reproduced() {
		t.Fatal("Reproduced() = true, want false (verdict diverged)")
	}
	if result.Steps[0].RecordedVerdict != "pass" || result.Steps[0].ReplayedVerdict != "fail" {
		t.Errorf("Steps[0] = %+v, want RecordedVerdict pass / ReplayedVerdict fail", result.Steps[0])
	}
}

// TestVerify_DivergedCheckedSameVerdictStillReproduced documents a
// deliberate design choice: Reproduced tracks the Verdict only (the
// operative accept/reject signal), not the exact Checked findings text.
// RecordedChecked/ReplayedChecked are still both captured for a caller that
// wants to compare them itself.
func TestVerify_DivergedCheckedSameVerdictStillReproduced(t *testing.T) {
	act := actWithSteps(
		domain.StepRecord{StepID: "1", Kind: domain.StepKindGenerate, Produced: []string{"diff-1"}},
		domain.StepRecord{StepID: "2", Kind: domain.StepKindVerify, JudgmentVerdict: "pass", Checked: []string{"build: pass in 2.1s"}},
	)
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "pass", Checked: []string{"build: pass in 1.9s"}}}}

	result, err := replay.Verify(context.Background(), act, verifier, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !result.Reproduced() {
		t.Fatal("Reproduced() = false, want true (verdict matched despite differing Checked text)")
	}
	if result.Steps[0].RecordedChecked[0] == result.Steps[0].ReplayedChecked[0] {
		t.Error("RecordedChecked and ReplayedChecked unexpectedly identical — test fixture is not exercising the divergent-checked case")
	}
}

func TestVerify_NoPrecedingGenerateFails(t *testing.T) {
	act := actWithSteps(
		domain.StepRecord{StepID: "verify", Kind: domain.StepKindVerify, JudgmentVerdict: "pass"},
	)
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "pass"}}}

	_, err := replay.Verify(context.Background(), act, verifier, "workspace")
	if err == nil {
		t.Fatal("Verify with no preceding generate Step returned nil error")
	}
	if !strings.Contains(err.Error(), "act-1") || !strings.Contains(err.Error(), "verify") {
		t.Errorf("error = %q, want it to name the Act and Step", err.Error())
	}
}

func TestVerify_GenerateWithNoProducedPatchThenVerifyFails(t *testing.T) {
	act := actWithSteps(
		domain.StepRecord{StepID: "1", Kind: domain.StepKindGenerate}, // no Produced patch
		domain.StepRecord{StepID: "2", Kind: domain.StepKindVerify, JudgmentVerdict: "pass"},
	)
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "pass"}}}

	_, err := replay.Verify(context.Background(), act, verifier, "workspace")
	if err == nil {
		t.Fatal("Verify with a produced-nothing generate step returned nil error")
	}
}

func TestVerify_EmptyStepsReturnsEmptyResult(t *testing.T) {
	act := actWithSteps()

	result, err := replay.Verify(context.Background(), act, &seqVerifier{}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if len(result.Steps) != 0 {
		t.Errorf("Steps = %+v, want none", result.Steps)
	}
	if !result.Reproduced() {
		t.Error("Reproduced() = false, want true (nothing to have diverged)")
	}
}
