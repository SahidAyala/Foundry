// Package gitlab implements ticket.Fetcher by shelling out to the glab
// CLI (`glab issue view <id> --output json`) — reusing the exact same
// already-authenticated glab session the same way ticket/github reuses
// gh's. Foundry reads no separate credential for this: glab's own `glab
// auth login` (or a token it already reads itself) is entirely outside
// Foundry's knowledge, the same PIC-2-style pattern executor/claude and
// executor/geminicli already establish for their own vendor CLIs.
//
// This package is substrate (docs/05-reference/invariants.md I12): it
// only fetches an Issue. It never builds a domain.Intent, runs a
// Pipeline, or seeks approval — those remain session.IssueCommand's and
// the Engine's responsibilities.
package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"foundry/ticket"
)

// Fetcher fetches a GitLab issue's content via the glab CLI, run in a
// fixed repository directory so glab can infer which project to query
// (the same convention `gh pr create` already relies on for GitHub, no
// project path passed explicitly).
type Fetcher struct {
	workspace string
	run       glabRunner
}

// NewFetcher returns a Fetcher that runs glab in workspace.
func NewFetcher(workspace string) *Fetcher {
	return &Fetcher{workspace: workspace, run: runGlab}
}

var _ ticket.Fetcher = (*Fetcher)(nil)

// issueJSON is glab issue view --output json's own documented field
// shape, mirroring GitLab's REST API issue object
// (docs.gitlab.com/api/issues) — a strict subset: only the fields this
// Fetcher reads.
type issueJSON struct {
	IID         int    `json:"iid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	WebURL      string `json:"web_url"`
}

// Fetch runs `glab issue view <id> --output json` and decodes its output
// into a ticket.Issue. id must be the issue's bare IID (glab issue view
// also accepts a full URL, but session.IssueCommand only ever passes
// what a user typed after "/issue", by convention a bare number).
func (f *Fetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ticket.Issue{}, errors.New("gitlab: issue id is required")
	}
	if _, err := strconv.Atoi(id); err != nil {
		return ticket.Issue{}, fmt.Errorf("gitlab: issue id %q is not a number (glab issue view expects one, e.g. /issue 42)", id)
	}

	out, err := f.run(ctx, f.workspace, []string{"issue", "view", id, "--output", "json"})
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("gitlab: fetch issue %s: %w", id, err)
	}

	var decoded issueJSON
	if err := json.Unmarshal(out, &decoded); err != nil {
		return ticket.Issue{}, fmt.Errorf("gitlab: decode issue %s: %w", id, err)
	}
	return ticket.Issue{
		ID:          strconv.Itoa(decoded.IID),
		Title:       decoded.Title,
		Description: decoded.Description,
		URL:         decoded.WebURL,
	}, nil
}

// glabRunner runs the glab CLI with args in dir and returns its stdout.
// It is the seam Fetch calls through instead of shelling out directly,
// so tests never require a real glab binary, network access, or GitLab
// credentials — mirroring ticket/github's own ghRunner seam.
type glabRunner func(ctx context.Context, dir string, args []string) ([]byte, error)

// runGlab is the production glabRunner: a real glab subprocess.
func runGlab(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("glab %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("glab %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
