package claude

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
)

func newReviewer(r runner) *Reviewer {
	return &Reviewer{
		executable: "claude",
		runner:     r,
	}
}

func TestReviewerVerify_PassNoFindings(t *testing.T) {
	r := &fakeRunner{stdout: "VERDICT: pass\nFINDINGS:\n- none"}
	rv := newReviewer(r)

	judgment, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff --git a/x b/x"}, "/staged")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	if len(judgment.Checked) != 1 || judgment.Checked[0] != "claude-review: pass" {
		t.Errorf("Checked = %v, want a single clean-pass entry", judgment.Checked)
	}
	if r.gotDir != "/staged" {
		t.Errorf("runner dir = %q, want the staged workspace argument %q", r.gotDir, "/staged")
	}
}

func TestReviewerVerify_PassWithFindingsStillPasses(t *testing.T) {
	r := &fakeRunner{stdout: "VERDICT: pass\nFINDINGS:\n- consider renaming x"}
	rv := newReviewer(r)

	judgment, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	if len(judgment.Checked) != 2 || judgment.Checked[1] != "consider renaming x" {
		t.Errorf("Checked = %v, want pass entry plus the finding", judgment.Checked)
	}
}

func TestReviewerVerify_Fail(t *testing.T) {
	r := &fakeRunner{stdout: "VERDICT: fail\nFINDINGS:\n- missing error handling"}
	rv := newReviewer(r)

	judgment, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "fail")
	}
}

func TestReviewerVerify_MalformedResponseIsFail(t *testing.T) {
	r := &fakeRunner{stdout: "I looked at the diff and it seems fine."}
	rv := newReviewer(r)

	judgment, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q for an unparseable response", judgment.Verdict, "fail")
	}
}

func TestReviewerVerify_CaseInsensitiveVerdictLabel(t *testing.T) {
	r := &fakeRunner{stdout: "verdict: PASS\nfindings:\n- none"}
	rv := newReviewer(r)

	judgment, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
}

func TestReviewerVerify_NoWorkspaceFails(t *testing.T) {
	rv := newReviewer(&fakeRunner{})
	_, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "")
	if err == nil {
		t.Fatal("Verify with no workspace argument returned nil error")
	}
}

func TestReviewerVerify_ExecutableMissingFails(t *testing.T) {
	rv := newReviewer(&fakeRunner{err: exec.ErrNotFound})
	_, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged")
	if err == nil {
		t.Fatal("Verify with a missing executable returned nil error")
	}
}

func TestReviewerVerify_PassesModelFlagWhenSet(t *testing.T) {
	r := &fakeRunner{stdout: "VERDICT: pass\nFINDINGS:\n- none"}
	rv := newReviewer(r)
	rv.model = "opus"

	if _, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged"); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	wantArgs := []string{"-p", "--model", "opus"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
}

func TestReviewerVerify_OmitsModelFlagWhenUnset(t *testing.T) {
	r := &fakeRunner{stdout: "VERDICT: pass\nFINDINGS:\n- none"}
	rv := newReviewer(r)

	if _, err := rv.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "/staged"); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	wantArgs := []string{"-p"}
	if len(r.gotArgs) != len(wantArgs) || r.gotArgs[0] != wantArgs[0] {
		t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
}

func TestReviewPrompt_IncludesIntentWhenSet(t *testing.T) {
	prompt := reviewPrompt("Fix the greeting", "diff --git a/x b/x")
	if !strings.Contains(prompt, "Fix the greeting") {
		t.Errorf("prompt missing intent, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "diff --git a/x b/x") {
		t.Errorf("prompt missing patch, got:\n%s", prompt)
	}
}

func TestReviewPrompt_OmitsIntentSectionWhenEmpty(t *testing.T) {
	prompt := reviewPrompt("", "diff --git a/x b/x")
	if strings.Contains(prompt, "original request") {
		t.Errorf("prompt should not reference an original request when intent is empty, got:\n%s", prompt)
	}
}

func TestParseReview_FiltersNoneFinding(t *testing.T) {
	judgment := parseReview("VERDICT: pass\nFINDINGS:\n- none")
	if len(judgment.Checked) != 1 {
		t.Errorf("Checked = %v, want the \"none\" finding filtered out", judgment.Checked)
	}
}

func TestParseReview_EmptyContentIsFail(t *testing.T) {
	judgment := parseReview("")
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q for empty content", judgment.Verdict, "fail")
	}
}
