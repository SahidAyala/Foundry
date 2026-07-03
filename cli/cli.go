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

// applyPatch applies act's patch to repoPath on an isolated branch named for
// the Act, reusing the workspace package's git-apply mechanism.
func applyPatch(ctx context.Context, repoPath string, act *domain.Act) error {
	ws, err := workspace.NewWorkspace(repoPath, "foundry/act-"+act.ID)
	if err != nil {
		return err
	}
	return ws.Apply(ctx, act.Patch)
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
