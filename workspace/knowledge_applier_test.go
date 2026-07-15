package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/workspace"
)

func TestKnowledgeNoteApplier_WritesNoteUnderKnowledgeNoteDir(t *testing.T) {
	root := t.TempDir()
	act := &domain.Act{ID: "abc123", Intent: "Add CSV export to the reports page", Patch: "the plan: add an exporter"}

	if err := (workspace.KnowledgeNoteApplier{}).Apply(context.Background(), root, act); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, workspace.KnowledgeNoteDir))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("KnowledgeNoteDir has %d entries, want 1", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "abc123-") || !strings.HasSuffix(name, ".md") {
		t.Errorf("note filename = %q, want prefix %q and suffix %q", name, "abc123-", ".md")
	}

	content, err := os.ReadFile(filepath.Join(root, workspace.KnowledgeNoteDir, name))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(content), act.Patch) {
		t.Errorf("note content = %q, want it to contain %q", content, act.Patch)
	}
}

func TestKnowledgeNoteApplier_TwoActsGetTwoNotes(t *testing.T) {
	root := t.TempDir()
	applier := workspace.KnowledgeNoteApplier{}

	first := &domain.Act{ID: "act-1", Intent: "add CSV export", Patch: "note one"}
	second := &domain.Act{ID: "act-2", Intent: "add CSV export", Patch: "note two"}
	if err := applier.Apply(context.Background(), root, first); err != nil {
		t.Fatalf("Apply(first) failed: %v", err)
	}
	if err := applier.Apply(context.Background(), root, second); err != nil {
		t.Fatalf("Apply(second) failed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, workspace.KnowledgeNoteDir))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("KnowledgeNoteDir has %d entries, want 2 (one per Act)", len(entries))
	}
}

func TestProjectDocApplier_AppendsAcrossMultipleActs(t *testing.T) {
	root := t.TempDir()
	applier := workspace.ProjectDocApplier{DocsPath: "docs/decisions.md"}

	first := &domain.Act{ID: "act-1", Intent: "decide on retries", Patch: "first decision"}
	second := &domain.Act{ID: "act-2", Intent: "decide on timeouts", Patch: "second decision"}
	if err := applier.Apply(context.Background(), root, first); err != nil {
		t.Fatalf("Apply(first) failed: %v", err)
	}
	if err := applier.Apply(context.Background(), root, second); err != nil {
		t.Fatalf("Apply(second) failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "docs/decisions.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(content), "first decision") || !strings.Contains(string(content), "second decision") {
		t.Errorf("docs/decisions.md = %q, want it to contain both Acts' content", content)
	}
}

func TestProjectDocApplier_EmptyDocsPathFails(t *testing.T) {
	root := t.TempDir()
	applier := workspace.ProjectDocApplier{}

	err := applier.Apply(context.Background(), root, &domain.Act{ID: "act-1", Intent: "x", Patch: "y"})
	if err == nil {
		t.Fatal("Apply with an empty DocsPath returned nil error")
	}
}
