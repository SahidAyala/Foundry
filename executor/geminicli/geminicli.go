// Package geminicli implements an Executor that invokes the Gemini CLI
// (google-gemini/gemini-cli) as a subprocess and parses a unified diff from
// its JSON output.
//
// Unlike executor/gemini (a pure HTTP call against Gemini's REST API, which
// needs a GEMINI_API_KEY), this package delegates authentication entirely
// to the Gemini CLI itself — the same pattern executor/claude already
// established for Claude Code (PIC-2: Foundry reads no API key). A user
// runs `gemini` once, interactively, and picks "Sign in with Google"; the
// CLI caches that login on disk and reuses it for every later headless
// invocation (confirmed against the CLI's own docs,
// geminicli.com/docs/get-started/authentication and
// geminicli.com/docs/cli/headless).
//
// This is the recommended way to configure a Gemini Executor — per the
// maintainer's own framing, a raw API key is a last resort, not the
// default. executor/gemini remains available for environments where no
// browser is ever available to complete that one-time login (some CI
// runners) and a project explicitly opts into it.
//
// This package is substrate (docs/05-reference/invariants.md I12): it only
// proposes an Outcome. It never applies patches, records Acts, or seeks
// approval — those remain the Engine's and CLI's responsibilities.
package geminicli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

const (
	defaultExecutable = "gemini"
	defaultTimeout    = 5 * time.Minute
)

// Executor proposes an Outcome by running the Gemini CLI in a fixed
// workspace directory and extracting a unified git patch from its output.
type Executor struct {
	workspace  string
	model      string
	executable string
	timeout    time.Duration
	runner     runner
}

// NewExecutor returns an Executor that runs the Gemini CLI in workspace. An
// empty model lets the CLI use its own default; the workspace is fixed at
// construction because engine.Executor.Execute does not carry a workspace
// argument, mirroring executor/claude.NewClaudeExecutor exactly.
func NewExecutor(workspace, model string) *Executor {
	return &Executor{
		workspace:  workspace,
		model:      model,
		executable: defaultExecutable,
		timeout:    defaultTimeout,
		runner:     execRunner{},
	}
}

var _ engine.Executor = (*Executor)(nil)

// headlessResponse is the Gemini CLI's documented --output-format json
// shape (geminicli.com/docs/cli/headless): the model's final answer, an
// optional error, and a stats object this Executor has no use for and
// therefore does not model.
type headlessResponse struct {
	Response string `json:"response"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Execute runs the Gemini CLI against the workspace and returns the
// proposed Outcome as a unified git patch. It fails cleanly with a
// descriptive error on a missing executable, a timeout, a non-zero exit,
// a CLI-reported error, or unparseable output.
func (e *Executor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	if e.workspace == "" {
		return nil, errors.New("geminicli: no workspace configured")
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	prompt := buildPrompt(intent, considered)
	args := []string{"--output-format", "json"}
	if e.model != "" {
		args = append(args, "-m", e.model)
	}

	// The prompt is piped via stdin (not passed as a -p argument) so a
	// large gathered-context prompt never risks the OS's command-line
	// length limit (ARG_MAX) — confirmed against the CLI's own docs that
	// piped, non-TTY stdin alone already triggers headless mode, with no
	// -p needed at all.
	stdout, stderr, err := e.runner.Run(ctx, e.workspace, e.executable, args, prompt)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return nil, fmt.Errorf("geminicli: executable %q not found in PATH", e.executable)
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("geminicli: timed out after %s", e.timeout)
		default:
			return nil, executionError(err, stdout, stderr)
		}
	}

	resp, err := decodeHeadlessResponse(stdout)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("geminicli: %s", resp.Error.Message)
	}
	if strings.TrimSpace(resp.Response) == "" {
		return nil, errors.New("geminicli: empty response")
	}

	patch, err := executor.ParsePatch(resp.Response)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch}, nil
}

// decodeHeadlessResponse parses stdout as headlessResponse's documented
// JSON shape. A direct decode is tried first; if it fails, this falls back
// to extracting the substring from the first '{' to the last '}' and
// retrying, since a cached-credential login can print a diagnostic line
// (e.g. "Loaded cached credentials.") to stdout ahead of the JSON blob —
// unconfirmed by the CLI's own docs for JSON mode specifically, but cheap
// to tolerate defensively rather than fail outright on it.
func decodeHeadlessResponse(stdout string) (headlessResponse, error) {
	var resp headlessResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
		return resp, nil
	}

	start := strings.IndexByte(stdout, '{')
	end := strings.LastIndexByte(stdout, '}')
	if start == -1 || end == -1 || end < start {
		return headlessResponse{}, fmt.Errorf("geminicli: no JSON output found: %s", strings.TrimSpace(stdout))
	}
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &resp); err != nil {
		return headlessResponse{}, fmt.Errorf("geminicli: decode JSON output: %w (raw output: %s)", err, strings.TrimSpace(stdout))
	}
	return resp, nil
}

// executionError builds a diagnostic error for a failed Gemini CLI
// invocation, mirroring executor/claude's own executionError exactly: a
// non-zero exit with empty stderr is otherwise undebuggable.
func executionError(err error, stdout, stderr string) error {
	stdout, stderr = strings.TrimSpace(stdout), strings.TrimSpace(stderr)

	var detail string
	switch {
	case stderr != "" && stdout != "":
		detail = fmt.Sprintf("\nstderr: %s\nstdout: %s", stderr, stdout)
	case stderr != "":
		detail = "\nstderr: " + stderr
	case stdout != "":
		detail = "\nstdout: " + stdout
	default:
		detail = "\n(no output on stdout or stderr; run `gemini -p \"say ok\"` in the workspace to check the CLI is installed and authenticated)"
	}
	return fmt.Errorf("geminicli: execution failed: %w%s", err, detail)
}

// buildPrompt assembles the instruction sent to the Gemini CLI: the
// Intent, any gathered context, and a directive to emit only a
// git-apply-compatible unified diff — the same shape
// executor/claude.buildPrompt and executor/gemini.buildUserContent already
// use.
func buildPrompt(intent *domain.Intent, considered []string) string {
	var b strings.Builder
	b.WriteString("Intent:\n")
	b.WriteString(intent.Text)
	b.WriteString("\n\n")

	for i, c := range considered {
		fmt.Fprintf(&b, "Context %d:\n%s\n\n", i+1, c)
	}

	b.WriteString("Respond with only a unified git diff (compatible with `git apply`) ")
	b.WriteString("that implements the Intent. Do not include any prose or explanation.")
	return b.String()
}

// runner runs a subprocess and returns its captured stdout and stderr —
// the seam that lets tests exercise Executor without the real Gemini CLI,
// mirroring executor/claude's own runner interface exactly.
type runner interface {
	Run(ctx context.Context, dir, name string, args []string, stdin string) (stdout, stderr string, err error)
}

// execRunner is the production runner: it invokes the real subprocess.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir, name string, args []string, stdin string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
