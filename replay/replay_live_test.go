package replay_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor/claude"
	"foundry/gatherer"
	"foundry/record"
	"foundry/replay"
	"foundry/verify"
	"foundry/workspace"
)

// TestVerify_ReproducesARealClaudeCodeProducedAct closes a real gap: every
// other replay test (this file's siblings) replays a hand-built domain.Act
// fixture whose Produced patch is a canned string — replay.Verify has never
// been exercised against a patch an actual model actually wrote. A real
// diff is exactly the kind of messy, fuzzy-context output (unusual
// whitespace, hunk framing) that a fixture can never represent, and
// StagedVerifier's git apply is the one place that difference could bite:
// a patch that applied once during the original Produce, staged via a
// fresh worktree, might not apply cleanly a second time during replay.
//
// This test is skipped by default — it shells out to a real `claude` CLI,
// costs real money/time, and needs a real, authenticated Claude Code
// install — and only runs when FOUNDRY_LIVE_TEST=1 is set explicitly.
func TestVerify_ReproducesARealClaudeCodeProducedAct(t *testing.T) {
	if os.Getenv("FOUNDRY_LIVE_TEST") != "1" {
		t.Skip("set FOUNDRY_LIVE_TEST=1 to run this test against a real Claude Code CLI")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skipf("claude CLI not on PATH: %v", err)
	}

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "foundry-live-test@example.com")
	runGit(t, root, "config", "user.name", "Foundry Live Test")
	notes := filepath.Join(root, "NOTES.md")
	if err := os.WriteFile(notes, []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("write NOTES.md: %v", err)
	}
	runGit(t, root, "add", "NOTES.md")
	runGit(t, root, "commit", "-m", "initial commit")

	ctx := context.Background()

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(root)...)
	if err != nil {
		t.Fatalf("NewGate failed: %v", err)
	}
	stagedVerifier := workspace.NewStagedVerifier(gate)

	eng := engine.NewEngine(gatherer.NewNaiveGatherer(root), claude.NewClaudeExecutor(root), stagedVerifier, root, engine.DefaultPipeline())

	intent := &domain.Intent{Text: "Append a single line to NOTES.md that says exactly: hello from foundry"}
	act, err := eng.Run(ctx, intent)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Fatalf("original run's JudgmentVerdict = %q, want %q — findings: %v", act.JudgmentVerdict, "pass", act.CheckedFindings)
	}

	actsDir := t.TempDir()
	store, err := record.NewFileStore(actsDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	if err := store.Write(ctx, act); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	recorded, err := store.Read(ctx, act.ID)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	result, err := replay.Verify(ctx, recorded, stagedVerifier, root)
	if err != nil {
		t.Fatalf("replay.Verify failed: %v", err)
	}
	if !result.Reproduced() {
		t.Errorf("Reproduced() = false against a real Claude-Code-produced patch, want true: %+v", result.Steps)
	}
	for _, s := range result.Steps {
		if s.ReplayedVerdict != s.RecordedVerdict {
			t.Errorf("Step %s: ReplayedVerdict = %q, RecordedVerdict = %q", s.StepID, s.ReplayedVerdict, s.RecordedVerdict)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
