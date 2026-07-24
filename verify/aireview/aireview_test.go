package aireview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

// fakeDoer is an injectable doer that returns a canned response and
// captures the request it received — mirrors executor/openai's own test
// double.
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

func newTestVerifier(d doer) *Verifier {
	return &Verifier{
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

func fixtureResponse(t *testing.T, content string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"role": "assistant", "content": content}},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(body)
}

func TestVerify_PassWithNoFindings(t *testing.T) {
	body := fixtureResponse(t, "VERDICT: pass\nFINDINGS:\n- none")
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	v := newTestVerifier(d)

	judgment, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff --git a/x b/x\n"}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	if len(judgment.Checked) != 1 || judgment.Checked[0] != "ai-review: pass" {
		t.Errorf("Checked = %v, want a single ai-review: pass entry", judgment.Checked)
	}

	if !strings.Contains(d.gotBody, "diff --git a/x b/x") {
		t.Errorf("request body missing the patch to review, got:\n%s", d.gotBody)
	}
	if d.gotReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want %q", d.gotReq.Header.Get("Authorization"), "Bearer test-key")
	}
}

func TestVerify_PassWithFindingsSurfacesThemWithoutFailing(t *testing.T) {
	body := fixtureResponse(t, "VERDICT: pass\nFINDINGS:\n- Consider adding a comment here\n- Variable name could be clearer")
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	v := newTestVerifier(d)

	judgment, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q (findings alone must not fail the review)", judgment.Verdict, "pass")
	}
	if len(judgment.Checked) != 3 {
		t.Fatalf("Checked = %v, want 3 entries (the pass marker plus 2 findings)", judgment.Checked)
	}
	if !strings.Contains(judgment.Checked[1], "Consider adding a comment") {
		t.Errorf("Checked[1] = %q, want the first finding", judgment.Checked[1])
	}
}

func TestVerify_Fail(t *testing.T) {
	body := fixtureResponse(t, "VERDICT: fail\nFINDINGS:\n- Possible nil pointer dereference on line 42\n- Missing error handling for the file read")
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	v := newTestVerifier(d)

	judgment, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "fail")
	}
	if len(judgment.Checked) == 0 {
		t.Fatal("Checked is empty, want the failure findings")
	}
}

func TestVerify_MalformedResponseIsTreatedAsFail(t *testing.T) {
	body := fixtureResponse(t, "I looked at the diff and it seems mostly fine, though I have some concerns.")
	d := &fakeDoer{resp: jsonResponse(http.StatusOK, body)}
	v := newTestVerifier(d)

	judgment, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "workspace")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q for an unparseable response — must never look like a clean pass", judgment.Verdict, "fail")
	}
	if !strings.Contains(judgment.Checked[0], "seems mostly fine") {
		t.Errorf("Checked = %v, want the raw unparseable response preserved for debugging", judgment.Checked)
	}
}

func TestVerify_TransportError(t *testing.T) {
	v := newTestVerifier(&fakeDoer{err: context.DeadlineExceeded})

	_, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "workspace")
	if err == nil {
		t.Fatal("Verify returned nil error on transport failure")
	}
}

func TestVerify_Unauthorized(t *testing.T) {
	body := `{"error": {"message": "Incorrect API key provided"}}`
	v := newTestVerifier(&fakeDoer{resp: jsonResponse(http.StatusUnauthorized, body)})

	_, err := v.Verify(context.Background(), &domain.Outcome{Patch: "diff"}, "workspace")
	if err == nil {
		t.Fatal("Verify returned nil error on 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want it to mention 'authentication failed'", err)
	}
}

func TestParseReview_CaseInsensitiveVerdictLabel(t *testing.T) {
	judgment := parseReview("Verdict: PASS\nFindings:\n- none")
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q (verdict parsing must be case-insensitive)", judgment.Verdict, "pass")
	}
}

func TestParseReview_EmptyContentIsFail(t *testing.T) {
	judgment := parseReview("")
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q for empty content", judgment.Verdict, "fail")
	}
}
