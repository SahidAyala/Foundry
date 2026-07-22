package cli

import (
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

func TestFormatAct_Golden(t *testing.T) {
	approvedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	act := &domain.Act{
		ID:        "a1b2c3d4e5f6a7b8",
		Intent:    "add logging to main.go",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ConsideredFiles: []string{
			"main.go:\npackage main\n",
			"verification findings from the failed previous attempt:\ntests: fail",
		},
		CheckedFindings: []string{"go-build: pass", "go-test: pass"},
		Patch:           "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n package main\n+// added\n",
		JudgmentVerdict: "pass",
		ApprovedBy:      "tester",
		ApprovedAt:      &approvedAt,
		Iterations:      2,
		CostEstimateUSD: 1.00,
	}

	want := `Act:        a1b2c3d4e5f6a7b8
Created:    2026-01-01T00:00:00Z
Intent:     add logging to main.go
Verdict:    ✓ pass
Approved:   by tester at 2026-01-02T03:04:05Z
Iterations: 2 (estimated cost $1.00)

Steps:
  (none)

Considered evidence:
  - main.go:
  - verification findings from the failed previous attempt:

Checked evidence:
  go-build: pass
  go-test: pass

Patch:
diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

	if got := formatAct(act, false); got != want {
		t.Errorf("formatAct golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatAct_UnapprovedWithoutEvidenceOrPatch(t *testing.T) {
	act := &domain.Act{
		ID:              "ffff000011112222",
		Intent:          "do nothing",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		JudgmentVerdict: "fail",
	}

	want := `Act:        ffff000011112222
Created:    2026-01-01T00:00:00Z
Intent:     do nothing
Verdict:    ✗ fail
Approved:   no
Iterations: 0 (estimated cost $0.00)

Steps:
  (none)

Considered evidence:
  (none)

Checked evidence:
  (none)

Patch:
  (none)
`

	if got := formatAct(act, false); got != want {
		t.Errorf("formatAct golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatAct_ColorTintsVerdictAndPatch(t *testing.T) {
	act := &domain.Act{
		ID:              "a1b2c3d4e5f6a7b8",
		Intent:          "add logging to main.go",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Patch:           "+// added\n",
		JudgmentVerdict: "pass",
	}

	got := formatAct(act, true)
	wantVerdict := "Verdict:    " + renderVerdict("pass", true) + "\n"
	if !strings.Contains(got, wantVerdict) {
		t.Errorf("formatAct(color=true) missing tinted verdict line %q in:\n%s", wantVerdict, got)
	}
	wantPatchLine := renderDiff("+// added", true)
	if !strings.Contains(got, wantPatchLine) {
		t.Errorf("formatAct(color=true) missing tinted patch line %q in:\n%s", wantPatchLine, got)
	}
}

// TestFormatAct_StepsTrace covers the built-in "review" Pipeline's shape —
// generate, then two independent verify Steps with different verdicts — the
// case the flat JudgmentVerdict/CheckedFindings fields alone would collapse
// into only the last Step's outcome, silently dropping the first verify
// Step's own findings from `foundry show`'s output even though the Record
// already carries them (domain.Act.Steps).
func TestFormatAct_StepsTrace(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	act := &domain.Act{
		ID:              "a1b2c3d4e5f6a7b8",
		Intent:          "review the auth module",
		CreatedAt:       t0,
		JudgmentVerdict: "fail",
		Steps: []domain.StepRecord{
			{
				StepID:     "generate",
				Kind:       domain.StepKindGenerate,
				StartedAt:  t0,
				FinishedAt: t0.Add(800 * time.Millisecond),
			},
			{
				StepID:          "verify",
				Kind:            domain.StepKindVerify,
				JudgmentVerdict: "pass",
				StartedAt:       t0.Add(800 * time.Millisecond),
				FinishedAt:      t0.Add(1100 * time.Millisecond),
			},
			{
				StepID:          "verify-again",
				Kind:            domain.StepKindVerify,
				JudgmentVerdict: "fail",
				Checked:         []string{"security-review: found a hardcoded secret"},
				StartedAt:       t0.Add(1100 * time.Millisecond),
				FinishedAt:      t0.Add(1300 * time.Millisecond),
			},
		},
	}

	want := `Act:        a1b2c3d4e5f6a7b8
Created:    2026-01-01T00:00:00Z
Intent:     review the auth module
Verdict:    ✗ fail
Approved:   no
Iterations: 0 (estimated cost $0.00)

Steps:
  generate       (generate)  800ms
  verify         (verify)  ✓ pass  300ms
  verify-again   (verify)  ✗ fail  200ms

Considered evidence:
  (none)

Checked evidence:
  (none)

Patch:
  (none)
`

	if got := formatAct(act, false); got != want {
		t.Errorf("formatAct Steps trace golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestFormatAct_ActualCostOmittedWhenNil covers every Act produced by an
// Executor with no billing signal (executor/claude.ClaudeExecutor,
// executor.ScriptedExecutor) — the overwhelming majority today. The
// "Actual cost" line must not appear at all, not as a permanently-empty
// placeholder.
func TestFormatAct_ActualCostOmittedWhenNil(t *testing.T) {
	act := &domain.Act{
		ID:              "ffff000011112222",
		Intent:          "do nothing",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		JudgmentVerdict: "fail",
	}

	if got := formatAct(act, false); strings.Contains(got, "Actual cost") {
		t.Errorf("formatAct() with nil ActualCostUSD contains an Actual cost line:\n%s", got)
	}
}

// TestFormatAct_ActualCostShownWithCoverage covers ADR-0011's actual-cost
// reporting: one of two generate Steps reported a real cost, the other
// (e.g. a repair round handled by executor/claude.ClaudeExecutor) did not
// — the total must render alongside how many Steps it actually covers,
// never implying it is a complete accounting.
func TestFormatAct_ActualCostShownWithCoverage(t *testing.T) {
	cost := 0.0042
	total := 0.0042
	act := &domain.Act{
		ID:              "a1b2c3d4e5f6a7b8",
		Intent:          "implement X",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		JudgmentVerdict: "pass",
		ActualCostUSD:   &total,
		Steps: []domain.StepRecord{
			{StepID: "1", Kind: domain.StepKindGenerate, ActualCostUSD: &cost},
			{StepID: "2", Kind: domain.StepKindGenerate},
		},
	}

	got := formatAct(act, false)
	want := "Actual cost: $0.0042 (reported for 1 of 2 generate Steps — ADR-0011)\n"
	if !strings.Contains(got, want) {
		t.Errorf("formatAct() missing %q in:\n%s", want, got)
	}
}
