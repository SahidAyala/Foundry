// Package jira implements ticket.Fetcher against Jira Cloud's REST API
// (v3), the second ticket provider after ticket/github, per the
// maintainer's own stated priority order (GitHub, then Jira, then
// GitLab/Asana).
//
// Unlike GitHub (which reuses the gh CLI's own already-authenticated
// session), Jira has no equivalent first-party CLI convention to piggy-
// back on — this is a pure HTTP API call, authenticating with HTTP Basic
// Auth (base64 "email:api_token", Atlassian's own documented scheme,
// developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis)
// the same invocation shape executor/openai/executor/gemini already
// establish for their own vendors. Foundry never persists or logs the
// API token; per ADR-0005 Decision 5's credential pattern (mirrored here
// for a new kind of external system, not a model vendor), resolving it
// from the environment variable a project's Config names
// (project.Config.JiraAPITokenEnv) is the caller's responsibility.
//
// This package is substrate (docs/05-reference/invariants.md I12): it
// only fetches an Issue. It never builds a domain.Intent, runs a
// Pipeline, or seeks approval — those remain session.IssueCommand's and
// the Engine's responsibilities.
package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"foundry/ticket"
)

const defaultTimeout = 30 * time.Second

// Fetcher fetches a Jira issue's content via Jira Cloud's REST API v3.
type Fetcher struct {
	baseURL  string // e.g. "https://yourcompany.atlassian.net", no trailing slash
	email    string
	apiToken string
	timeout  time.Duration
	doer     doer
}

// NewFetcher returns a Fetcher against baseURL (a Jira Cloud site's own
// base URL, e.g. "https://yourcompany.atlassian.net"), authenticating as
// email with apiToken (an Atlassian API token, not the account
// password — id.atlassian.com/manage/api-tokens).
func NewFetcher(baseURL, email, apiToken string) *Fetcher {
	return &Fetcher{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		timeout:  defaultTimeout,
		doer:     http.DefaultClient,
	}
}

var _ ticket.Fetcher = (*Fetcher)(nil)

// doer sends an HTTP request and returns its response — the same seam
// executor/openai and executor/gemini already use to let tests exercise
// Fetcher without a real network call.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// issueResponse is Jira's own documented GET /rest/api/3/issue/{id}
// response shape — a strict subset: only the fields this Fetcher asks
// for via ?fields= are present, and only key/summary/description are
// decoded.
type issueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
	} `json:"fields"`
}

// jiraErrorResponse is Jira's documented error envelope for a non-2xx
// response.
type jiraErrorResponse struct {
	ErrorMessages []string `json:"errorMessages"`
}

// Fetch calls GET {baseURL}/rest/api/3/issue/{id}?fields=summary,description
// and decodes the result into a ticket.Issue, converting the response's
// Atlassian Document Format description into plain text.
func (f *Fetcher) Fetch(ctx context.Context, id string) (ticket.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ticket.Issue{}, errors.New("jira: issue id is required")
	}
	if f.baseURL == "" {
		return ticket.Issue{}, errors.New("jira: no base URL configured (jira_base_url in .foundry/config.json)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,description", f.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("jira: build request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(f.email, f.apiToken))
	req.Header.Set("Accept", "application/json")

	resp, err := f.doer.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ticket.Issue{}, fmt.Errorf("jira: timed out after %s", f.timeout)
		}
		return ticket.Issue{}, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ticket.Issue{}, fmt.Errorf("jira: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ticket.Issue{}, statusError(resp.StatusCode, body)
	}

	var decoded issueResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ticket.Issue{}, fmt.Errorf("jira: decode issue %s: %w", id, err)
	}

	return ticket.Issue{
		ID:          decoded.Key,
		Title:       decoded.Fields.Summary,
		Description: adfToPlainText(decoded.Fields.Description),
		URL:         fmt.Sprintf("%s/browse/%s", f.baseURL, decoded.Key),
	}, nil
}

// basicAuth renders the base64(email:apiToken) string Jira's Basic Auth
// scheme expects.
func basicAuth(email, apiToken string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
}

// statusError renders a diagnostic error for a non-2xx response,
// preferring Jira's own documented {"errorMessages": [...]} body when it
// parses.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var decoded jiraErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && len(decoded.ErrorMessages) > 0 {
		message = strings.Join(decoded.ErrorMessages, "; ")
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("jira: authentication failed (status %d): %s", status, message)
	case status == http.StatusNotFound:
		return fmt.Errorf("jira: issue not found (status %d): %s", status, message)
	case status >= 500:
		return fmt.Errorf("jira: server error (status %d): %s", status, message)
	default:
		return fmt.Errorf("jira: request rejected (status %d): %s", status, message)
	}
}

// adfNode is one node of an Atlassian Document Format tree
// (developer.atlassian.com/cloud/jira/platform/apis/document/structure) —
// only the fields adfToPlainText actually reads.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

// blockLevelADFTypes are ADF node types adfToPlainText inserts a newline
// after, so extracted text keeps roughly the same paragraph/line
// structure the original document had, rather than becoming one run-on
// line.
var blockLevelADFTypes = map[string]bool{
	"paragraph": true,
	"heading":   true,
	"listItem":  true,
	"codeBlock": true,
}

// adfToPlainText extracts a plain-text rendering of an Atlassian Document
// Format description — Jira Cloud's own rich-text JSON representation,
// not a plain string. An empty or unparseable doc (a Jira issue with no
// description at all decodes to JSON null) yields an empty string rather
// than an error: a missing description is not a fetch failure.
func adfToPlainText(doc json.RawMessage) string {
	if len(doc) == 0 || string(doc) == "null" {
		return ""
	}
	var root adfNode
	if err := json.Unmarshal(doc, &root); err != nil {
		return ""
	}
	var b strings.Builder
	walkADF(root, &b)
	return strings.TrimSpace(b.String())
}

func walkADF(node adfNode, b *strings.Builder) {
	if node.Type == "text" {
		b.WriteString(node.Text)
	}
	for _, child := range node.Content {
		walkADF(child, b)
	}
	if blockLevelADFTypes[node.Type] {
		b.WriteString("\n")
	}
}
