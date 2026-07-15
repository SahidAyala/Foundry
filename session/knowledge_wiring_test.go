package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/session"
)

// TestSession_KnowledgeNoteReachesLaterActsConsideredContext proves
// RFC-0005's whole point end to end: a note previously written under
// .foundry/knowledge/ (RFC-0004 §2.6's knowledge-note apply target,
// simulated here by writing the file directly, since Piece 6 of that
// mechanism is already covered elsewhere) is retrieved by
// knowledge.Gatherer and reaches a later Act's considered Context through
// session.NewSession's gatherer.Compose wiring (session.go) — the
// "project is better positioned for the next Act" promise (knowledge.md)
// made real for the first time.
func TestSession_KnowledgeNoteReachesLaterActsConsideredContext(t *testing.T) {
	root := initGitRepo(t)

	knowledgeDir := filepath.Join(root, ".foundry", "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	noteContent := "# Add CSV export\n\nWe decided to stream export rows instead of buffering the whole file.\n"
	if err := os.WriteFile(filepath.Join(knowledgeDir, "act-0-csv-export.md"), []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	s, err := session.NewSession(context.Background(), root, strings.NewReader(""), &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	eng, err := s.Engine("default")
	if err != nil {
		t.Fatalf(`Engine("default") failed: %v`, err)
	}

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "add csv export streaming to the reports page"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	found := false
	for _, entry := range act.ConsideredFiles {
		if strings.Contains(entry, "stream export rows instead of buffering") {
			found = true
			if !strings.HasPrefix(entry, ".foundry/knowledge/act-0-csv-export.md:\n") {
				t.Errorf("entry = %q, want it prefixed with the note's own path (provenance)", entry)
			}
		}
	}
	if !found {
		t.Errorf("ConsideredFiles = %v, want it to include the matching Knowledge note's content", act.ConsideredFiles)
	}
}

// TestSession_NoKnowledgeDirectoryBehavesExactlyAsBefore is the
// backward-compatibility guarantee gatherer.Compose rests on: a project
// with no .foundry/knowledge/ directory at all sees identical
// ConsideredFiles to what NaiveGatherer alone would have produced.
func TestSession_NoKnowledgeDirectoryBehavesExactlyAsBefore(t *testing.T) {
	root := initGitRepo(t)

	s, err := session.NewSession(context.Background(), root, strings.NewReader(""), &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	eng, err := s.Engine("default")
	if err != nil {
		t.Fatalf(`Engine("default") failed: %v`, err)
	}

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "update README.md"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	for _, entry := range act.ConsideredFiles {
		if strings.Contains(entry, ".foundry/knowledge/") {
			t.Errorf("ConsideredFiles = %v, want no Knowledge entries when the directory does not exist", act.ConsideredFiles)
		}
	}
}
