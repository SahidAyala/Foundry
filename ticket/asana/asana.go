// Package asana implements ticket.Fetcher against Asana's REST API, the
// fourth ticket provider (GitHub, Jira, GitLab, then Asana — the
// maintainer's own stated priority order, GitHub/Jira most common).
//
// Like ticket/jira, Asana has no first-party CLI convention to piggy-back
// on — this is a pure HTTP API call, authenticating with a Personal
// Access Token as a Bearer token (developers.asana.com/reference/gettask),
// the same invocation shape executor/openai/executor/gemini/ticket/jira
// already establish. Foundry never persists or logs the token; per
// ADR-0005 Decision 5's credential pattern (mirrored here for a new kind
// of external system, not a model vendor), resolving it from the
// environment variable a project's Config names
// (project.Config.AsanaAPITokenEnv) is the caller's responsibility.
//
// This package is substrate (docs/05-reference/invariants.md I12): it
// only fetches an Issue (an Asana "task"). It never builds a
// domain.Intent, runs a Pipeline, or seeks approval — those remain
// session.IssueCommand's and the Engine's responsibilities.
package asana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"foundry/ticket"
)

const (
	defaultEndpoint = "https://app.asana.com/api/1.0/tasks/%s?opt_fields=gid,name,notes,permalink_url"
	defaultTimeout  = 30 * time.Second
)

// Fetcher fetches an Asana task's content via Asana's REST API.
type Fetcher struct {
	apiToken string
	endpoint string
	timeout  time.Duration
	doer     doer
}

// NewFetcher returns a Fetcher authenticating with apiToken (a Personal
// Access Token, developers.asana.com/reference/gettask — "tasks:read"
// scope).
func NewFetcher(apiToken string) *Fetcher {
	return &Fetcher{
		apiToken: apiToken,
		endpoint: defaultEndpoint,
		timeout:  defaultTimeout,
		doer:     http.DefaultClient,
	}
}

var _ ticket.Fetcher = (*Fetcher)(nil)

// doer sends an HTTP request and returns its response — the same seam
// executor/openai, executor/gemini, and ticket/jira already use to let
// tests exercise Fetcher without a real network call.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// taskResponse is Asana's own documented GET /tasks/{task_gid} response
// shape — a "data" envelope around the task itself. Asana calls a ticket
// a "task"; "name" is its title, "notes" its plain-text description (no
// rich-text document format the way Jira's ADF is), and
// "permalink_url" its browser link.
type taskResponse struct {
	Data struct {
		GID          string `json:"gid"`
		Name         string `json:"name"`
		Notes        string `json:"notes"`
		PermalinkURL string `json:"permalink_url"`
	} `json:"data"`
}

// asanaErrorResponse is Asana's documented error envelope for a non-2xx
// response.
type asanaErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Fetch calls GET /tasks/{id} and decodes the result into a ticket.Issue.
func (f *Fetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ticket.Issue{}, errors.New("asana: task id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	url := fmt.Sprintf(f.endpoint, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("asana: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := f.doer.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ticket.Issue{}, fmt.Errorf("asana: timed out after %s", f.timeout)
		}
		return ticket.Issue{}, fmt.Errorf("asana: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("asana: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ticket.Issue{}, statusError(resp.StatusCode, body)
	}

	var decoded taskResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ticket.Issue{}, fmt.Errorf("asana: decode task %s: %w", id, err)
	}

	return ticket.Issue{
		ID:          decoded.Data.GID,
		Title:       decoded.Data.Name,
		Description: decoded.Data.Notes,
		URL:         decoded.Data.PermalinkURL,
	}, nil
}

// statusError renders a diagnostic error for a non-2xx response,
// preferring Asana's own documented {"errors": [{"message": ...}]} body
// when it parses.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var decoded asanaErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && len(decoded.Errors) > 0 {
		messages := make([]string, len(decoded.Errors))
		for i, e := range decoded.Errors {
			messages[i] = e.Message
		}
		message = strings.Join(messages, "; ")
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("asana: authentication failed (status %d): %s", status, message)
	case status == http.StatusNotFound:
		return fmt.Errorf("asana: task not found (status %d): %s", status, message)
	case status >= 500:
		return fmt.Errorf("asana: server error (status %d): %s", status, message)
	default:
		return fmt.Errorf("asana: request rejected (status %d): %s", status, message)
	}
}
