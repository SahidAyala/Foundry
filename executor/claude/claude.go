// Package claude implements an Executor that proposes an Outcome by invoking
// the Claude Code CLI as a subprocess in a workspace and parsing a unified
// diff from its output.
//
// This package is substrate (docs/05-reference/invariants.md I12): it only
// proposes an Outcome. It never applies patches, records Acts, or seeks
// approval — those remain the Engine's and CLI's responsibilities.
//
// Authentication is handled entirely by the Claude Code CLI itself; Foundry
// reads no API key (PIC-2).
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/executor"
)

const (
	defaultExecutable = "claude"
	defaultTimeout    = 5 * time.Minute
)

// ClaudeExecutor proposes an Outcome by running the Claude Code CLI in a
// fixed workspace directory and extracting a unified git patch from its
// output.
type ClaudeExecutor struct {
	workspace  string
	model      string
	executable string
	timeout    time.Duration
	runner     runner
}

// NewClaudeExecutor returns an executor that runs Claude Code in workspace,
// passing model to the CLI's own --model flag when non-empty (confirmed
// against the CLI's own documented flag — code.claude.com/docs/en/headless
// — before adding it, not guessed; accepts either an alias like "sonnet"/
// "opus"/"haiku"/"fable" or a full model name). An empty model omits the
// flag entirely, leaving Claude Code to use whatever its own configured
// default is (a project's `.claude/settings.json`, the `ANTHROPIC_MODEL`
// environment variable, or its own built-in default) — the same behavior
// this package had before model selection existed, so the zero-config
// default Executor (cmd/foundry/main.go's claudeExecutor) is unaffected.
// The workspace is fixed at construction because engine.Executor.Execute does
// not carry a workspace argument; the Engine is wired with the same directory.
func NewClaudeExecutor(workspace, model string) *ClaudeExecutor {
	return &ClaudeExecutor{
		workspace:  workspace,
		model:      model,
		executable: defaultExecutable,
		timeout:    defaultTimeout,
		runner:     execRunner{},
	}
}

var _ engine.Executor = (*ClaudeExecutor)(nil)

// Execute runs Claude Code against the workspace and returns the proposed
// Outcome as a unified git patch. It fails cleanly with a descriptive error
// on a missing executable, a timeout, a non-zero exit, or unparseable output.
func (e *ClaudeExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	if e.workspace == "" {
		return nil, errors.New("claude: no workspace configured")
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	prompt := buildPrompt(intent, considered)
	args := []string{"-p"}
	if e.model != "" {
		args = append(args, "--model", e.model)
	}
	stdout, stderr, err := e.runner.Run(ctx, e.workspace, e.executable, args, prompt)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return nil, fmt.Errorf("claude: executable %q not found in PATH", e.executable)
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("claude: timed out after %s", e.timeout)
		default:
			return nil, executionError(err, stdout, stderr)
		}
	}

	patch, err := parsePatch(stdout)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch}, nil
}

// executionError builds a diagnostic error for a failed Claude Code
// invocation. A non-zero exit with an empty stderr (observed, e.g., when the
// CLI's own environment checks reject the process silently) previously
// surfaced as "execution failed: exit status 1: " with nothing to debug;
// this includes whichever of stdout/stderr carry content, and a concrete
// next step when neither does.
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
		detail = "\n(no output on stdout or stderr; run `claude -p \"say ok\"` in the workspace to check the CLI is installed and authenticated)"
	}
	return fmt.Errorf("claude: execution failed: %w%s", err, detail)
}

// buildPrompt assembles the instruction sent to Claude Code: the Intent, any
// gathered context, and a directive to emit only a git-apply-compatible
// unified diff.
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

// parsePatch extracts a unified diff from Claude Code's output. It
// delegates to executor.ParsePatch — the parsing logic every Executor that
// asks a model to emit a diff shares (executor/claude, executor/openai) —
// so this package carries no logic of its own beyond calling it.
func parsePatch(out string) (string, error) {
	return executor.ParsePatch(out)
}

// runner runs a subprocess and returns its captured stdout and stderr. It is
// the seam that lets tests exercise the executor without Claude Code.
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
