// Package cli parses foundry's command-line arguments and drives an Act from
// production through the human trust boundary: it runs the Engine, seeks an
// Authority's approval, and — only once accepted — applies the Outcome and
// records the Act.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"foundry/domain"
	"foundry/engine"
	"foundry/record"
	"foundry/replay"
	"foundry/workspace"
)

// ErrHelp indicates the caller asked for usage information rather than
// requesting work.
var ErrHelp = errors.New("cli: help requested")

// Usage returns the help text for the `do` command.
func Usage() string {
	return `Usage: foundry do "<intent>" --repo <path>

Runs the Act lifecycle for <intent> against the repository at <path> by
invoking Claude Code, shows the proposed patch and verdict, and prompts for
approval. On approval the patch is applied to the repository and the Act is
recorded.

Flags:
  --repo <path>   path to the repository to operate on (required)
  -h, --help      show this help message
`
}

// CLI drives the Act lifecycle across the trust boundary.
type CLI struct {
	engine   *engine.Engine
	recorder record.Recorder
	in       io.Reader
	out      io.Writer
}

// NewCLI wires an Engine, a Recorder, an input source (for approval), and an
// output writer into a CLI.
func NewCLI(eng *engine.Engine, rec record.Recorder, in io.Reader, out io.Writer) *CLI {
	return &CLI{engine: eng, recorder: rec, in: in, out: out}
}

// Do produces an Act for intent, presents it to the Authority, and — only if
// accepted — applies the patch to the repository and records the Act. A
// declined Act is neither applied nor recorded (recording rejections is
// deferred to M0.2).
//
// A Pipeline that declares its own approve/apply/record Steps (RFC-0002 §9
// Phase 4) has already sought approval, applied the patch, and persisted the
// Act inside c.engine.Run, via whatever Authority/Applier/Checkpointer the
// caller gave the Engine (typically an InteractiveAuthority wrapping this
// same package's PromptForApproval, and a workspace.GitApplier) — Do detects
// each of those from the returned Act (hasStepKind) and never repeats the
// work. A Pipeline that declares none of them behaves exactly as before
// RFC-0002 §9 Phase 4 existed: Do prompts, applies, and records itself.
func (c *CLI) Do(ctx context.Context, intent string, repoPath string) error {
	info, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("cli: repo path %q: %w", repoPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cli: repo path %q is not a directory", repoPath)
	}

	act, err := c.engine.Run(ctx, &domain.Intent{Text: intent})
	if err != nil {
		return fmt.Errorf("cli: run: %w", err)
	}

	return c.finalize(ctx, act, repoPath)
}

// checkpointLoader is the one method Resume needs from a
// record.CheckpointStore — a small local interface so cli does not need a
// hard dependency on that concrete type, matching how Replay takes its
// engine.Verifier as a plain parameter instead of a CLI field.
type checkpointLoader interface {
	Load(ctx context.Context, actID string) (*domain.Act, error)
}

// Resume continues the Act checkpointed under actID — left behind by an
// attempt that was interrupted before reaching a terminal Judgment — via
// c.engine.Resume, then drives the same trust boundary Do does: approval,
// apply, and recording, whichever of those a Pipeline's own declared Steps
// did not already handle inside Resume.
func (c *CLI) Resume(ctx context.Context, actID string, checkpoints checkpointLoader, repoPath string) error {
	act, err := checkpoints.Load(ctx, actID)
	if err != nil {
		return fmt.Errorf("cli: resume: %w", err)
	}

	act, err = c.engine.Resume(ctx, act)
	if err != nil {
		return fmt.Errorf("cli: resume: %w", err)
	}

	return c.finalize(ctx, act, repoPath)
}

