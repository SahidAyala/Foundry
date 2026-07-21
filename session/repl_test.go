package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/project"
	"foundry/session"
)

// TestREPL_EndToEnd_InitFeatureExit is the first true end-to-end test of
// the interactive session's whole vertical slice: /init scaffolds a
// project, /feature runs a Pipeline through to approval, and /exit ends
// the session cleanly.
func TestREPL_EndToEnd_InitFeatureExit(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("/init\n/feature add x\ny\n/exit\n")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(out.String(), "Initialized") {
		t.Errorf("output missing /init's confirmation: %q", out.String())
	}
	if !strings.Contains(out.String(), "Applied and recorded") {
		t.Errorf("output missing /feature's applied-and-recorded confirmation: %q", out.String())
	}

	if _, err := os.Stat(filepath.Join(root, project.PipelinesDir, "feature.json")); err != nil {
		t.Errorf("expected /init to have scaffolded feature.json: %v", err)
	}

	acts, err := s.Recorder().List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 1 {
		t.Errorf("recorded Acts = %d, want 1", len(acts))
	}
}

func TestREPL_UnknownCommandDoesNotStopSession(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("/bogus\n/exit\n")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "bogus") {
		t.Errorf("output = %q, want it to report the unknown command", out.String())
	}
}

func TestREPL_BlankLinesAreIgnored(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("\n\n/exit\n")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestREPL_EndOfInputEndsSessionCleanly(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("/init")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Initialized") {
		t.Errorf("output = %q, want /init to have run even with no trailing newline before EOF", out.String())
	}
}

// TestREPL_HelpListsRegisteredCommands verifies /help — the command the
// startup banner has always promised (ADR-0009 Decision 6) — actually
// lists every command DefaultCommandRegistry registers, itself included.
func TestREPL_HelpListsRegisteredCommands(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("/help\n/exit\n")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	got := out.String()
	for _, name := range []string{"/init", "/feature", "/bug", "/review", "/release", "/help"} {
		if !strings.Contains(got, name) {
			t.Errorf("output = %q, want it to list %s", got, name)
		}
	}
}

func TestREPL_PlainTextIsReportedNotSilentlyDropped(t *testing.T) {
	root := initGitRepo(t)
	out := &bytes.Buffer{}
	in := strings.NewReader("implementa refresh tokens\n/exit\n")

	s, err := session.NewSession(context.Background(), root, in, out, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "not yet supported") {
		t.Errorf("output = %q, want it to report plain text is not yet supported", out.String())
	}
}
