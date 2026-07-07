package cli

import (
	"bytes"
	"strings"
	"testing"

	"foundry/domain"
)

// A bytes.Buffer is never a character device, so ProgressReporter emits
// plain, uncolored text in tests — matching what a piped or captured
// `foundry do` run produces.

func TestProgressReporter_Gathering(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Gathering()
	if got := out.String(); !strings.Contains(got, "Gathering repository context") {
		t.Errorf("Gathering() = %q, missing expected text", got)
	}
}

func TestProgressReporter_ExecutingFirstAttemptVsRepair(t *testing.T) {
	var first, repair bytes.Buffer
	NewProgressReporter(&first).Executing(1)
	NewProgressReporter(&repair).Executing(2)

	if strings.Contains(first.String(), "repair") {
		t.Errorf("Executing(1) = %q, should not mention repair", first.String())
	}
	if !strings.Contains(repair.String(), "repair the failed attempt (round 2)") {
		t.Errorf("Executing(2) = %q, missing repair round", repair.String())
	}
}

func TestProgressReporter_VerifiedRendersVerdict(t *testing.T) {
	var pass, fail bytes.Buffer
	NewProgressReporter(&pass).Verified(1, &domain.Judgment{Verdict: "pass"})
	NewProgressReporter(&fail).Verified(1, &domain.Judgment{Verdict: "fail", Checked: []string{"go-build: fail\nboom"}})

	if !strings.Contains(pass.String(), "✓ pass") {
		t.Errorf("Verified(pass) = %q, want it to contain %q", pass.String(), "✓ pass")
	}
	if !strings.Contains(fail.String(), "✗ fail") {
		t.Errorf("Verified(fail) = %q, want it to contain %q", fail.String(), "✗ fail")
	}
}

// TestProgressReporter_VerifiedShowsFailureFindings is the point of
// carrying the Judgment into Verified: a demo audience should see *why*
// verification failed, not just that it did.
func TestProgressReporter_VerifiedShowsFailureFindings(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "fail",
		Checked: []string{"go-build: fail\nuser.go:5: undefined: User"},
	})

	got := out.String()
	if !strings.Contains(got, "undefined: User") {
		t.Errorf("Verified(fail) = %q, want it to show the compiler finding", got)
	}
}

// TestProgressReporter_VerifiedOmitsFindingsOnPass keeps a passing run's
// output to a single line.
func TestProgressReporter_VerifiedOmitsFindingsOnPass(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "pass",
		Checked: []string{"go-build: pass", "go-test: pass"},
	})

	if lines := strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("Verified(pass) printed %d lines, want 1:\n%s", lines, out.String())
	}
}

// TestProgressReporter_VerifiedTruncatesLongFindings keeps one verbose
// validator from flooding the live demo terminal.
func TestProgressReporter_VerifiedTruncatesLongFindings(t *testing.T) {
	var findingLines []string
	for i := 0; i < 30; i++ {
		findingLines = append(findingLines, "error line")
	}

	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "fail",
		Checked: []string{strings.Join(findingLines, "\n")},
	})

	got := out.String()
	if strings.Count(got, "error line") != maxFindingLines {
		t.Errorf("printed %d finding lines, want the capped %d", strings.Count(got, "error line"), maxFindingLines)
	}
	if !strings.Contains(got, "more lines") {
		t.Errorf("output missing truncation notice:\n%s", got)
	}
}

func TestProgressReporter_RepairingAndSkipped(t *testing.T) {
	var repairing, skipped, exceeded bytes.Buffer
	NewProgressReporter(&repairing).Repairing()
	NewProgressReporter(&skipped).RepairSkipped("budget exceeded: iteration 2 over limit 1")
	NewProgressReporter(&exceeded).BudgetExceeded("budget exceeded: iteration 1 over limit 0")

	if !strings.Contains(repairing.String(), "attempting one bounded repair") {
		t.Errorf("Repairing() = %q, missing expected text", repairing.String())
	}
	if !strings.Contains(skipped.String(), "Repair skipped: budget exceeded") {
		t.Errorf("RepairSkipped() = %q, missing reason", skipped.String())
	}
	if !strings.Contains(exceeded.String(), "Budget exceeded: budget exceeded") {
		t.Errorf("BudgetExceeded() = %q, missing reason", exceeded.String())
	}
}

func TestProgressReporter_NoColorOnNonTerminal(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Gathering()
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("output contains ANSI escapes for a non-terminal writer: %q", out.String())
	}
}
