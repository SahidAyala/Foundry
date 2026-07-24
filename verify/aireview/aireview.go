// Package aireview implements engine.Verifier by asking a model to review
// an Outcome's proposed patch and report a verdict — a non-deterministic
// verify Step, meant to be composed alongside a deterministic
// verify.Gate (via verify.Compose), never used alone. Foundry's trust
// model (docs/02-architecture/trust.md) states a preference for
// deterministic checks (compilers, type-checkers, tests) but does not
// exclude model judgment as supplementary Evidence — this is that
// supplementary layer, not a replacement for the deterministic one.
//
// Unlike a generate Step's Executor (executor/claude, executor/openai,
// ...), which every concrete implementation prompts to emit only a
// unified diff, this package asks a genuinely different question ("does
// this diff look right?") and expects a genuinely different answer shape
// (a verdict plus findings, not a patch) — reusing an Executor directly
// for this would fight its own hardcoded prompt. This package therefore
// makes its own direct HTTP call, in the same OpenAI-Chat-Completions-
// compatible shape executor/openai and the "openai-compatible" Executor
// vendor already establish, so the same review Verifier works against
// OpenAI, Gemini's API, Ollama, Groq, DeepSeek, or any other endpoint
// speaking that shape — a project names one via NewVerifier's endpoint
// parameter, mirroring project.ExecutorConfig.BaseURL's own pattern.
//
// A verdict this package cannot confidently parse as "pass" is treated
// as "fail", not silently passed — an ambiguous model response must
// never look identical to a clean bill of health.
package aireview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"foundry/domain"
	"foundry/engine"
)

const defaultTimeout = 2 * time.Minute

// systemPrompt instructs the model to review a diff and respond in a
// fixed, easily-parsed shape — never a diff of its own, never free-form
// prose.
const systemPrompt = `You are a code reviewer. You will be shown a proposed unified diff and asked whether it should be accepted.
Respond in exactly this format and nothing else:

VERDICT: pass
FINDINGS:
- (one finding per line, or "none" if there are no findings)

VERDICT must be exactly "pass" or "fail". Use "fail" for anything you would ask a human engineer to fix before merging — bugs, missing error handling, security issues, or anything unclear enough that you cannot confirm it is correct.`

// Verifier asks a model to review an Outcome's Patch and reports a
// domain.Judgment. It satisfies engine.Verifier; workspace is accepted
// only to match that interface and is never used — a diff review needs
// no filesystem access.
type Verifier struct {
	model    string
	apiKey   string
	endpoint string
	timeout  time.Duration
	doer     doer
}

// NewVerifier returns a Verifier that calls model at endpoint (an
// OpenAI-Chat-Completions-compatible endpoint — see the package doc),
// authenticating with apiKey. apiKey may be empty for an endpoint with no
// auth of its own (e.g. a local Ollama instance), mirroring
// executor/openai.NewExecutorWithEndpoint's own handling.
func NewVerifier(model, apiKey, endpoint string) *Verifier {
	return &Verifier{
		model:    model,
		apiKey:   apiKey,
		endpoint: endpoint,
		timeout:  defaultTimeout,
		doer:     http.DefaultClient,
	}
}

var _ engine.Verifier = (*Verifier)(nil)

// doer sends an HTTP request and returns its response — the same seam
// executor/openai and executor/gemini already use to let tests exercise
// Verifier without a real network call.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type chatCompletionErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Verify sends outcome.Patch to the configured model and parses its
// response into a Judgment. A patch the model reports no findings
// against still yields Verdict "pass" with a single explanatory Checked
// entry, never an empty slice that could be confused with "not run at
// all".
func (v *Verifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	content, err := v.call(ctx, outcome.Patch)
	if err != nil {
		return nil, err
	}
	return parseReview(content), nil
}

func (v *Verifier) call(ctx context.Context, patch string) (string, error) {
	body, err := json.Marshal(chatCompletionRequest{
		Model: v.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Review this diff:\n\n" + patch},
		},
	})
	if err != nil {
		return "", fmt.Errorf("aireview: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("aireview: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.doer.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("aireview: timed out after %s", v.timeout)
		}
		return "", fmt.Errorf("aireview: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aireview: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp.StatusCode, respBody)
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("aireview: decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("aireview: response contained no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// statusError renders a diagnostic error for a non-2xx response,
// mirroring executor/openai's own statusError exactly.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var decoded chatCompletionErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		message = decoded.Error.Message
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("aireview: authentication failed (status %d): %s", status, message)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("aireview: rate limited (status %d): %s", status, message)
	case status >= 500:
		return fmt.Errorf("aireview: server error (status %d): %s", status, message)
	default:
		return fmt.Errorf("aireview: request rejected (status %d): %s", status, message)
	}
}

// parseReview extracts a Judgment from the model's response, expecting
// systemPrompt's own documented "VERDICT: .../FINDINGS: ..." shape. A
// response with no parseable "VERDICT: pass" line — a malformed reply, a
// model that ignored the instruction, empty output — is treated as
// "fail" with the raw response preserved as the one Checked entry: an
// unparseable review must never look identical to a clean pass.
func parseReview(content string) *domain.Judgment {
	lines := strings.Split(content, "\n")

	verdict := ""
	findingsStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case verdict == "" && strings.HasPrefix(strings.ToUpper(trimmed), "VERDICT:"):
			verdict = strings.ToLower(strings.TrimSpace(trimmed[len("VERDICT:"):]))
		case findingsStart == -1 && strings.HasPrefix(strings.ToUpper(trimmed), "FINDINGS:"):
			findingsStart = i + 1
		}
	}

	if verdict != "pass" {
		return &domain.Judgment{
			Verdict: "fail",
			Checked: []string{"ai-review: fail\n" + strings.TrimSpace(content)},
		}
	}

	var findings []string
	if findingsStart >= 0 {
		for _, line := range lines[findingsStart:] {
			f := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
			f = strings.TrimSpace(f)
			if f == "" || strings.EqualFold(f, "none") || strings.EqualFold(f, "(none)") {
				continue
			}
			findings = append(findings, f)
		}
	}

	if len(findings) == 0 {
		return &domain.Judgment{Verdict: "pass", Checked: []string{"ai-review: pass"}}
	}
	// The model said "pass" but still listed findings — surface them as
	// reported Evidence without failing the Act over them; a human
	// reviewing the recorded Act (foundry show) sees them either way.
	return &domain.Judgment{Verdict: "pass", Checked: append([]string{"ai-review: pass"}, findings...)}
}
