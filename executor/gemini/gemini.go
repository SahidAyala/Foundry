// Package gemini implements an Executor that proposes an Outcome by calling
// Google's Gemini API (the generateContent endpoint) and parsing a unified
// diff from the response.
//
// Like executor/openai (and unlike executor/claude's subprocess), this is a
// pure HTTP API call — no local binary dependency, and the same HTTP-status/
// rate-limit failure taxonomy openai's own adapter already established. This
// is exactly the invocation-shape difference
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md Decision
// 2 names and requires to stay entirely adapter-internal: nothing in
// engine, engine.Strategy, or engine.Router assumes either shape, so adding
// a third vendor here needs no new architectural decision — ADR-0005's
// contract and ADR-0006's explicit-pin routing already cover it.
//
// This package is substrate (docs/05-reference/invariants.md I12): it only
// proposes an Outcome. It never applies patches, records Acts, or seeks
// approval — those remain the Engine's and CLI's responsibilities.
//
// Foundry never persists or logs the API key passed to NewExecutor; per
// ADR-0005 Decision 5, resolving it from the environment variable a
// project's ".foundry/executors.json" names is the caller's responsibility,
// mirroring executor/openai.
package gemini

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
	"foundry/executor"
)

const (
	defaultEndpointTemplate = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	defaultTimeout          = 5 * time.Minute

	// charsPerTokenEstimate is the same rough, well-known approximation
	// executor/openai uses (English prose averages ~4 characters per
	// token) — used only for a pre-execution cost estimate
	// (EstimateCostUSD), never to decide what to send the API.
	charsPerTokenEstimate = 4

	// defaultCostPerMillionTokensUSD is the blended rate EstimateCostUSD
	// falls back to for a model not listed in costPerMillionTokensUSD.
	defaultCostPerMillionTokensUSD = 5.00
)

// costPerMillionTokensUSD is a small, static, blended (input+output,
// simple-averaged) per-model price table used only for EstimateCostUSD's
// rough pre-execution estimate — not billing data. Google's own Gemini API
// pricing page (ai.google.dev/gemini-api/docs/pricing) is the source of
// truth for actual spend; prices there are quoted separately for input and
// output, so the values here are a simple average of the two, the same
// undifferentiated blending executor/openai's own table already uses.
var costPerMillionTokensUSD = map[string]float64{
	"gemini-3.6-flash":      4.50,
	"gemini-3.5-flash":      5.25,
	"gemini-3.5-flash-lite": 1.40,
	"gemini-3.1-flash-lite": 0.875,
	"gemini-3.1-pro":        7.00,
}

// systemInstruction mirrors executor/openai's own systemPrompt: instruct
// the model to emit only a unified diff.
const systemInstruction = "You are a code-generation Executor. Respond with only a unified git diff " +
	"(compatible with `git apply`) that implements the Intent. Do not include any prose or explanation."

// Executor proposes an Outcome by calling Gemini's generateContent API and
// extracting a unified git patch from the response.
type Executor struct {
	model    string
	apiKey   string
	endpoint string
	timeout  time.Duration
	doer     doer
}

// NewExecutor returns an Executor that calls model via Gemini's
// generateContent API, authenticating with apiKey. Per ADR-0005 Decision 5,
// resolving apiKey from the environment variable a project's
// ExecutorConfig names (project.ExecutorConfig.APIKeyEnv) is the caller's
// responsibility — this constructor accepts the resolved credential
// directly and never reads the environment itself, mirroring
// executor/openai exactly.
func NewExecutor(model, apiKey string) *Executor {
	return &Executor{
		model:    model,
		apiKey:   apiKey,
		endpoint: fmt.Sprintf(defaultEndpointTemplate, model),
		timeout:  defaultTimeout,
		doer:     http.DefaultClient,
	}
}

var _ engine.Executor = (*Executor)(nil)
var _ engine.CostEstimator = (*Executor)(nil)

