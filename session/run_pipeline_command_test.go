package session_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

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
