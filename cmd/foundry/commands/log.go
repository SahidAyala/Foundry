package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"foundry/cli"
)

const defaultLogLimit = 10

func logUsage() string {
	return `Usage: foundry log --repo <path> [-n <count>]

Lists the most recently recorded Acts for the repository, newest first.

Flags:
  --repo <path>   path to the repository whose history to list (required)
  -n <count>      number of Acts to list (default 10)
  -h, --help      show this help message
`
}

// Log implements the `foundry log` command: list the most recently recorded
// Acts for a repository, reading from its filesystem Record.
func Log(ctx context.Context, args []string, stdout io.Writer) int {
	repoPath, limit, err := parseLogArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, logUsage())
			return 0
		}
		fmt.Fprintln(stdout, err)
		fmt.Fprint(stdout, logUsage())
		return 2
	}

	c, ok, err := historyCLI(repoPath, stdout)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if !ok {
		fmt.Fprintln(stdout, "No acts recorded.")
		return 0
	}

	if err := c.Log(ctx, limit); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// parseLogArgs parses `foundry log` arguments: a required --repo flag and an
// optional -n count.
func parseLogArgs(args []string) (repoPath string, limit int, err error) {
	limit = defaultLogLimit

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help" || arg == "-help":
			return "", 0, cli.ErrHelp
		case arg == "--repo" || arg == "-repo":
			i++
			if i >= len(args) {
				return "", 0, errors.New("cli: --repo requires a value")
			}
			repoPath = args[i]
		case arg == "-n" || arg == "--limit":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("cli: %s requires a value", arg)
			}
			limit, err = strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return "", 0, fmt.Errorf("cli: -n requires a positive number, got %q", args[i])
			}
		default:
			return "", 0, fmt.Errorf("cli: unexpected argument %q", arg)
		}
	}

	if repoPath == "" {
		return "", 0, errors.New("cli: --repo is required")
	}
	return repoPath, limit, nil
}