// finalize drives an Act across the human trust boundary once its Outcome
// and Judgment are produced: seek approval (unless a Pipeline-declared
// approve Step already captured a decision), apply, and record — whichever
// of those a Pipeline's own declared Steps did not already do. Do and
// Resume both call this once c.engine has finished producing act, so the
// trust-boundary logic exists in exactly one place.
func (c *CLI) finalize(ctx context.Context, act *domain.Act, repoPath string) error {
	fmt.Fprintf(c.out, "Act ID:  %s\n", act.ID)
	fmt.Fprintf(c.out, "Repo:    %s\n", repoPath)
	fmt.Fprintf(c.out, "Intent:  %s\n", act.Intent)

	approved := false
	switch {
	case act.ApprovedAt != nil:
		// An approve Step inside the Pipeline already accepted.
		approved = true
	case act.JudgmentVerdict == engine.VerdictRejected:
		// An approve Step inside the Pipeline already declined.
		approved = false
	default:
		// No approve Step ran — this Pipeline still relies on the CLI's
		// own prompt.
		authority, ok, err := PromptForApproval(c.in, c.out, act)
		if err != nil {
			return err
		}
		approved = ok
		if approved {
			now := time.Now()
			act.ApprovedBy = authority
			act.ApprovedAt = &now
		}
	}

	if !approved {
		fmt.Fprintln(c.out, "Declined; nothing was applied or recorded.")
		return nil
	}

	if !hasStepKind(act, domain.StepKindApply) {
		// No apply Step ran inside the Pipeline — this Pipeline still
		// relies on the CLI's own apply, exactly as before RFC-0002 §9
		// Phase 4 existed.
		if err := workspace.ApplyAct(ctx, repoPath, act); err != nil {
			// An approved Act must never vanish without a trace (I8):
			// record it now, with the failure as its verdict and finding.
			// Unlike a Pipeline-declared apply Step (engine/strategy.go),
			// which leaves a checkpoint `foundry resume` can retry from,
			// this fallback path has no other mechanism that would ever
			// record this Act if applying it fails.
			act.JudgmentVerdict = engine.VerdictApplyFailed
			act.CheckedFindings = append(act.CheckedFindings, "apply-failed: "+err.Error())
			if writeErr := c.recorder.Write(ctx, act); writeErr != nil {
				return fmt.Errorf("cli: apply: %w (additionally failed to record: %v)", err, writeErr)
			}
			return fmt.Errorf("cli: apply: %w (recorded as Act %s so the approval is not lost — see `foundry show %s`)", err, act.ID, act.ID)
		}
	}
	if !hasStepKind(act, domain.StepKindRecord) {
		// No record Step ran inside the Pipeline — this Pipeline still
		// relies on the CLI's own recording, exactly as before RFC-0002 §9
		// Phase 4 existed.
		if err := c.recorder.Write(ctx, act); err != nil {
			return fmt.Errorf("cli: record: %w", err)
		}
	}

	fmt.Fprintf(c.out, "Applied and recorded Act %s (approved by %s).\n", act.ID, act.ApprovedBy)
	return nil
}

// Log writes the most recent limit recorded Acts, newest first, one per
// line: ID, creation time, verdict, and Intent.
func (c *CLI) Log(ctx context.Context, limit int) error {
	if limit <= 0 {
		return fmt.Errorf("cli: log limit must be positive, got %d", limit)
	}

	acts, err := c.recorder.List(ctx)
	if err != nil {
		return fmt.Errorf("cli: log: %w", err)
	}
	if len(acts) == 0 {
		fmt.Fprintln(c.out, "No acts recorded.")
		return nil
	}
	if len(acts) > limit {
		acts = acts[len(acts)-limit:]
	}

	for i := len(acts) - 1; i >= 0; i-- {
		act := acts[i]
		fmt.Fprintf(c.out, "%s  %s  %-4s  %s\n",
			act.ID, act.CreatedAt.Format(time.RFC3339), act.JudgmentVerdict, act.Intent)
	}
	return nil
}

// Show writes the full recorded Act identified by actID, pretty-printed —
// piped through $PAGER (maybePage) when it is long and c.out is an
// interactive terminal, so a large patch or findings dump doesn't scroll
// past what a human can read.
func (c *CLI) Show(ctx context.Context, actID string) error {
	act, err := c.recorder.Read(ctx, actID)
	if err != nil {
		return fmt.Errorf("cli: show: %w", err)
	}
	color := colorEnabled(c.out)
	return maybePage(c.out, color, formatAct(act, color))
}

// Replay re-runs verifier against the recorded Act identified by actID —
// never calling an Executor again — and writes whether each verify Step
// reproduces its recorded Judgment. This is read-only, like Show and Log:
// it never mutates the Record, and it is a same-version replay guarantee
// only (replay/replay.go's package doc).
func (c *CLI) Replay(ctx context.Context, actID string, verifier engine.Verifier, workspacePath string) error {
	act, err := c.recorder.Read(ctx, actID)
	if err != nil {
		return fmt.Errorf("cli: replay: %w", err)
	}
	result, err := replay.Verify(ctx, act, verifier, workspacePath)
	if err != nil {
		return fmt.Errorf("cli: replay: %w", err)
	}
	fmt.Fprint(c.out, formatReplayResult(result, colorEnabled(c.out)))
	return nil
}

