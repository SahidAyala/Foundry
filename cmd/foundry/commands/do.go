// Package commands implements foundry's subcommands.
package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"foundry/cli"
	"foundry/engine"
	"foundry/gatherer"
	"foundry/record"
	"foundry/verify"
)

// Do implements the `foundry do` command: parse its arguments, wire the Act
// lifecycle for the requested repository, run it through approval, and return
// the process exit code.
//
// newExecutor builds the Executor for the resolved workspace. Production
// injects the Claude Code executor; the deterministic golden/integration
// tests inject a scripted fixture, so this command never requires Claude Code
// to be present under test.
func Do(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor) int {
	intent, repoPath, err := cli.ParseArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, cli.Usage())
			return 0
		}
		fmt.Fprintln(stdout, err)
		fmt.Fprint(stdout, cli.Usage())
		return 2
	}

	store, err := record.NewFileStore(filepath.Join(repoPath, ".foundry", "acts"))
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	// PIC-1 pins real, project-specific build/test commands once a target
	// project is chosen; until then this checks that repoPath is a usable
	// git repository.
	gate, err := verify.NewGate("all-pass", &verify.Validator{Name: "repo-sanity", Cmd: "git rev-parse HEAD"})
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	eng := engine.NewEngine(gatherer.NewNaiveGatherer(repoPath), newExecutor(repoPath), gate, repoPath)
	c := cli.NewCLI(eng, store, stdin, stdout)

	if err := c.Do(ctx, intent, repoPath); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}
