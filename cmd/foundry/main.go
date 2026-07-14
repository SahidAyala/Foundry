// Command foundry is the CLI entry point.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"foundry/cmd/foundry/commands"
	"foundry/engine"
	"foundry/executor/claude"
	"foundry/session"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, claudeExecutor))
}

// claudeExecutor is the production Executor factory: it invokes the Claude
// Code CLI against the Act's workspace. It is injected at the composition root
// so that main's tests can substitute a deterministic fixture and never depend
// on Claude Code being installed.
func claudeExecutor(workspace string) engine.Executor {
	return claude.NewClaudeExecutor(workspace)
}

func run(args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor) int {
	if len(args) == 0 {
		return runSession(context.Background(), stdin, stdout, newExecutor)
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage())
		return 0
	case "do":
		return commands.Do(context.Background(), args[1:], stdin, stdout, newExecutor)
	case "log":
		return commands.Log(context.Background(), args[1:], stdout)
	case "show":
		return commands.Show(context.Background(), args[1:], stdout)
	case "replay":
		return commands.Replay(context.Background(), args[1:], stdout)
	case "resume":
		return commands.Resume(context.Background(), args[1:], stdin, stdout, newExecutor)
	default:
		fmt.Fprintf(stdout, "foundry: unknown command %q\n\n", args[0])
		fmt.Fprint(stdout, usage())
		return 2
	}
}

// runSession starts an interactive session rooted at the current
// working directory — this is what `foundry` with no subcommand at all
// now does, replacing the old "print usage, exit 2" behavior. The
// one-shot do/log/show subcommands below remain available unchanged for
// scripting and CI; the interactive session is additive, not a
// replacement
// (docs/01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md
// §3.1).
func runSession(ctx context.Context, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	s, err := session.NewSession(ctx, root, stdin, stdout, newExecutor)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(ctx); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// usage lists foundry's subcommands. Each subcommand's own --help gives
// its full usage.
func usage() string {
	return `Usage: foundry <command> [arguments]

Commands:
  do      Run the Act lifecycle for an Intent against a repository
  log     List recorded Acts for a repository
  show    Show one recorded Act in full
  replay  Re-run verification against a recorded Act and report reproducibility
  resume  Continue an Act interrupted mid-Pipeline, or list interrupted acts

Run 'foundry <command> --help' for details on a specific command.
`
}