// formatAct renders a recorded Act for human review: identity, judgment,
// approval, budget usage, the per-Step trace (RFC-0002 §4.5's StepRecord —
// each declared Step's own verdict and duration, not only the flat
// final-round view the fields below it carry), the considered and checked
// Evidence, and the patch as a unified diff — tinted the same way the live
// approval prompt (PromptForApproval) already is when color is true, so
// `foundry show` stops being the one Act-review surface without it. A
// multi-verify Pipeline (e.g. the built-in "review" Pipeline's independent
// verify/verify-again Steps) already narrates each Step's verdict live via
// ProgressReporter; the Steps section is what makes that same trace visible
// again after the fact, from the Record alone. An "Actual cost" line
// appears only when at least one generate Step's Executor reported a real,
// post-execution cost (ADR-0011) — omitted entirely for an Act where none
// did, rather than always showing a permanently-empty line.
func formatAct(act *domain.Act, color bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Act:        %s\n", act.ID)
	fmt.Fprintf(&b, "Created:    %s\n", act.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Intent:     %s\n", act.Intent)
	fmt.Fprintf(&b, "Verdict:    %s\n", renderVerdict(act.JudgmentVerdict, color))
	if act.ApprovedBy != "" && act.ApprovedAt != nil {
		fmt.Fprintf(&b, "Approved:   by %s at %s\n", act.ApprovedBy, act.ApprovedAt.Format(time.RFC3339))
	} else {
		b.WriteString("Approved:   no\n")
	}
	fmt.Fprintf(&b, "Iterations: %d (estimated cost $%.2f)\n", act.Iterations, act.CostEstimateUSD)
	if act.ActualCostUSD != nil {
		reported, total := act.ActualCostCoverage()
		fmt.Fprintf(&b, "Actual cost: $%.4f (reported for %d of %d generate Steps — ADR-0011)\n", *act.ActualCostUSD, reported, total)
	}

	b.WriteString("\nSteps:\n")
	if len(act.Steps) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, step := range act.Steps {
			fmt.Fprintf(&b, "  %-14s (%s)", step.StepID, step.Kind)
			if label := renderStepVerdictLabel(step.Kind, step.JudgmentVerdict, color); label != "" {
				fmt.Fprintf(&b, "  %s", label)
			}
			fmt.Fprintf(&b, "  %s\n", step.FinishedAt.Sub(step.StartedAt))
		}
	}

	b.WriteString("\nConsidered evidence:\n")
	if len(act.ConsideredFiles) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, entry := range act.ConsideredFiles {
			fmt.Fprintf(&b, "  - %s\n", firstLine(entry))
		}
	}

	b.WriteString("\nChecked evidence:\n")
	if len(act.CheckedFindings) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, entry := range act.CheckedFindings {
			for _, line := range strings.Split(strings.TrimRight(entry, "\n"), "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}

	b.WriteString("\nPatch:\n")
	if act.Patch == "" {
		b.WriteString("  (none)\n")
	} else {
		fmt.Fprintf(&b, "%s\n", renderDiff(strings.TrimRight(act.Patch, "\n"), color))
	}
	return b.String()
}

// firstLine returns s up to its first newline. Considered-evidence entries
// carry file contents after a "name:" header line; the header alone keeps
// the listing readable.
func firstLine(s string) string {
	head, _, _ := strings.Cut(s, "\n")
	return head
}

// hasStepKind reports whether act's recorded trace already contains a
// StepRecord of the given kind — used to tell whether an apply (or, later,
// record) Step already ran inside the Pipeline itself, so Do never repeats
// work a Step already did.
func hasStepKind(act *domain.Act, kind string) bool {
	for _, step := range act.Steps {
		if step.Kind == kind {
			return true
		}
	}
	return false
}

// ParseArgs parses arguments for the `do` command: a required positional
// intent and a required --repo flag, in either order. The standard library
// flag package is not used here because it stops parsing flags at the first
// positional argument, which would reject the documented invocation order
// `foundry do "<intent>" --repo <path>`.
func ParseArgs(args []string) (intent string, repoPath string, err error) {
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help" || arg == "-help":
			return "", "", ErrHelp
		case arg == "--repo" || arg == "-repo":
			i++
			if i >= len(args) {
				return "", "", errors.New("cli: --repo requires a value")
			}
			repoPath = args[i]
		case strings.HasPrefix(arg, "--repo="):
			repoPath = strings.TrimPrefix(arg, "--repo=")
		case strings.HasPrefix(arg, "-repo="):
			repoPath = strings.TrimPrefix(arg, "-repo=")
		default:
			positional = append(positional, arg)
		}
	}

	if repoPath == "" {
		return "", "", errors.New("cli: --repo is required")
	}
	if len(positional) != 1 {
		return "", "", errors.New("cli: exactly one intent argument is required")
	}

	return positional[0], repoPath, nil
}
