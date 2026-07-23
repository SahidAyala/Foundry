package gemini

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
// transport error) and captures the request it received — the same shape
// executor/openai's own test double uses.
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
		model:    "gemini-3.5-flash",
		apiKey:   "test-key",
		endpoint: "https://example.invalid/v1beta/models/gemini-3.5-flash:generateContent",
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

// fixtureGenerateContent is a hand-authored struct matching Gemini's
// documented generateContent response shape (ai.google.dev/api/generate-content,
// as of this writing). It is NOT a captured live transcript — this
// environment has no GEMINI_API_KEY to record one against the real API. It
// exists to prove the adapter decodes the documented shape correctly.
func successFixture(t *testing.T, text string) string {
	t.Helper()
	fixture := struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion"`
	}{}
	fixture.Candidates = make([]struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	}, 1)
	fixture.Candidates[0].Content.Role = "model"
	fixture.Candidates[0].Content.Parts = []struct {
		Text string `json:"text"`
	}{{Text: text}}
	fixture.Candidates[0].FinishReason = "STOP"
	fixture.UsageMetadata.PromptTokenCount = 42
	fixture.UsageMetadata.CandidatesTokenCount = 17
	fixture.UsageMetadata.TotalTokenCount = 59
	fixture.ModelVersion = "gemini-3.5-flash"

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
	// successFixture's usage is PromptTokenCount: 42, CandidatesTokenCount:
	// 17 -- 59 tokens at gemini-3.5-flash's $5.25/million blended rate
	// (costPerMillionTokensUSD). tokens is a variable (not a constant
	// expression) so this runs the same float64 runtime arithmetic
	// actualCostUSD itself does, rather than Go's arbitrary-precision
	// constant folding, which can round the very last bit differently.
	tokens := 59.0
	wantActualCost := tokens / 1_000_000 * 5.25
	if outcome.ActualCostUSD == nil {
		t.Fatal("ActualCostUSD = nil, want a real value derived from the fixture's usage")
	}
	if *outcome.ActualCostUSD != wantActualCost {
		t.Errorf("ActualCostUSD = %v, want %v", *outcome.ActualCostUSD, wantActualCost)
	}

	if d.gotReq.Header.Get("x-goog-api-key") != "test-key" {
		t.Errorf("x-goog-api-key header = %q, want %q", d.gotReq.Header.Get("x-goog-api-key"), "test-key")
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
	if !strings.Contains(d.gotBody, `"systemInstruction"`) {
		t.Errorf("request body missing systemInstruction, got:\n%s", d.gotBody)
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
	body := `{"error": {"code": 401, "message": "API key not valid", "status": "UNAUTHENTICATED"}}`
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusUnauthorized, body)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want it to mention 'authentication failed'", err)
	}
	if !strings.Contains(err.Error(), "API key not valid") {
		t.Errorf("error = %q, want it to include the vendor's own error message", err)
	}
}

func TestExecute_RateLimited(t *testing.T) {
	body := `{"error": {"code": 429, "message": "Resource exhausted", "status": "RESOURCE_EXHAUSTED"}}`
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

func TestExecute_NoCandidatesInResponse(t *testing.T) {
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusOK, `{"candidates": []}`)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for a response with no candidates")
	}
	if !strings.Contains(err.Error(), "no candidates") {
		t.Errorf("error = %q, want it to mention 'no candidates'", err)
	}
}

func TestExecute_NoDiffInContent(t *testing.T) {
	body := successFixture(t, "I could not make the change.")
	e := newTestExecutor(&fakeDoer{resp: jsonResponse(http.StatusOK, body)})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error when the candidate text has no diff")
	}
}

func TestEstimateCostUSD_KnownModelUsesItsRate(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})
	e.model = "gemini-3.1-flash-lite"

	cost, err := e.EstimateCostUSD(context.Background(), &domain.Intent{Text: strings.Repeat("x", 4000)}, nil)
	if err != nil {
		t.Fatalf("EstimateCostUSD failed: %v", err)
	}
	// 4000 chars / 4 chars-per-token = 1000 tokens; gemini-3.1-flash-lite
	// is $0.875 per million tokens => 1000/1_000_000 * 0.875.
	want := 1000.0 / 1_000_000 * 0.875
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

func TestActualCostUSD_UnknownModelFallsBackToDefaultRate(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})
	e.model = "some-future-model"

	got := e.actualCostUSD(usageMetadata{PromptTokenCount: 500_000, CandidatesTokenCount: 500_000})
	if got == nil {
		t.Fatal("actualCostUSD = nil, want a real value")
	}
	want := 1_000_000.0 / 1_000_000 * defaultCostPerMillionTokensUSD
	if *got != want {
		t.Errorf("actualCostUSD = %v, want %v", *got, want)
	}
}

func TestActualCostUSD_NoUsageIsNil(t *testing.T) {
	e := newTestExecutor(&fakeDoer{})

	if got := e.actualCostUSD(usageMetadata{}); got != nil {
		t.Errorf("actualCostUSD(zero usage) = %v, want nil — no Executor should ever fabricate a $0.00 cost it doesn't actually know", *got)
	}
}

func TestExecute_NoUsageInResponseLeavesActualCostNil(t *testing.T) {
	// A response with a candidates array but no usageMetadata object at
	// all (as if a vendor's API omitted it) -- Outcome.ActualCostUSD must
	// stay nil, never silently read as a real $0.00.
	body := successFixture(t, "```diff\n"+sampleDiff+"```\n")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	delete(decoded, "usageMetadata")
	noUsageBody, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	d := &fakeDoer{resp: jsonResponse(http.StatusOK, string(noUsageBody))}
	e := newTestExecutor(d)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "add a comment"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.ActualCostUSD != nil {
		t.Errorf("ActualCostUSD = %v, want nil when the response carries no usageMetadata", *outcome.ActualCostUSD)
	}
}
