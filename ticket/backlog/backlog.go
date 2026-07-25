// Package backlog implements ticket.Fetcher by reading a local,
// project-committed JSON file instead of calling any external ticketing
// API — the same "Context Source" extension point ticket/github,
// ticket/jira, ticket/gitlab, and ticket/asana already implement
// (docs/02-architecture/extensibility.md), for a project that tracks
// work as a plain file in its own repository instead of (or alongside)
// an external tracker.
//
// The shape (an ordered list of features, each with a status and
// acceptance criteria) is deliberately close to a pattern validated
// against a small "harness engineering" example project before adopting
// it here — but this package is exactly one more Fetcher, nothing more:
// no new Step kind, no orchestration change, no automatic status
// mutation. Fetch is read-only, mirroring every other ticket.Fetcher —
// nothing here writes back to the backlog file, the same way
// ticket/github never moves a GitHub issue to "in progress" either.
package backlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"foundry/ticket"
)

// DefaultPath is where Fetcher reads its backlog from, relative to the
// project's workspace, when NewFetcher is given an empty path — the same
// .foundry/ convention every other project-local file already follows
// (.foundry/pipelines, .foundry/knowledge, .foundry/executors.json).
const DefaultPath = ".foundry/backlog.json"

// NextID is the special id Fetch treats as "the first Feature whose
// Status is StatusPending, in file order" — letting `/issue next` work
// without session.IssueCommand needing to know anything about this
// provider's own convention; every other id is matched literally against
// a Feature's own ID.
const NextID = "next"

// Feature status values — the same vocabulary a project's own backlog.json
// uses, close to the harness pattern this package validated before
// adopting.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusBlocked    = "blocked"
)

// Feature is one backlog entry's on-disk shape.
type Feature struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Acceptance  []string `json:"acceptance,omitempty"`
	Status      string   `json:"status"`
}

// document is a backlog.json file's top-level shape.
type document struct {
	Features []Feature `json:"features"`
}

// Fetcher fetches a Feature from a local JSON file and normalizes it
// into a ticket.Issue.
type Fetcher struct {
	path string
}

// NewFetcher returns a Fetcher reading path (relative to workspace), or
// workspace/DefaultPath if path is empty.
func NewFetcher(workspace, path string) *Fetcher {
	if path == "" {
		path = DefaultPath
	}
	return &Fetcher{path: filepath.Join(workspace, path)}
}

var _ ticket.Fetcher = (*Fetcher)(nil)

// Fetch reads f.path and returns the Feature named by id as a
// ticket.Issue — id's special value NextID picks the first
// StatusPending Feature in file order instead of matching a literal ID.
// A Feature's Acceptance criteria, when present, are appended to the
// returned Issue's Description as a plain bulleted list, so they flow
// into the Act's Intent text (session.IssueCommand's formatIssueIntent)
// exactly like the rest of the description — and, from there, into
// anything reading domain.Outcome.Intent (e.g. verify/aireview).
func (f *Fetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ticket.Issue{}, fmt.Errorf("backlog: issue id is required (a literal id, or %q for the first pending feature)", NextID)
	}

	doc, err := f.load()
	if err != nil {
		return ticket.Issue{}, err
	}

	feature, err := selectFeature(doc.Features, id)
	if err != nil {
		return ticket.Issue{}, err
	}
	return toIssue(feature), nil
}

func (f *Fetcher) load() (document, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return document{}, fmt.Errorf("backlog: read %s: %w", f.path, err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, fmt.Errorf("backlog: decode %s: %w", f.path, err)
	}
	return doc, nil
}

func selectFeature(features []Feature, id string) (Feature, error) {
	if id == NextID {
		for _, feat := range features {
			if feat.Status == StatusPending {
				return feat, nil
			}
		}
		return Feature{}, fmt.Errorf("backlog: no feature with status %q found", StatusPending)
	}
	for _, feat := range features {
		if feat.ID == id {
			return feat, nil
		}
	}
	return Feature{}, fmt.Errorf("backlog: no feature with id %q found", id)
}

func toIssue(feat Feature) ticket.Issue {
	description := feat.Description
	if len(feat.Acceptance) > 0 {
		var b strings.Builder
		b.WriteString(description)
		b.WriteString("\n\nAcceptance criteria:\n")
		for _, a := range feat.Acceptance {
			fmt.Fprintf(&b, "- %s\n", a)
		}
		description = strings.TrimRight(b.String(), "\n")
	}
	return ticket.Issue{
		ID:          feat.ID,
		Title:       feat.Title,
		Description: description,
	}
}
