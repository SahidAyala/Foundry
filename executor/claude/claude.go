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
	// defaultTimeout bounds one Execute call. It is deliberately far
	// larger than the per-request deadline an HTTP-API Executor
	// (executor/openai, executor/gemini) uses, because these are not the
	// same kind of call: those send one completion request, while Claude
	// Code is an *agent* — given a prompt it reads the repository with its
	// own tools, often for dozens of turns, before answering.
	//
	// The original 5 minutes was inherited from the HTTP framing and was
	// simply wrong here. Measured against a ~1,300-file project with
	// Foundry's own ~96 KB prompt, a single call to the fastest model took
	// 98 seconds and roughly 30 tool calls; a plan or implement Step on a
	// larger model routinely exceeds five minutes. A deadline below the
	// real distribution does not protect anything — it converts working
	// calls into failures, and each retry pays for the same work again
	// before hitting the same wall.
	//
	// A timeout exists to catch a genuine hang, so it should sit well past
	// where real work finishes, not in the middle of it. Lower it per
	// Executor with timeout_seconds in .foundry/executors.json for a
	// project that wants a tighter leash (an unattended CI run).
	defaultTimeout = 30 * time.Minute
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

	// progress, when non-nil, receives a one-line summary of each event
	// Claude Code emits while it works (see stream.go). Nil — the default
	// — leaves the invocation and the output parsing exactly as they were
	// before streaming existed.
	progress func(string)
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

// SetTimeout overrides how long Execute waits for the Claude Code CLI
// before giving up (defaultTimeout, 5 minutes, otherwise) — a project
// whose own repository or Intent genuinely needs longer per call (a
// large codebase, a multi-file change) sets this via
// project.ExecutorConfig.TimeoutSeconds on a named "claude"-vendor
// entry in .foundry/executors.json, rather than always failing at a
// fixed ceiling with no way to raise it.
func (e *ClaudeExecutor) SetTimeout(d time.Duration) {
	e.timeout = d
}

// SetProgress installs sink as the destination for live narration of what
// Claude Code is doing during a call: the model it resolved, each tool it
// invokes, and its final turn count and cost. Installing a sink switches
// the invocation into the CLI's own line-delimited event mode (stream.go);
// leaving it nil keeps the invocation and output parsing byte-for-byte what
// they were before streaming existed. sink is called from the goroutine
// running Execute, one line at a time, and must not block for long — see
// execRunner.RunStream.
func (e *ClaudeExecutor) SetProgress(sink func(string)) {
	e.progress = sink
}

// Timeout returns the duration Execute currently waits for the Claude
// Code CLI before giving up — the counterpart to SetTimeout, so a
// caller (or a test) can confirm what actually took effect without a
// package-private field to inspect directly.
func (e *ClaudeExecutor) Timeout() time.Duration {
	return e.timeout
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

	// Streaming needs both a sink to narrate to and a runner that supports
	// it; a configured runner that doesn't (any test fake predating
	// stream.go) silently keeps the buffered path rather than losing the
	// call.
	streamer, canStream := e.runner.(streamRunner)
	stream := e.progress != nil && canStream

	var (
		reader         *streamReader
		stdout, stderr string
		err            error
	)
	if stream {
		reader = &streamReader{sink: e.progress}
		args = append(args, streamJSONArgs...)
		stdout, stderr, err = streamer.RunStream(ctx, e.workspace, e.executable, args, prompt, reader.line)
	} else {
		stdout, stderr, err = e.runner.Run(ctx, e.workspace, e.executable, args, prompt)
	}

	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return nil, fmt.Errorf("claude: executable %q not found in PATH", e.executable)
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			return nil, timeoutError(e.timeout, reader, stdout, stderr)
		default:
			return nil, executionError(err, stdout, stderr)
		}
	}

	if stream {
		return e.streamedOutcome(reader, stdout, stderr)
	}

	patch, err := parsePatch(stdout)
	if err != nil {
		return nil, err
	}
	return &domain.Outcome{Patch: patch}, nil
}

