package cli

import (
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
Verdict:    pass
Approved:   by tester at 2026-01-02T03:04:05Z
Iterations: 2 (estimated cost $1.00)

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

	if got := formatAct(act); got != want {
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
Verdict:    fail
Approved:   no
Iterations: 0 (estimated cost $0.00)

Considered evidence:
  (none)

Checked evidence:
  (none)

Patch:
  (none)
`

	if got := formatAct(act); got != want {
		t.Errorf("formatAct golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
