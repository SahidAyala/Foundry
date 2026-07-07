package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_Do_RepairFlowIsRecorded is the PR-011 golden test: an Act whose
// first verification fails, is repaired once, passes, and is approved must be
// recorded with Evidence of both iterations — the considered context carries
// the findings the repair worked from, and the budget usage shows two
// Executor calls.
func TestCLI_Do_RepairFlowIsRecorded(t *testing.T) {
	t.Setenv("USER", "tester")
	repo := initGitRepo(t)

	// Stateful validator: the first Verify fails (leaving a marker and
	// findings), the second passes — deterministically forcing exactly one
	// repair, with no model involved.
	validator := `test -f .repair-marker || { touch .repair-marker; echo "1 test failed"; exit 1; }`

	var out bytes.Buffer
	c, store := newCLI(t, repo, newFilePatch("REPAIRED.md"), validator, "y\n", &out)

	if err := c.Do(context.Background(), "fix the failing test", repo); err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "REPAIRED.md")); err != nil {
		t.Errorf("repaired patch was not applied to the repo: %v", err)
	}

	acts, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("store has %d acts, want 1", len(acts))
	}
	act := acts[0]

	if act.JudgmentVerdict != "pass" {
		t.Errorf("recorded JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.Iterations != 2 {
		t.Errorf("recorded Iterations = %d, want 2 (first attempt + one repair)", act.Iterations)
	}
	if act.ApprovedBy != "tester" {
		t.Errorf("recorded ApprovedBy = %q, want %q", act.ApprovedBy, "tester")
	}

	// Evidence shows both iterations: the last considered entry is the
	// failed first attempt's findings that the repair worked from.
	if len(act.ConsideredFiles) == 0 {
		t.Fatal("recorded Act has no considered Evidence")
	}
	last := act.ConsideredFiles[len(act.ConsideredFiles)-1]
	if !strings.Contains(last, "failed previous attempt") || !strings.Contains(last, "1 test failed") {
		t.Errorf("recorded Evidence missing the repair findings, got %q", last)
	}

	// The recorded checked Evidence reflects the repair's passing round,
	// not the failed first attempt — a later `foundry show` sees why the
	// final verdict is pass.
	if len(act.CheckedFindings) != 1 || act.CheckedFindings[0] != "check: pass" {
		t.Errorf("recorded CheckedFindings = %v, want [\"check: pass\"]", act.CheckedFindings)
	}
}
