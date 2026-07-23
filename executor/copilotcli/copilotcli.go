// Package copilotcli implements an Executor that invokes the GitHub
// Copilot CLI (`copilot`, GA since 2026-02-25) as a subprocess and parses a
// unified diff from its output — the same "delegate auth to the vendor's
// own CLI" pattern executor/claude and executor/geminicli already
// establish (PIC-2: Foundry reads no API key of its own; the CLI reads its
// own COPILOT_GITHUB_TOKEN, or reuses an interactive `gh auth login`
// session, entirely outside Foundry).
//
// The Copilot CLI is genuinely agentic — unlike Claude Code and the Gemini
// CLI (both already proven, live, to stay text-only when prompted this
// way), it ships with its own file/shell tools and is documented to
// support broad tool grants (`--allow-all-tools`) for autonomous use. This
// Executor deliberately grants it none: no `--allow-tool`/`--allow-all` flag
// is ever passed, and Execute additionally verifies the workspace's `git
// status --porcelain` is byte-identical before and after the call,
// refusing outright if it isn't. Foundry's whole trust model depends on a
// generate Step only ever *proposing* an Outcome for later approval
// (docs/02-architecture/trust.md, Accountability) — an Executor that can
// mutate the workspace directly, bypassing that gate, would be a real
// violation, not a cosmetic one.
//
// This package is substrate (docs/05-reference/invariants.md I12): it only
// proposes an Outcome. It never applies patches, records Acts, or seeks
// approval — those remain the Engine's and CLI's responsibilities.
package copilotcli

import (
	"bytes"
	"context"
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
	defaultExecutable = "copilot"
	defaultTimeout    = 5 * time.Minute
)

// Executor proposes an Outcome by running the GitHub Copilot CLI in a
// fixed workspace directory and extracting a unified git patch from its
// output.
type Executor struct {
	workspace  string
	model      string
	executable string
	timeout    time.Duration
	runner     runner
}

// NewExecutor returns an Executor that runs the Copilot CLI in workspace.
// An empty model lets the CLI use its own default; the workspace is fixed
// at construction because engine.Executor.Execute does not carry a
// workspace argument, mirroring executor/claude.NewClaudeExecutor and
// executor/geminicli.NewExecutor exactly.
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

// Execute runs the Copilot CLI against the workspace and returns the
// proposed Outcome as a unified git patch. It fails cleanly with a
// descriptive error on a missing executable, a timeout, a non-zero exit,
// unparseable output, or — the one check unique to this Executor — any
// sign the CLI edited the workspace directly instead of only proposing a
// diff.
func (e *Executor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	if e.workspace == "" {
		return nil, errors.New("copilotcli: no workspace configured")
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	before, err := gitStatusPorcelain(ctx, e.workspace)
	if err != nil {
		return nil, fmt.Errorf("copilotcli: check workspace state before running: %w", err)
	}

	prompt := buildPrompt(intent, considered)
	// -s (silent) suppresses session metadata so stdout is clean text to
	// parse; --no-ask-user prevents the CLI from blocking on a clarifying
	// question with nothing running headlessly to answer it. Deliberately
	// no --allow-tool/--allow-all-tools: see the package doc comment for
	// why this Executor never grants either.
	args := []string{"-s", "--no-ask-user"}
	if e.model != "" {
		args = append(args, "--model", e.model)
	}

	// The prompt is piped via stdin, not passed as a -p argument, to avoid
	// any OS command-line length limit (ARG_MAX) with a large
	// gathered-context prompt — the same reasoning executor/geminicli
	// documents for its own identical choice.
	stdout, stderr, err := e.runner.Run(ctx, e.workspace, e.executable, args, prompt)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return nil, fmt.Errorf("copilotcli: executable %q not found in PATH", e.executable)
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("copilotcli: timed out after %s", e.timeout)
		default:
			return nil, executionError(err, stdout, stderr)
		}
	}

	after, err := gitStatusPorcelain(ctx, e.workspace)
	if err != nil {
		return nil, fmt.Errorf("copilotcli: check workspace state after running: %w", err)
	}
	if after != before {
		return nil, fmt.Errorf("copilotcli: the CLI modified the workspace directly instead of only proposing a diff (git status changed from %q to %q) — this Executor grants it no file/shell tools deliberately, and a change here means that restriction did not hold; refusing to proceed rather than silently accepting an already-mutated workspace that bypassed Foundry's own approval gate", before, after)
	}

	patch, err := executor.ParsePatch(stdout)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch}, nil
}

// gitStatusPorcelain returns `git status --porcelain`'s output for dir, the
// same machine-stable format Execute compares before and after invoking
// the CLI to detect a direct workspace mutation.
func gitStatusPorcelain(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status --porcelain: %w", err)
	}
	return string(out), nil
}

// executionError builds a diagnostic error for a failed Copilot CLI
// invocation, mirroring executor/claude's and executor/geminicli's own
// executionError exactly: a non-zero exit with empty stderr is otherwise
// undebuggable.
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
		detail = "\n(no output on stdout or stderr; run `copilot -s --no-ask-user -p \"say ok\"` in the workspace to check the CLI is installed and authenticated)"
	}
	return fmt.Errorf("copilotcli: execution failed: %w%s", err, detail)
}

// buildPrompt assembles the instruction sent to the Copilot CLI: the
// Intent, any gathered context, and a directive to emit only a
// git-apply-compatible unified diff rather than editing files directly —
// the same shape executor/claude.buildPrompt and
// executor/geminicli.buildPrompt already use.
func buildPrompt(intent *domain.Intent, considered []string) string {
	var b strings.Builder
	b.WriteString("Intent:\n")
	b.WriteString(intent.Text)
	b.WriteString("\n\n")

	for i, c := range considered {
		fmt.Fprintf(&b, "Context %d:\n%s\n\n", i+1, c)
	}

	b.WriteString("Respond with only a unified git diff (compatible with `git apply`) ")
	b.WriteString("that implements the Intent. Do not edit any files yourself — only describe the change as a diff.")
	return b.String()
}

// runner runs a subprocess and returns its captured stdout and stderr —
// the seam that lets tests exercise Executor without the real Copilot
// CLI, mirroring executor/claude's and executor/geminicli's own runner
// interface exactly.
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
