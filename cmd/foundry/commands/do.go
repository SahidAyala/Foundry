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
	"foundry/workspace"
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

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(repoPath)...)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	// Validators judge the proposed patch, not the developer's checkout:
	// the Gate runs inside a staged worktree with the patch applied.
	verifier := workspace.NewStagedVerifier(gate)

	// pipelineName is the one place `foundry do` selects which Pipeline
	// runs. It is hardcoded today; a future --pipeline flag replaces this
	// literal with a parsed value — no change to engine.go required.
	const pipelineName = "default"
	pipeline, err := engine.NewDefaultRegistry().Get(pipelineName)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	eng := engine.NewEngine(gatherer.NewNaiveGatherer(repoPath), newExecutor(repoPath), verifier, repoPath, pipeline)
	eng.SetReporter(cli.NewProgressReporter(stdout))
	eng.SetAuthority(cli.InteractiveAuthority{In: stdin, Out: stdout})
	eng.SetApplier(workspace.GitApplier{})
	eng.SetCheckpointer(store)
	c := cli.NewCLI(eng, store, stdin, stdout)

	if err := c.Do(ctx, intent, repoPath); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}
