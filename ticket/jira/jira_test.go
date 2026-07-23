package jira

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeDoer is an injectable doer that returns a canned response and
// captures the request it received — mirrors executor/openai's and
// executor/gemini's own test double.
type fakeDoer struct {
	resp *http.Response
	err  error

	gotReq *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestFetcher(d doer) *Fetcher {
	return &Fetcher{
		baseURL:  "https://example.atlassian.net",
		email:    "test@example.com",
		apiToken: "test-token",
		timeout:  time.Minute,
		doer:     d,
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const sampleIssueBody = `{
	"key": "PROJ-123",
	"fields": {
		"summary": "Fix the greeting",
		"description": {
			"type": "doc",
			"version": 1,
			"content": [
				{"type": "paragraph", "content": [{"type": "text", "text": "It says goodbye instead of hello."}]}
			]
		}
	}
}`

func TestFetch_Success(t *testing.T) {
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, sampleIssueBody)}
	f := newTestFetcher(d)

	issue, err := f.Fetch(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "PROJ-123" {
		t.Errorf("ID = %q, want %q", issue.ID, "PROJ-123")
	}
	if issue.Title != "Fix the greeting" {
		t.Errorf("Title = %q, want %q", issue.Title, "Fix the greeting")
	}
	if issue.Description != "It says goodbye instead of hello." {
		t.Errorf("Description = %q, want the plain-text-extracted ADF content", issue.Description)
	}
	if issue.URL != "https://example.atlassian.net/browse/PROJ-123" {
		t.Errorf("URL = %q, want the issue's browse URL", issue.URL)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("test@example.com:test-token"))
	if got := d.gotReq.Header.Get("Authorization"); got != wantAuth {
		t.Errorf("Authorization header = %q, want %q", got, wantAuth)
	}
	if got := d.gotReq.URL.String(); !strings.Contains(got, "/rest/api/3/issue/PROJ-123") {
		t.Errorf("request URL = %q, want it to hit the issue endpoint", got)
	}
	if got := d.gotReq.URL.String(); !strings.Contains(got, "fields=summary%2Cdescription") && !strings.Contains(got, "fields=summary,description") {
		t.Errorf("request URL = %q, want it to request only summary,description", got)
	}
}

func TestFetch_NullDescriptionYieldsEmptyString(t *testing.T) {
	body := `{"key": "PROJ-1", "fields": {"summary": "No description here", "description": null}}`
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	f := newTestFetcher(d)

	issue, err := f.Fetch(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.Description != "" {
		t.Errorf("Description = %q, want empty for a null ADF description", issue.Description)
	}
}

func TestFetch_MultipleParagraphsAndHeading(t *testing.T) {
	body := `{
		"key": "PROJ-9",
		"fields": {
			"summary": "Multi-paragraph",
			"description": {
				"type": "doc",
				"content": [
					{"type": "heading", "content": [{"type": "text", "text": "Steps to reproduce"}]},
					{"type": "paragraph", "content": [{"type": "text", "text": "First, do this."}]},
					{"type": "paragraph", "content": [{"type": "text", "text": "Then, do that."}]}
				]
			}
		}
	}`
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	f := newTestFetcher(d)

	issue, err := f.Fetch(context.Background(), "PROJ-9")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	for _, want := range []string{"Steps to reproduce", "First, do this.", "Then, do that."} {
		if !strings.Contains(issue.Description, want) {
			t.Errorf("Description = %q, want it to contain %q", issue.Description, want)
		}
	}
}

func TestFetch_EmptyID(t *testing.T) {
	f := newTestFetcher(&fakeDoer{})
	if _, err := f.Fetch(context.Background(), "   "); err == nil {
		t.Fatal("Fetch returned nil error for an empty id")
	}
}

func TestFetch_NoBaseURLConfigured(t *testing.T) {
	f := newTestFetcher(&fakeDoer{})
	f.baseURL = ""
	if _, err := f.Fetch(context.Background(), "PROJ-1"); err == nil {
		t.Fatal("Fetch returned nil error with no base URL configured")
	}
}

func TestFetch_TransportError(t *testing.T) {
	f := newTestFetcher(&fakeDoer{err: context.DeadlineExceeded})
	_, err := f.Fetch(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("Fetch returned nil error on transport failure")
	}
}

func TestFetch_Unauthorized(t *testing.T) {
	body := `{"errorMessages": ["You do not have permission to view this issue."]}`
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusUnauthorized, body)})

	_, err := f.Fetch(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("Fetch returned nil error on 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want it to mention 'authentication failed'", err)
	}
	if !strings.Contains(err.Error(), "You do not have permission") {
		t.Errorf("error = %q, want it to include Jira's own error message", err)
	}
}

func TestFetch_NotFound(t *testing.T) {
	body := `{"errorMessages": ["Issue does not exist"]}`
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusNotFound, body)})

	_, err := f.Fetch(context.Background(), "PROJ-999")
	if err == nil {
		t.Fatal("Fetch returned nil error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err)
	}
}

func TestFetch_ErrorBodyNotJSON(t *testing.T) {
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusBadGateway, "upstream connect error")})

	_, err := f.Fetch(context.Background(), "PROJ-1")
	if err == nil {
		t.Fatal("Fetch returned nil error on a non-JSON error body")
	}
	if !strings.Contains(err.Error(), "upstream connect error") {
		t.Errorf("error = %q, want it to include the raw error body when it isn't JSON", err)
	}
}

func TestAdfToPlainText_EmptyDoc(t *testing.T) {
	if got := adfToPlainText(nil); got != "" {
		t.Errorf("adfToPlainText(nil) = %q, want empty", got)
	}
	if got := adfToPlainText([]byte("null")); got != "" {
		t.Errorf(`adfToPlainText("null") = %q, want empty`, got)
	}
}
