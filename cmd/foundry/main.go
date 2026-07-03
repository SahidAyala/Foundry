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
		fmt.Fprintln(os.Stderr, "usage: foundry <command> [arguments]")
		return 2
	}

	switch args[0] {
	case "do":
		return commands.Do(context.Background(), args[1:], stdin, stdout, newExecutor)
	default:
		fmt.Fprintf(os.Stderr, "foundry: unknown command %q\n", args[0])
		return 2
	}
}
