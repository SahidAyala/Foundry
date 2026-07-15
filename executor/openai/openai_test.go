package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

// fakeDoer is an injectable doer that returns a canned response (or
// transport error) and captures the request it received.
type fakeDoer struct {
	resp *http.Response
	err  error

	gotReq  *http.Request
	gotBody string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.gotReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		f.gotBody = string(b)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestExecutor(d doer) *Executor {
	return &Executor{
		model:    "gpt-5.1",
		apiKey:   "test-key",
		endpoint: "https://example.invalid/v1/chat/completions",
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

// sampleDiff ends in a newline because git apply rejects a patch whose last
// line is unterminated; executor.ParsePatch guarantees this normalization.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

// fixtureChatCompletion is a hand-authored struct matching OpenAI's
// documented Chat Completions response shape (as of this writing). It is
// NOT a captured live transcript — this environment has no OPENAI_API_KEY
// to record one against the real API. It exists to prove the adapter
// decodes the documented shape correctly; recording a genuine cassette
// against a live call is left for whoever next has credentials and wants
// one.
type fixtureChatCompletion struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func successFixture(t *testing.T, content string) string {
	t.Helper()
	fixture := fixtureChatCompletion{
		ID:      "chatcmpl-fixture-0001",
		Object:  "chat.completion",
		Created: 1730000000,
		Model:   "gpt-5.1",
	}
	fixture.Choices = make([]struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}, 1)
	fixture.Choices[0].Index = 0
	fixture.Choices[0].Message.Role = "assistant"
	fixture.Choices[0].Message.Content = content
	fixture.Choices[0].FinishReason = "stop"
	fixture.Usage.PromptTokens = 42
	fixture.Usage.CompletionTokens = 17
	fixture.Usage.TotalTokens = 59

	body, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(body)
}

func TestExecute_Success(t *testing.T) {
	body := successFixture(t, "```diff\n"+sampleDiff+"```\n")
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	e := newTestExecutor(d)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "add a comment"}, []string{"main.go contents"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}

	if d.gotReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want %q", d.gotReq.Header.Get("Authorization"), "Bearer test-key")
	}
	if d.gotReq.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", d.gotReq.Header.Get("Content-Type"), "application/json")
	}
	if !strings.Contains(d.gotBody, "add a comment") {
		t.Errorf("request body missing intent, got:\n%s", d.gotBody)
	}
	if !strings.Contains(d.gotBody, "main.go contents") {
		t.Errorf("request body missing gathered context, got:\n%s", d.gotBody)
	}
	if !strings.Contains(d.gotBody, `"role":"system"`) {
		t.Errorf("request body missing a system-role message, got:\n%s", d.gotBody)
	}
}

func TestExecute_TransportError(t *testing.T) {
	e := newTestExecutor(&fakeDoer{err: errors.New("connection refused")})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on transport failure")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %q, want it to mention 'request failed'", err)
	}
}

func TestExecute_Unauthorized(t *testing.T) {
	body := `{"error": {"message": "Incorrect API key provided", "type": "invalid_request_error"}}`
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusUnauthorized, body)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want it to mention 'authentication failed'", err)
	}
	if !strings.Contains(err.Error(), "Incorrect API key provided") {
		t.Errorf("error = %q, want it to include the vendor's own error message", err)
	}
}

func TestExecute_RateLimited(t *testing.T) {
	body := `{"error": {"message": "Rate limit reached", "type": "rate_limit_error"}}`
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusTooManyRequests, body)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want it to mention 'rate limited'", err)
	}
}

func TestExecute_ServerError(t *testing.T) {
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusInternalServerError, `{"error": {"message": "internal error"}}`)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on 500")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("error = %q, want it to mention 'server error'", err)
	}
}

func TestExecute_ErrorBodyNotJSON(t *testing.T) {
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusBadGateway, "upstream connect error")})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on a non-JSON error body")
	}
	if !strings.Contains(err.Error(), "upstream connect error") {
		t.Errorf("error = %q, want it to include the raw error body when it isn't JSON", err)
	}
}

func TestExecute_NoChoicesInResponse(t *testing.T) {
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusOK, `{"id": "chatcmpl-empty", "choices": []}`)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for a response with no choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %q, want it to mention 'no choices'", err)
	}
}

func TestExecute_NoDiffInContent(t *testing.T) {
	body := successFixture(t, "I could not make the change.")
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusOK, body)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error when the message content has no diff")
	}
}

func TestEstimateCostUSD_KnownModelUsesItsRate(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})
	e.model = "gpt-5.1-mini"

	cost, err := e.EstimateCostUSD(context.Background(), &domain.Intent{Text: strings.Repeat("x", 4000)}, nil)
	if err != nil {
		t.Fatalf("EstimateCostUSD failed: %v", err)
	}
	// 4000 chars / 4 chars-per-token = 1000 tokens; gpt-5.1-mini is
	// $1.00 per million tokens => 1000/1_000_000 * 1.00 = 0.001.
	want := 0.001
	if cost != want {
		t.Errorf("EstimateCostUSD = %v, want %v", cost, want)
	}
}

func TestEstimateCostUSD_UnknownModelFallsBackToDefaultRate(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})
	e.model = "some-future-model"

	cost, err := e.EstimateCostUSD(context.Background(), &domain.Intent{Text: strings.Repeat("x", 4000)}, nil)
	if err != nil {
		t.Fatalf("EstimateCostUSD failed: %v", err)
	}
	want := 4000 / charsPerTokenEstimate / 1_000_000.0 * defaultCostPerMillionTokensUSD
	if cost != want {
		t.Errorf("EstimateCostUSD = %v, want %v", cost, want)
	}
}

func TestEstimateCostUSD_IncludesConsideredContext(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})

	withoutContext, err := e.EstimateCostUSD(context.Background(), &domain.Intent{Text: "short"}, nil)
	if err != nil {
		t.Fatalf("EstimateCostUSD failed: %v", err)
	}
	withContext, err := e.EstimateCostUSD(context.Background(), &domain.Intent{Text: "short"}, []string{strings.Repeat("y", 8000)})
	if err != nil {
		t.Fatalf("EstimateCostUSD failed: %v", err)
	}
	if withContext <= withoutContext {
		t.Errorf("EstimateCostUSD with considered context = %v, want it greater than without (%v)", withContext, withoutContext)
	}
}
