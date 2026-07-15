package knowledge_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/knowledge"
)

func writeNote(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".foundry", "knowledge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
}

func TestGather_MissingDirectoryReturnsEmptyNotError(t *testing.T) {
	root := t.TempDir()

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "add CSV export"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Gather = %v, want empty for a missing directory", got)
	}
}

func TestGather_EmptyDirectoryReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry", "knowledge"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "add CSV export"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Gather = %v, want empty for an empty directory", got)
	}
}

func TestGather_MatchingNoteIsReturnedWithProvenance(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-csv-export.md", "# Add CSV export\n\nWe decided to stream export rows instead of buffering.\n")

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "add csv export to the reports page"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Gather = %v, want exactly 1 matching note", got)
	}
	if !strings.HasPrefix(got[0], ".foundry/knowledge/act-1-csv-export.md:\n") {
		t.Errorf("entry = %q, want it prefixed with the note's path", got[0])
	}
	if !strings.Contains(got[0], "stream export rows") {
		t.Errorf("entry = %q, want it to contain the note's content", got[0])
	}
}

func TestGather_NonMatchingNoteIsExcluded(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-unrelated.md", "# Unrelated topic\n\nSomething about deployment timing windows.\n")

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "add csv export to the reports page"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Gather = %v, want no matches for an unrelated note", got)
	}
}

func TestGather_RanksMoreOverlappingNoteFirst(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-weak.md", "csv export mentioned once here.\n")
	writeNote(t, root, "act-2-strong.md", "csv export reports page pagination streaming details.\n")

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "csv export reports page pagination streaming"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Gather = %v, want both notes matched", got)
	}
	if !strings.Contains(got[0], "act-2-strong.md") {
		t.Errorf("entries = %v, want the higher-overlap note (act-2-strong.md) first", got)
	}
}

func TestGather_CapsAtMaxNotes(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		writeNote(t, root, "act-"+string(rune('a'+i))+"-export.md", "csv export streaming pagination details.\n")
	}

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "csv export streaming pagination"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Gather returned %d entries, want capped at 3", len(got))
	}
}

func TestGather_NoSignificantWordsInIntentMatchesNothing(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-note.md", "csv export streaming details.\n")

	got, err := knowledge.NewGatherer(root).Gather(context.Background(), &domain.Intent{Text: "fix it"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Gather = %v, want empty when the Intent has no significant words", got)
	}
}

func TestGather_CancelledContextFails(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-note.md", "csv export streaming details.\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := knowledge.NewGatherer(root).Gather(ctx, &domain.Intent{Text: "csv export streaming"})
	if err == nil {
		t.Fatal("Gather with a cancelled context returned nil error")
	}
}

func TestGather_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeNote(t, root, "act-1-a.md", "csv export streaming details alpha.\n")
	writeNote(t, root, "act-2-b.md", "csv export streaming details beta.\n")

	g := knowledge.NewGatherer(root)
	intent := &domain.Intent{Text: "csv export streaming details"}

	first, err := g.Gather(context.Background(), intent)
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	second, err := g.Gather(context.Background(), intent)
	if err != nil {
		t.Fatalf("second Gather failed: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("Gather is not deterministic: %v vs %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("Gather is not deterministic at entry %d: %q vs %q", i, first[i], second[i])
		}
	}
}