// doer sends an HTTP request and returns its response — the same seam
// executor/openai uses to let tests exercise Executor without a real
// network call.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// part is one piece of a Gemini content turn — only a text part is ever
// sent or expected back, per this Executor's own systemInstruction.
type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type generateContentRequest struct {
	Contents          []content `json:"contents"`
	SystemInstruction *content  `json:"systemInstruction,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
	// UsageMetadata is Gemini's own real, post-execution token accounting
	// for this call. Execute uses it to populate
	// domain.Outcome.ActualCostUSD (ADR-0011), mirroring
	// executor/openai's own Usage handling.
	UsageMetadata usageMetadata `json:"usageMetadata"`
}

// usageMetadata is Gemini's documented per-call token accounting.
type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

// googleErrorResponse is the standard Google API error envelope
// (google.rpc.Status-shaped), used across Google's REST APIs generally,
// not just Gemini's.
type googleErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Execute calls Gemini's generateContent API and returns the proposed
// Outcome as a unified git patch. It fails cleanly with a descriptive error
// on a transport failure, a timeout, a non-2xx response, or unparseable
// output.
func (e *Executor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	decoded, err := e.call(ctx, intent, considered)
	if err != nil {
		return nil, err
	}

	text, err := responseText(decoded)
	if err != nil {
		return nil, err
	}
	patch, err := executor.ParsePatch(text)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch, ActualCostUSD: e.actualCostUSD(decoded.UsageMetadata)}, nil
}

// responseText extracts the first candidate's text, failing descriptively
// if Gemini returned no candidates at all (e.g. a prompt blocked entirely
// by promptFeedback, which carries no candidate to read from).
func responseText(resp generateContentResponse) (string, error) {
	if len(resp.Candidates) == 0 {
		return "", errors.New("gemini: response contained no candidates")
	}
	var b strings.Builder
	for _, p := range resp.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return b.String(), nil
}

// actualCostUSD computes the real, post-execution cost of a call from
// Gemini's own reported token usage (ADR-0011), using the same per-model
// price table EstimateCostUSD's pre-execution heuristic already reads —
// nil if usage carries no tokens at all (a malformed or test response with
// no real signal), so a caller never mistakes "we don't know" for "this
// call cost $0.00."
func (e *Executor) actualCostUSD(usage usageMetadata) *float64 {
	tokens := usage.PromptTokenCount + usage.CandidatesTokenCount
	if tokens == 0 {
		return nil
	}
	rate, ok := costPerMillionTokensUSD[e.model]
	if !ok {
		rate = defaultCostPerMillionTokensUSD
	}
	cost := float64(tokens) / 1_000_000 * rate
	return &cost
}

// EstimateCostUSD returns a rough, pre-execution cost estimate for calling
// Execute with intent and considered, satisfying engine.CostEstimator
// (ADR-0005 Decision 3) — the same characters-per-token heuristic over a
// blended per-model rate executor/openai's own EstimateCostUSD uses.
func (e *Executor) EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error) {
	chars := len(intent.Text)
	for _, c := range considered {
		chars += len(c)
	}
	tokens := chars / charsPerTokenEstimate

	rate, ok := costPerMillionTokensUSD[e.model]
	if !ok {
		rate = defaultCostPerMillionTokensUSD
	}
	return float64(tokens) / 1_000_000 * rate, nil
}

// call sends intent and considered to the generateContent API and returns
// the decoded response.
func (e *Executor) call(ctx context.Context, intent *domain.Intent, considered []string) (generateContentResponse, error) {
	body, err := json.Marshal(generateContentRequest{
		Contents:          []content{{Role: "user", Parts: []part{{Text: buildUserContent(intent, considered)}}}},
		SystemInstruction: &content{Parts: []part{{Text: systemInstruction}}},
	})
	if err != nil {
		return generateContentResponse{}, fmt.Errorf("gemini: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return generateContentResponse{}, fmt.Errorf("gemini: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// x-goog-api-key keeps the credential out of the URL (and therefore out
	// of any request-line logging) — Google's documented alternative to
	// the ?key= query parameter.
	req.Header.Set("x-goog-api-key", e.apiKey)

	resp, err := e.doer.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return generateContentResponse{}, fmt.Errorf("gemini: timed out after %s", e.timeout)
		}
		return generateContentResponse{}, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return generateContentResponse{}, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return generateContentResponse{}, statusError(resp.StatusCode, respBody)
	}

	var decoded generateContentResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return generateContentResponse{}, fmt.Errorf("gemini: decode response: %w", err)
	}
	return decoded, nil
}

// statusError renders a diagnostic error for a non-2xx response, preferring
// Google's own documented {"error": {"message": ...}} body when it parses,
// and naming the specific, common failure modes (auth, rate limit) a caller
// is most likely to hit.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var decoded googleErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		message = decoded.Error.Message
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("gemini: authentication failed (status %d): %s", status, message)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("gemini: rate limited (status %d): %s", status, message)
	case status >= 500:
		return fmt.Errorf("gemini: server error (status %d): %s", status, message)
	default:
		return fmt.Errorf("gemini: request rejected (status %d): %s", status, message)
	}
}

// buildUserContent assembles the user-turn text: the Intent and any
// gathered context, mirroring executor/openai's own buildUserContent shape
// exactly (Gemini's systemInstruction plays the same role as OpenAI's
// system-role message, so the user turn only needs Intent + Context).
func buildUserContent(intent *domain.Intent, considered []string) string {
	var b strings.Builder
	b.WriteString("Intent:\n")
	b.WriteString(intent.Text)
	b.WriteString("\n\n")
	for i, c := range considered {
		fmt.Fprintf(&b, "Context %d:\n%s\n\n", i+1, c)
	}
	return b.String()
}
