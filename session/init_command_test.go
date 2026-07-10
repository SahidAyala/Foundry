package session_test

import (
	"context"
	"strings"
	"testing"

	"foundry/session"
)

func TestInitCommand_ScaffoldsPipelinesDirectory(t *testing.T) {
	s, out := newTestSession(t, "")

	if err := (session.InitCommand{}).Run(context.Background(), s, ""); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Initialized") {
		t.Errorf("output = %q, want it to report initialization", out.String())
	}
}

// TestInitCommand_MakesScaffoldedPipelinesImmediatelyUsable proves /init's
// effect is visible in the same session without a restart: before Run,
// "feature" is unresolved (no starter has been scaffolded yet); after
// Run, it resolves.
func TestInitCommand_MakesScaffoldedPipelinesImmediatelyUsable(t *testing.T) {
	s, _ := newTestSession(t, "")

	if _, err := s.Engine("feature"); err == nil {
		t.Fatal("Engine(\"feature\") succeeded before /init ran — test fixture assumption is wrong")
	}

	if err := (session.InitCommand{}).Run(context.Background(), s, ""); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, name := range []string{"feature", "bugfix", "release"} {
		if _, err := s.Engine(name); err != nil {
			t.Errorf("Engine(%q) failed after /init: %v", name, err)
		}
	}
}

func TestInitCommand_IsSafeToRunTwice(t *testing.T) {
	s, _ := newTestSession(t, "")

	if err := (session.InitCommand{}).Run(context.Background(), s, ""); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if err := (session.InitCommand{}).Run(context.Background(), s, ""); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
}
