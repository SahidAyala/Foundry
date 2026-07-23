// Package github implements ticket.Fetcher by shelling out to the gh CLI
// (`gh issue view <number> --json ...`) — reusing the exact same
// already-authenticated gh session vcs.GitHubPRApplier's own PR-opening
// already requires. Foundry reads no separate credential for this: gh's
// own `gh auth login` (or a `GH_TOKEN` it already reads itself) is
// entirely outside Foundry's knowledge, the same PIC-2-style pattern
// executor/claude and executor/geminicli already establish for their own
// vendor CLIs.
//
// This package is substrate (docs/05-reference/invariants.md I12): it
// only fetches an Issue. It never builds a domain.Intent, runs a
// Pipeline, or seeks approval — those remain session.IssueCommand's and
// the Engine's responsibilities.
package github

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

// Fetcher fetches a GitHub issue's content via the gh CLI, run in a fixed
// repository directory so gh can infer which repository to query (the
// same convention `gh pr create` already relies on, no repository name or
// owner ever passed explicitly).
type Fetcher struct {
	workspace string
	run       ghRunner
}

// NewFetcher returns a Fetcher that runs gh in workspace.
func NewFetcher(workspace string) *Fetcher {
	return &Fetcher{workspace: workspace, run: runGH}
}

var _ ticket.Fetcher = (*Fetcher)(nil)

// issueJSON is gh issue view's own documented --json field shape (a
// strict subset — only the fields this Fetcher asks for are requested).
type issueJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

// Fetch runs `gh issue view <id> --json number,title,body,url` and
// decodes its output into a ticket.Issue. id must be the issue's bare
// number (gh issue view also accepts a full URL, but session.IssueCommand
// only ever passes what a user typed after "/issue", by convention a bare
// number).
func (f *Fetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ticket.Issue{}, errors.New("github: issue id is required")
	}
	if _, err := strconv.Atoi(id); err != nil {
		return ticket.Issue{}, fmt.Errorf("github: issue id %q is not a number (gh issue view expects one, e.g. /issue 42)", id)
	}

	out, err := f.run(ctx, f.workspace, []string{"issue", "view", id, "--json", "number,title,body,url"})
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("github: fetch issue %s: %w", id, err)
	}

	var decoded issueJSON
	if err := json.Unmarshal(out, &decoded); err != nil {
		return ticket.Issue{}, fmt.Errorf("github: decode issue %s: %w", id, err)
	}
	return ticket.Issue{
		ID:          strconv.Itoa(decoded.Number),
		Title:       decoded.Title,
		Description: decoded.Body,
		URL:         decoded.URL,
	}, nil
}

// ghRunner runs the gh CLI with args in dir and returns its stdout. It is
// the seam Fetch calls through instead of shelling out directly, so tests
// never require a real gh binary, network access, or GitHub credentials —
// mirroring vcs.GitHubPRApplier's own ghRunner seam.
type ghRunner func(ctx context.Context, dir string, args []string) ([]byte, error)

// runGH is the production ghRunner: a real gh subprocess.
func runGH(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
