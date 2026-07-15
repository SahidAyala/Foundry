// Package openai implements an Executor that proposes an Outcome by calling
// OpenAI's Chat Completions API and parsing a unified diff from the
// response.
//
// Unlike executor/claude (a subprocess wrapping the Claude Code CLI), this
// is a pure HTTP API call — no local binary dependency, and a different
// failure taxonomy (HTTP status codes and rate limits, not process exit
// codes). This is the invocation-shape difference
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md Decision
// 2 names explicitly and rules must stay entirely adapter-internal: nothing
// in engine, engine.Strategy, or engine.Router assumes either shape.
//
// This package is substrate (docs/05-reference/invariants.md I12): it only
// proposes an Outcome. It never applies patches, records Acts, or seeks
// approval — those remain the Engine's and CLI's responsibilities.
//
// Foundry never persists or logs the API key passed to NewExecutor; per
// ADR-0005 Decision 5, resolving it from the environment variable a
// project's ".foundry/executors.json" names is the caller's responsibility
// (mirroring how executor/claude reads its own credential from its own
// environment, unmanaged by Foundry).
package openai

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
	defaultEndpoint = "https://api.openai.com/v1/chat/completions"
	defaultTimeout  = 5 * time.Minute

	// charsPerTokenEstimate is a rough, well-known approximation (English
	// prose averages ~4 characters per token) used only to produce a
	// pre-execution cost estimate (EstimateCostUSD) — never to decide
	// what to send the API, which sends its own exact request regardless.
	charsPerTokenEstimate = 4

	// defaultCostPerMillionTokensUSD is the blended (prompt+completion,
	// undifferentiated) rate EstimateCostUSD falls back to for a model
	// not listed in costPerMillionTokensUSD.
	defaultCostPerMillionTokensUSD = 5.00
)

// costPerMillionTokensUSD is a small, static, blended per-model price
// table used only for EstimateCostUSD's rough pre-execution estimate. It
// is not billing data — OpenAI's own usage dashboard is the source of
// truth for actual spend.
var costPerMillionTokensUSD = map[string]float64{
	"gpt-5.1":      5.00,
	"gpt-5.1-mini": 1.00,
}

// systemPrompt instructs the model to emit only a unified diff, mirroring
// the same instruction executor/claude's buildPrompt gives Claude Code.
const systemPrompt = "You are a code-generation Executor. Respond with only a unified git diff " +
	"(compatible with `git apply`) that implements the Intent. Do not include any prose or explanation."

// Executor proposes an Outcome by calling OpenAI's Chat Completions API and
// extracting a unified git patch from the response.
type Executor struct {
	model    string
	apiKey   string
	endpoint string
	timeout  time.Duration
	doer     doer
}

// NewExecutor returns an Executor that calls model via OpenAI's Chat
// Completions API, authenticating with apiKey. Per ADR-0005 Decision 5,
// resolving apiKey from the environment variable a project's
// ExecutorConfig names (project.ExecutorConfig.APIKeyEnv) is the caller's
// responsibility — this constructor accepts the resolved credential
// directly and never reads the environment itself, keeping this package
// decoupled from the project package exactly as executor/claude is.
func NewExecutor(model, apiKey string) *Executor {
	return &Executor{
		model:    model,
		apiKey:   apiKey,
		endpoint: defaultEndpoint,
		timeout:  defaultTimeout,
		doer:     http.DefaultClient,
	}
}

var _ engine.Executor = (*Executor)(nil)
var _ engine.CostEstimator = (*Executor)(nil)

// doer sends an HTTP request and returns its response. It is the seam that
// lets tests exercise Executor without a real network call — mirroring
// executor/claude's runner seam for its subprocess.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// chatMessage is one message in a Chat Completions request or response,
// per OpenAI's documented API shape.
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
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Execute calls OpenAI's Chat Completions API and returns the proposed
// Outcome as a unified git patch. It fails cleanly with a descriptive error
// on a transport failure, a timeout, a non-2xx response, or unparseable
// output.
func (e *Executor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	content, err := e.call(ctx, intent, considered)
	if err != nil {
		return nil, err
	}

	patch, err := executor.ParsePatch(content)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch}, nil
}

// EstimateCostUSD returns a rough, pre-execution cost estimate for calling
// Execute with intent and considered, satisfying engine.CostEstimator
// (ADR-0005 Decision 3). It is deliberately approximate: a
// characters-per-token heuristic (charsPerTokenEstimate) over a blended,
// undifferentiated per-model rate (costPerMillionTokensUSD) — not an exact
// accounting, and not derived from any actual API usage response.
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

// call sends intent and considered to the Chat Completions API and returns
// the first choice's message content.
func (e *Executor) call(ctx context.Context, intent *domain.Intent, considered []string) (string, error) {
	body, err := json.Marshal(chatCompletionRequest{
		Model: e.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: buildUserContent(intent, considered)},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.doer.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("openai: timed out after %s", e.timeout)
		}
		return "", fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp.StatusCode, respBody)
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("openai: decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("openai: response contained no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// statusError renders a diagnostic error for a non-2xx response, preferring
// the vendor's own documented {"error": {"message": ...}} body when it
// parses, and naming the specific, common failure modes (auth, rate limit)
// a caller is most likely to hit.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var decoded chatCompletionErrorResponse
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		message = decoded.Error.Message
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("openai: authentication failed (status %d): %s", status, message)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("openai: rate limited (status %d): %s", status, message)
	case status >= 500:
		return fmt.Errorf("openai: server error (status %d): %s", status, message)
	default:
		return fmt.Errorf("openai: request rejected (status %d): %s", status, message)
	}
}

// buildUserContent assembles the user-role message content: the Intent and
// any gathered context, mirroring executor/claude's buildPrompt shape
// (Intent, then each considered entry numbered) but split so the model's
// behavioral instruction lives in the system-role message instead, per the
// Chat Completions API's role-based shape.
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
