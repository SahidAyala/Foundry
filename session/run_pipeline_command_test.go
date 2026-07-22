package session_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/session"
)

func newTestSession(t *testing.T, in string) (*session.Session, *bytes.Buffer) {
	t.Helper()
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	s, err := session.NewSession(context.Background(), root, strings.NewReader(in), out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	return s, out
}

func TestRunPipelineCommand_EmptyArgsFails(t *testing.T) {
	s, _ := newTestSession(t, "y\n")

	err := session.RunPipelineCommand{PipelineName: "default"}.Run(context.Background(), s, "   ")
	if err == nil {
		t.Fatal("Run with empty args returned nil error")
	}
}

func TestRunPipelineCommand_UnknownPipelineFails(t *testing.T) {
	s, _ := newTestSession(t, "y\n")

	err := session.RunPipelineCommand{PipelineName: "does-not-exist"}.Run(context.Background(), s, "do something")
	if err == nil {
		t.Fatal("Run with an unresolved Pipeline name returned nil error")
	}
}

func TestRunPipelineCommand_RunsAndRecordsOnApproval(t *testing.T) {
	s, out := newTestSession(t, "y\n")

	err := session.RunPipelineCommand{PipelineName: "default"}.Run(context.Background(), s, "add a feature")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Applied and recorded") {
		t.Errorf("output = %q, want it to report the Act was applied and recorded", out.String())
	}

	acts, err := s.Recorder().List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("recorded Acts = %d, want 1", len(acts))
	}
	if acts[0].JudgmentVerdict != "pass" {
		t.Errorf("recorded Act JudgmentVerdict = %q, want %q", acts[0].JudgmentVerdict, "pass")
	}
}

func TestRunPipelineCommand_DeclinedApprovalDoesNotRecord(t *testing.T) {
	s, out := newTestSession(t, "n\n")

	err := session.RunPipelineCommand{PipelineName: "default"}.Run(context.Background(), s, "add a feature")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Declined") {
		t.Errorf("output = %q, want it to report the decline", out.String())
	}

	acts, err := s.Recorder().List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("recorded Acts = %d, want 0 after a declined approval", len(acts))
	}
}

// TestRunPipelineCommand_BacksMultipleSlashCommandsViaDifferentPipelineNames
// proves the one handler type instantiated twice, with only PipelineName
// differing, correctly backs two different slash commands ("/default"-
// shaped and "/review"-shaped) against the same Session — no per-command
// branching anywhere in RunPipelineCommand itself.
func TestRunPipelineCommand_BacksMultipleSlashCommandsViaDifferentPipelineNames(t *testing.T) {
	// Two distinct patches: cli.CLI.Do actually lands an approved patch on
	// the repo's real branch (unlike a bare engine.Engine.Run, which only
	// ever touches a throwaway staged worktree), so applying the same
	// create-this-file patch twice in a row against the same evolving
	// branch would fail on the second call for an unrelated reason (the
	// file already exists) — a test-fixture concern, not anything this
	// assertion is about.
	root := initGitRepo(t)
	s, err := session.NewSession(context.Background(), root, strings.NewReader("y\ny\n"), &bytes.Buffer{},
		newSequencedExecutorFactory(scriptedPatch, secondScriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	if err := (session.RunPipelineCommand{PipelineName: "default"}).Run(context.Background(), s, "add a feature"); err != nil {
		t.Fatalf("default Run failed: %v", err)
	}
	if err := (session.RunPipelineCommand{PipelineName: "review"}).Run(context.Background(), s, "review a change"); err != nil {
		t.Fatalf("review Run failed: %v", err)
	}

	acts, err := s.Recorder().List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("recorded Acts = %d, want 2", len(acts))
	}
}

// nonApplyingPatch references content that does not match initGitRepo's
// real README.md ("hello\n") — git apply rejects it deterministically,
// producing a legitimate "fail" Judgment (workspace.StagedVerifier's own
// applyIn error path), not a Go error.
const nonApplyingPatch = `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-this-line-does-not-exist-in-the-real-file
+replacement
`

// failThenErrorExecutor fails to apply on its first call (attempt 1 — a
// legitimate "fail" verdict, not a Go error) then returns a genuine Go
// error on its second call (attempt 2, the repair round default.json's
// "repair": {"max_attempts": 1} allows) — simulating a crash mid-repair,
// deterministically, after attempt 1's generate and verify Steps have
// already had their checkpoints durably saved with a valid context
// throughout. No timing-dependent simulation needed.
type failThenErrorExecutor struct {
	calls int
}

func (e *failThenErrorExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	e.calls++
	if e.calls == 1 {
		return &domain.Outcome{Patch: nonApplyingPatch}, nil
	}
	return nil, errors.New("simulated executor crash mid-repair")
}

// TestRunPipelineCommand_SavesCheckpointOnInterruption covers a real gap:
// session.NewSession never wired engine.Engine.SetCheckpointSaver, unlike
// cmd/foundry/commands/do.go's wireEngine (which always did) — so no
// interactive-session Act ever left a checkpoint behind, regardless of
// how far into a Pipeline it got. Since ADR-0009 made the interactive
// session Foundry's primary interface, this made `foundry resume`
// effectively dead for the common case. A genuine mid-repair Executor
// error (not merely a failing verdict, which reaches a terminal
// disposition and correctly deletes its checkpoint) must leave a
// checkpoint on disk that Checkpoints().List can find.
func TestRunPipelineCommand_SavesCheckpointOnInterruption(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	exec := &failThenErrorExecutor{}

	s, err := session.NewSession(context.Background(), root, strings.NewReader(""), out,
		func(string) engine.Executor { return exec })
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	err = session.RunPipelineCommand{PipelineName: "default"}.Run(context.Background(), s, "add a feature")
	if err == nil {
		t.Fatal("Run with a mid-repair Executor error returned nil error, want the simulated crash to propagate")
	}

	acts, err := s.Checkpoints().List(context.Background())
	if err != nil {
		t.Fatalf("Checkpoints().List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("checkpointed Acts = %d, want 1 (the interrupted Act's checkpoint must survive on disk)", len(acts))
	}
}
