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

	fmt.Fprintf(c.out, "Act ID:  %s\n", act.ID)
	fmt.Fprintf(c.out, "Repo:    %s\n", repoPath)
	fmt.Fprintf(c.out, "Intent:  %s\n", act.Intent)

	authority, approved, err := PromptForApproval(c.in, c.out, act)
	if err != nil {
		return err
	}
	if !approved {
		fmt.Fprintln(c.out, "Declined; nothing was applied or recorded.")
		return nil
	}

	now := time.Now()
	act.ApprovedBy = authority
	act.ApprovedAt = &now

	if err := applyPatch(ctx, repoPath, act); err != nil {
		return fmt.Errorf("cli: apply: %w", err)
	}
	if err := c.recorder.Write(ctx, act); err != nil {
		return fmt.Errorf("cli: record: %w", err)
	}

	fmt.Fprintf(c.out, "Applied and recorded Act %s (approved by %s).\n", act.ID, authority)
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

// Show writes the full recorded Act identified by actID, pretty-printed.
func (c *CLI) Show(ctx context.Context, actID string) error {
	act, err := c.recorder.Read(ctx, actID)
	if err != nil {
		return fmt.Errorf("cli: show: %w", err)
	}
	fmt.Fprint(c.out, formatAct(act))
	return nil
}

// formatAct renders a recorded Act for human review: identity, judgment,
// approval, budget usage, the considered and checked Evidence, and the
// patch as a unified diff.
func formatAct(act *domain.Act) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Act:        %s\n", act.ID)
	fmt.Fprintf(&b, "Created:    %s\n", act.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Intent:     %s\n", act.Intent)
	fmt.Fprintf(&b, "Verdict:    %s\n", act.JudgmentVerdict)
	if act.ApprovedBy != "" && act.ApprovedAt != nil {
		fmt.Fprintf(&b, "Approved:   by %s at %s\n", act.ApprovedBy, act.ApprovedAt.Format(time.RFC3339))
	} else {
		b.WriteString("Approved:   no\n")
	}
	fmt.Fprintf(&b, "Iterations: %d (estimated cost $%.2f)\n", act.Iterations, act.CostEstimateUSD)

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
		fmt.Fprintf(&b, "%s\n", strings.TrimRight(act.Patch, "\n"))
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

// applyPatch applies act's patch to repoPath on an isolated branch named for
// the Act, then lands it back on the branch the developer was on: a
// throwaway `foundry/act-<id>` branch must never be left behind for a
// successfully applied Act.
func applyPatch(ctx context.Context, repoPath string, act *domain.Act) error {
	ws, err := workspace.NewWorkspace(repoPath, "foundry/act-"+act.ID)
	if err != nil {
		return err
	}
	if err := ws.Apply(ctx, act.Patch); err != nil {
		return err
	}
	return ws.Land(ctx)
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
