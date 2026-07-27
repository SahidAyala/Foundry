package backlog_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SahidAyala/Foundry/ticket/backlog"
)

func writeBacklog(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "backlog.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write backlog.json: %v", err)
	}
	return path
}

const sampleBacklog = `{
	"features": [
		{"id": "1", "title": "First", "description": "Already shipped.", "status": "done"},
		{"id": "2", "title": "Second", "description": "Being worked on.", "status": "in_progress"},
		{"id": "3", "title": "Third", "description": "Next up.", "acceptance": ["Does X", "Does Y"], "status": "pending"},
		{"id": "4", "title": "Fourth", "description": "Also pending.", "status": "pending"}
	]
}`

func TestFetch_ByExplicitID_ReturnsFeature(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, sampleBacklog)
	f := backlog.NewFetcher(dir, "backlog.json")

	issue, err := f.Fetch(context.Background(), "1")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "1" {
		t.Errorf("ID = %q, want %q", issue.ID, "1")
	}
	if issue.Title != "First" {
		t.Errorf("Title = %q, want %q", issue.Title, "First")
	}
	if issue.Description != "Already shipped." {
		t.Errorf("Description = %q, want %q", issue.Description, "Already shipped.")
	}
}

func TestFetch_ByExplicitID_IncludesAcceptanceCriteriaInDescription(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, sampleBacklog)
	f := backlog.NewFetcher(dir, "backlog.json")

	issue, err := f.Fetch(context.Background(), "3")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if !strings.Contains(issue.Description, "Next up.") {
		t.Errorf("Description = %q, want it to still contain the base description", issue.Description)
	}
	if !strings.Contains(issue.Description, "- Does X") || !strings.Contains(issue.Description, "- Does Y") {
		t.Errorf("Description = %q, want it to contain both acceptance criteria as bullets", issue.Description)
	}
}

func TestFetch_Next_ReturnsFirstPendingInFileOrder(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, sampleBacklog)
	f := backlog.NewFetcher(dir, "backlog.json")

	issue, err := f.Fetch(context.Background(), backlog.NextID)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "3" {
		t.Errorf("ID = %q, want %q (the first pending feature, not the in_progress or done ones before it)", issue.ID, "3")
	}
}

func TestFetch_Next_NoPendingFeaturesFails(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, `{"features": [{"id": "1", "title": "First", "status": "done"}]}`)
	f := backlog.NewFetcher(dir, "backlog.json")

	_, err := f.Fetch(context.Background(), backlog.NextID)
	if err == nil {
		t.Fatal("Fetch(next) with no pending features returned nil error")
	}
	if !strings.Contains(err.Error(), "pending") {
		t.Errorf("error = %q, want it to mention no pending feature was found", err.Error())
	}
}

func TestFetch_UnknownIDFails(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, sampleBacklog)
	f := backlog.NewFetcher(dir, "backlog.json")

	_, err := f.Fetch(context.Background(), "999")
	if err == nil {
		t.Fatal("Fetch with an unknown id returned nil error")
	}
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("error = %q, want it to name the unknown id", err.Error())
	}
}

func TestFetch_EmptyIDFails(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, sampleBacklog)
	f := backlog.NewFetcher(dir, "backlog.json")

	_, err := f.Fetch(context.Background(), "   ")
	if err == nil {
		t.Fatal("Fetch with an empty id returned nil error")
	}
}

func TestFetch_MissingFileFails(t *testing.T) {
	dir := t.TempDir()
	f := backlog.NewFetcher(dir, "backlog.json")

	_, err := f.Fetch(context.Background(), "1")
	if err == nil {
		t.Fatal("Fetch against a missing backlog.json returned nil error")
	}
}

func TestFetch_MalformedJSONFails(t *testing.T) {
	dir := t.TempDir()
	writeBacklog(t, dir, `{not valid json`)
	f := backlog.NewFetcher(dir, "backlog.json")

	_, err := f.Fetch(context.Background(), "1")
	if err == nil {
		t.Fatal("Fetch against malformed JSON returned nil error")
	}
}

func TestNewFetcher_DefaultPathUsedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, backlog.DefaultPath), []byte(sampleBacklog), 0o644); err != nil {
		t.Fatalf("write default backlog: %v", err)
	}
	f := backlog.NewFetcher(dir, "")

	issue, err := f.Fetch(context.Background(), "1")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "1" {
		t.Errorf("ID = %q, want %q", issue.ID, "1")
	}
}
