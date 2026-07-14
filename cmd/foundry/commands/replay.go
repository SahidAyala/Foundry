package commands

import (
	"context"
	"errors"
	"fmt"
	"io"

	"foundry/cli"
	"foundry/verify"
	"foundry/workspace"
)

func replayUsage() string {
	return `Usage: foundry replay <act-id> --repo <path>

Re-runs verification against a previously recorded Act's patch — without
invoking the Executor again — and reports whether each verify Step
reproduces its recorded Judgment.

This is a same-version replay guarantee only
(docs/06-open-questions/OQ-003-replay-across-versions.md): it proves
verification is reproducible under this Engine build, not that it would
reproduce identically under a future one.

Flags:
  --repo <path>   path to the repository to verify against (required)
  -h, --help      show this help message
`
}

// Replay implements the `foundry replay <act-id>` command: re-run
// verification against a recorded Act's patch and report reproducibility,
// reading from the repository's filesystem Record.
func Replay(ctx context.Context, args []string, stdout io.Writer) int {
	actID, repoPath, err := parseReplayArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, replayUsage())
			return 0
		}
		fmt.Fprintln(stdout, err)
		fmt.Fprint(stdout, replayUsage())
		return 2
	}

	c, ok, err := historyCLI(repoPath, stdout)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if !ok {
		fmt.Fprintf(stdout, "cli: replay: act not found: %s\n", actID)
		return 1
	}

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(repoPath)...)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	verifier := workspace.NewStagedVerifier(gate)

	if err := c.Replay(ctx, actID, verifier, repoPath); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// parseReplayArgs parses `foundry replay` arguments: a required positional
// Act ID and a required --repo flag, in either order.
func parseReplayArgs(args []string) (actID string, repoPath string, err error) {
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help" || arg == "-help":
			return "", "", cli.ErrHelp
		case arg == "--repo" || arg == "-repo":
			i++
			if i >= len(args) {
				return "", "", errors.New("cli: --repo requires a value")
			}
			repoPath = args[i]
		default:
			positional = append(positional, arg)
		}
	}

	if repoPath == "" {
		return "", "", errors.New("cli: --repo is required")
	}
	if len(positional) != 1 {
		return "", "", errors.New("cli: exactly one act-id argument is required")
	}
	return positional[0], repoPath, nil
}
