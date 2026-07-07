package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"foundry/cli"
	"foundry/record"
)

func showUsage() string {
	return `Usage: foundry show <act-id> --repo <path>

Shows the full recorded Act: intent, judgment, approval, considered
evidence, and the patch as a unified diff.

Flags:
  --repo <path>   path to the repository whose history to read (required)
  -h, --help      show this help message
`
}

// Show implements the `foundry show <act-id>` command: display one recorded
// Act in full, reading from the repository's filesystem Record.
func Show(ctx context.Context, args []string, stdout io.Writer) int {
	actID, repoPath, err := parseShowArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, showUsage())
			return 0
		}
		fmt.Fprintln(stdout, err)
		fmt.Fprint(stdout, showUsage())
		return 2
	}

	c, ok, err := historyCLI(repoPath, stdout)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if !ok {
		fmt.Fprintf(stdout, "cli: show: act not found: %s\n", actID)
		return 1
	}

	if err := c.Show(ctx, actID); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// parseShowArgs parses `foundry show` arguments: a required positional Act
// ID and a required --repo flag, in either order.
func parseShowArgs(args []string) (actID string, repoPath string, err error) {
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

// historyCLI builds a CLI over the repository's Record for read-only history
// inspection. ok is false when the repository has no Record yet; reading
// history must not create one, so this is checked before opening the store.
// The Engine is nil because Log and Show never produce Acts.
func historyCLI(repoPath string, stdout io.Writer) (c *cli.CLI, ok bool, err error) {
	actsDir := filepath.Join(repoPath, ".foundry", "acts")
	if _, statErr := os.Stat(actsDir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cli: read record: %w", statErr)
	}

	store, err := record.NewFileStore(actsDir)
	if err != nil {
		return nil, false, err
	}
	return cli.NewCLI(nil, store, nil, stdout), true, nil
}