// streamedOutcome builds the Outcome from a completed streaming call: the
// patch comes from the CLI's own terminal result event rather than from raw
// stdout, which in this mode holds JSON events rather than the model's
// answer. A stream that ended without a result event, or whose result event
// reports an error, is a failed call — never a silently empty patch.
func (e *ClaudeExecutor) streamedOutcome(reader *streamReader, stdout, stderr string) (*domain.Outcome, error) {
	switch {
	case reader.isError:
		return nil, fmt.Errorf("claude: the CLI reported the run as failed%s", detailOf(reader.result, stderr))
	case !reader.sawResult:
		return nil, fmt.Errorf("claude: the CLI's event stream ended without a result%s", detailOf(lastLines(stdout, maxRecentNarration), stderr))
	}

	patch, err := parsePatch(reader.result)
	if err != nil {
		return nil, err
	}
	// ActualCostUSD is what the CLI itself charged for this call (ADR-0011
	// reported Evidence). It is available only in streaming mode, because
	// only the result event carries it — a non-streaming call leaves it nil,
	// exactly as every call did before.
	return &domain.Outcome{Patch: patch, ActualCostUSD: reader.costUSD}, nil
}

// timeoutError explains a call that hit its deadline. Before this, the
// message was "claude: timed out after 5m0s" and nothing else: whatever the
// CLI had done in those minutes was buffered inside execRunner and thrown
// away, so a run that timed out three times in a row (once per repair
// round) produced no evidence at all about what it had been doing. Now the
// last events narrated during the call — or, for a non-streaming call, the
// tail of whatever it had written — travel with the error.
// The "no output" case is reported differently depending on the mode,
// because it means completely different things. Streaming: events were
// arriving and then stopped, or none ever arrived — real evidence, worth
// acting on. Buffered: `claude -p` writes its answer only when it finishes,
// so a killed process leaves an empty buffer *every* time, no matter how
// healthy the call was. Claiming "the CLI may be waiting on authentication"
// there — which an earlier version of this message did — is a guess dressed
// as a finding, and it sent a real investigation down the wrong path when
// the true cause was simply that Claude Code is agentic: it reads the
// repository with its own tools before answering, which routinely takes
// longer than the 5-minute default on a large project.
func timeoutError(timeout time.Duration, reader *streamReader, stdout, stderr string) error {
	var last string
	streaming := reader != nil
	if streaming {
		last = strings.Join(reader.recent, "\n")
	} else {
		last = lastLines(stdout, maxRecentNarration)
	}

	if strings.TrimSpace(last) == "" && strings.TrimSpace(stderr) == "" {
		if streaming {
			return fmt.Errorf("claude: timed out after %s without emitting a single event "+
				"(the CLI never started work — check it is installed and authenticated with "+
				"`claude -p \"say ok\"` in the workspace)", timeout)
		}
		return fmt.Errorf("claude: timed out after %s, still running "+
			"(no partial output is possible in this mode — `claude -p` writes its answer only when it finishes, "+
			"and Claude Code explores the repository with its own tools first, which often exceeds this deadline: "+
			"raise timeout_seconds in .foundry/executors.json, or set FOUNDRY_VERBOSE=1 to watch what it is doing)", timeout)
	}
	return fmt.Errorf("claude: timed out after %s%s", timeout, detailOf(last, stderr))
}

// detailOf renders whichever of a call's output and stderr carry content,
// as indented trailing detail on an error message.
func detailOf(out, stderr string) string {
	out, stderr = strings.TrimSpace(out), strings.TrimSpace(stderr)
	var b strings.Builder
	if out != "" {
		b.WriteString("\nlast output:\n" + indent(out))
	}
	if stderr != "" {
		b.WriteString("\nstderr:\n" + indent(stderr))
	}
	return b.String()
}

// lastLines returns at most n trailing non-empty lines of s.
func lastLines(s string, n int) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return strings.Join(kept, "\n")
}

// indent prefixes every line of s with two spaces, so multi-line detail
// reads as attached to the error rather than as more error messages.
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
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
