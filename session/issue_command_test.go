package session_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"foundry/session"
	"foundry/ticket"
)

// fakeTicketFetcher is an injectable ticket.Fetcher that returns a canned
// Issue (or error) and records the id it was asked to fetch.
type fakeTicketFetcher struct {
	issue ticket.Issue
	err   error

	gotID string
}

func (f *fakeTicketFetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	f.gotID = id
	return f.issue, f.err
}

func TestIssueCommand_EmptyArgsFails(t *testing.T) {
	s, _ := newTestSession(t, "y\n")
	s.SetTicketFetcher(&fakeTicketFetcher{})

	err := session.IssueCommand{PipelineName: "default"}.Run(context.Background(), s, "   ")
	if err == nil {
		t.Fatal("Run with empty args returned nil error")
	}
}

func TestIssueCommand_NoFetcherConfiguredFailsWithClearError(t *testing.T) {
	s, _ := newTestSession(t, "y\n")
	// Deliberately never call SetTicketFetcher.

	err := session.IssueCommand{PipelineName: "default"}.Run(context.Background(), s, "42")
	if err == nil {
		t.Fatal("Run with no ticket provider configured returned nil error")
	}
	if !strings.Contains(err.Error(), "no ticket provider configured") {
		t.Errorf("error = %q, want it to name the missing configuration", err)
	}
}

func TestIssueCommand_FetchFailurePropagates(t *testing.T) {
	s, _ := newTestSession(t, "y\n")
	s.SetTicketFetcher(&fakeTicketFetcher{err: errors.New("gh: issue not found")})

	err := session.IssueCommand{PipelineName: "default"}.Run(context.Background(), s, "999")
	if err == nil {
		t.Fatal("Run returned nil error when the fetcher failed")
	}
	if !strings.Contains(err.Error(), "issue not found") {
		t.Errorf("error = %q, want it to include the fetcher's own error", err)
	}
}

// TestIssueCommand_RunsAndRecordsOnApproval proves the whole chain end to
// end: a fetched Issue becomes an Intent, the named Pipeline actually
// runs over it, and — on approval — the Act is applied and recorded, the
// exact same outcome /feature's own equivalent test already proves for
// typed args.
func TestIssueCommand_RunsAndRecordsOnApproval(t *testing.T) {
	s, out := newTestSession(t, "y\n")
	fetcher := &fakeTicketFetcher{issue: ticket.Issue{
		ID:          "42",
		Title:       "Fix the greeting",
		Description: "It says goodbye instead of hello.",
		URL:         "https://github.com/example/repo/issues/42",
	}}
	s.SetTicketFetcher(fetcher)

	err := session.IssueCommand{PipelineName: "default"}.Run(context.Background(), s, "42")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if fetcher.gotID != "42" {
		t.Errorf("fetcher was asked to fetch %q, want %q", fetcher.gotID, "42")
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
	if !strings.Contains(acts[0].Intent, "Fix the greeting") {
		t.Errorf("recorded Intent = %q, want it to include the fetched issue's title", acts[0].Intent)
	}
	if !strings.Contains(acts[0].Intent, "issues/42") {
		t.Errorf("recorded Intent = %q, want it to include the fetched issue's URL", acts[0].Intent)
	}
}
