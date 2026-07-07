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
		fmt.Fprint(stdout, usage())
		return 2
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
	default:
		fmt.Fprintf(stdout, "foundry: unknown command %q\n\n", args[0])
		fmt.Fprint(stdout, usage())
		return 2
	}
}

// usage lists foundry's subcommands. Each subcommand's own --help gives
// its full usage.
func usage() string {
	return `Usage: foundry <command> [arguments]

Commands:
  do    Run the Act lifecycle for an Intent against a repository
  log   List recorded Acts for a repository
  show  Show one recorded Act in full

Run 'foundry <command> --help' for details on a specific command.
`
}
