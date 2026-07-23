package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func newTestFetcher(run ghRunner) *Fetcher {
	return &Fetcher{workspace: "/repo", run: run}
}

func TestFetch_Success(t *testing.T) {
	var gotDir string
	var gotArgs []string
	run := func(ctx context.Context, dir string, args []string) ([]byte, error) {
		gotDir, gotArgs = dir, args
		return []byte(`{"number":42,"title":"Fix the greeting","body":"It says goodbye instead of hello.","url":"https://github.com/example/repo/issues/42"}`), nil
	}
	f := newTestFetcher(run)

	issue, err := f.Fetch(context.Background(), "42")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "42" {
		t.Errorf("ID = %q, want %q", issue.ID, "42")
	}
	if issue.Title != "Fix the greeting" {
		t.Errorf("Title = %q, want %q", issue.Title, "Fix the greeting")
	}
	if issue.Description != "It says goodbye instead of hello." {
		t.Errorf("Description = %q, want %q", issue.Description, "It says goodbye instead of hello.")
	}
	if issue.URL != "https://github.com/example/repo/issues/42" {
		t.Errorf("URL = %q, want the issue URL", issue.URL)
	}

	if gotDir != "/repo" {
		t.Errorf("gh dir = %q, want %q", gotDir, "/repo")
	}
	wantArgs := []string{"issue", "view", "42", "--json", "number,title,body,url"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("gh args = %v, want %v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("gh args = %v, want %v", gotArgs, wantArgs)
		}
	}
}

func TestFetch_EmptyID(t *testing.T) {
	f := newTestFetcher(func(ctx context.Context, dir string, args []string) ([]byte, error) {
		t.Fatal("gh should never be invoked for an empty id")
		return nil, nil
	})

	if _, err := f.Fetch(context.Background(), "   "); err == nil {
		t.Fatal("Fetch returned nil error for an empty id")
	}
}

func TestFetch_NonNumericID(t *testing.T) {
	f := newTestFetcher(func(ctx context.Context, dir string, args []string) ([]byte, error) {
		t.Fatal("gh should never be invoked for a non-numeric id")
		return nil, nil
	})

	_, err := f.Fetch(context.Background(), "PROJ-123")
	if err == nil {
		t.Fatal("Fetch returned nil error for a non-numeric id")
	}
	if !strings.Contains(err.Error(), "not a number") {
		t.Errorf("error = %q, want it to explain the id must be numeric", err)
	}
}

func TestFetch_GHFailurePropagates(t *testing.T) {
	f := newTestFetcher(func(ctx context.Context, dir string, args []string) ([]byte, error) {
		return nil, errors.New("gh: issue not found")
	})

	_, err := f.Fetch(context.Background(), "999")
	if err == nil {
		t.Fatal("Fetch returned nil error when gh failed")
	}
	if !strings.Contains(err.Error(), "issue not found") {
		t.Errorf("error = %q, want it to include gh's own error", err)
	}
}

func TestFetch_UnparseableOutput(t *testing.T) {
	f := newTestFetcher(func(ctx context.Context, dir string, args []string) ([]byte, error) {
		return []byte("not json"), nil
	})

	if _, err := f.Fetch(context.Background(), "1"); err == nil {
		t.Fatal("Fetch returned nil error for unparseable gh output")
	}
}
