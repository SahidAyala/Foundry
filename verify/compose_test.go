package verify_test

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/verify"
)

// fakeVerifier returns a canned Judgment or error from Verify, recording
// whether it was called — mirrors gatherer_test's own fakeGatherer.
type fakeVerifier struct {
	judgment *domain.Judgment
	err      error
	called   bool
}

func (v *fakeVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	v.called = true
	if v.err != nil {
		return nil, v.err
	}
	return v.judgment, nil
}

func TestCompose_NoVerifiersPasses(t *testing.T) {
	judgment, err := verify.Compose().Verify(context.Background(), &domain.Outcome{}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q for no verifiers at all", judgment.Verdict, "pass")
	}
}

func TestCompose_OneVerifierBehavesIdentically(t *testing.T) {
	v := &fakeVerifier{judgment: &domain.Judgment{Verdict: "pass", Checked: []string{"go-build: pass"}}}

	judgment, err := verify.Compose(v).Verify(context.Background(), &domain.Outcome{}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" || len(judgment.Checked) != 1 || judgment.Checked[0] != "go-build: pass" {
		t.Errorf("Verify = %+v, want the single verifier's own Judgment unchanged", judgment)
	}
}

func TestCompose_AllPassYieldsPass(t *testing.T) {
	first := &fakeVerifier{judgment: &domain.Judgment{Verdict: "pass", Checked: []string{"go-build: pass"}}}
	second := &fakeVerifier{judgment: &domain.Judgment{Verdict: "pass", Checked: []string{"ai-review: pass"}}}

	judgment, err := verify.Compose(first, second).Verify(context.Background(), &domain.Outcome{}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	want := []string{"go-build: pass", "ai-review: pass"}
	if len(judgment.Checked) != len(want) || judgment.Checked[0] != want[0] || judgment.Checked[1] != want[1] {
		t.Errorf("Checked = %v, want %v (concatenated in order)", judgment.Checked, want)
	}
	if !first.called || !second.called {
		t.Error("both verifiers must be called")
	}
}

func TestCompose_AnyFailureFailsTheWhole(t *testing.T) {
	first := &fakeVerifier{judgment: &domain.Judgment{Verdict: "pass", Checked: []string{"go-build: pass"}}}
	second := &fakeVerifier{judgment: &domain.Judgment{Verdict: "fail", Checked: []string{"ai-review: fail\nunhandled nil pointer"}}}

	judgment, err := verify.Compose(first, second).Verify(context.Background(), &domain.Outcome{}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q when any composed verifier fails", judgment.Verdict, "fail")
	}
	if len(judgment.Checked) != 2 {
		t.Errorf("Checked = %v, want both verifiers' findings present even though the first passed", judgment.Checked)
	}
}

func TestCompose_ErrorFromEarlierVerifierStopsComposition(t *testing.T) {
	wantErr := errors.New("first verifier failed")
	first := &fakeVerifier{err: wantErr}
	second := &fakeVerifier{judgment: &domain.Judgment{Verdict: "pass"}}

	_, err := verify.Compose(first, second).Verify(context.Background(), &domain.Outcome{}, "workspace")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if second.called {
		t.Error("second verifier must not be called once an earlier one errors")
	}
}

var _ engine.Verifier = &fakeVerifier{}
