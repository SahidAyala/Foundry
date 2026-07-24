package asana

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeDoer is an injectable doer that returns a canned response and
// captures the request it received — mirrors ticket/jira's own test
// double.
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
		apiToken: "test-token",
		endpoint: defaultEndpoint,
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

const sampleTaskBody = `{
	"data": {
		"gid": "123456789",
		"name": "Fix the greeting",
		"notes": "It says goodbye instead of hello.",
		"permalink_url": "https://app.asana.com/0/1/123456789"
	}
}`

func TestFetch_Success(t *testing.T) {
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, sampleTaskBody)}
	f := newTestFetcher(d)

	issue, err := f.Fetch(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if issue.ID != "123456789" {
		t.Errorf("ID = %q, want %q", issue.ID, "123456789")
	}
	if issue.Title != "Fix the greeting" {
		t.Errorf("Title = %q, want %q", issue.Title, "Fix the greeting")
	}
	if issue.Description != "It says goodbye instead of hello." {
		t.Errorf("Description = %q, want %q", issue.Description, "It says goodbye instead of hello.")
	}
	if issue.URL != "https://app.asana.com/0/1/123456789" {
		t.Errorf("URL = %q, want the task's permalink", issue.URL)
	}

	if got := d.gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := d.gotReq.URL.String(); !strings.Contains(got, "/tasks/123456789") {
		t.Errorf("request URL = %q, want it to hit the task endpoint", got)
	}
}

func TestFetch_EmptyID(t *testing.T) {
	f := newTestFetcher(&fakeDoer{})
	if _, err := f.Fetch(context.Background(), "   "); err == nil {
		t.Fatal("Fetch returned nil error for an empty id")
	}
}

func TestFetch_TransportError(t *testing.T) {
	f := newTestFetcher(&fakeDoer{err: context.DeadlineExceeded})
	if _, err := f.Fetch(context.Background(), "1"); err == nil {
		t.Fatal("Fetch returned nil error on transport failure")
	}
}

func TestFetch_Unauthorized(t *testing.T) {
	body := `{"errors": [{"message": "Authentication token was not provided or invalid"}]}`
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusUnauthorized, body)})

	_, err := f.Fetch(context.Background(), "1")
	if err == nil {
		t.Fatal("Fetch returned nil error on 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want it to mention 'authentication failed'", err)
	}
	if !strings.Contains(err.Error(), "Authentication token was not provided") {
		t.Errorf("error = %q, want it to include Asana's own error message", err)
	}
}

func TestFetch_NotFound(t *testing.T) {
	body := `{"errors": [{"message": "Object specified does not exist"}]}`
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusNotFound, body)})

	_, err := f.Fetch(context.Background(), "999")
	if err == nil {
		t.Fatal("Fetch returned nil error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err)
	}
}

func TestFetch_ErrorBodyNotJSON(t *testing.T) {
	f := newTestFetcher(&fakeDoer{resp: jsonResponse(http.StatusBadGateway, "upstream connect error")})

	_, err := f.Fetch(context.Background(), "1")
	if err == nil {
		t.Fatal("Fetch returned nil error on a non-JSON error body")
	}
	if !strings.Contains(err.Error(), "upstream connect error") {
		t.Errorf("error = %q, want it to include the raw error body when it isn't JSON", err)
	}
}
