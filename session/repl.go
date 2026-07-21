package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"foundry/cli"
)

// REPL is the interactive session's read loop: it reads one line at a
// time from the Session's input, renders a prompt via an
// InteractiveRenderer, parses each line with ParseLine, and dispatches
// slash commands through a CommandRegistry. It exits cleanly on /exit,
// /quit, or end of input (e.g. Ctrl-D).
//
// A dispatch error is rendered and the loop continues: one failed slash
// command must never end the session, mirroring how a shell survives a
// failed command.
//
// Plain text (not a slash command) is reported as unsupported rather
// than silently ignored. RFC-0002 §8 says free text should eventually
// become an Intent for a default Pipeline; wiring that is a later,
// separate change to this same loop, not a reason to hide the gap now.
type REPL struct {
	session  *Session
	commands *CommandRegistry
	renderer *cli.InteractiveRenderer
}

// NewREPL wires a REPL over s and commands, rendering to s.Out via a new
// InteractiveRenderer.
func NewREPL(s *Session, commands *CommandRegistry) *REPL {
	return &REPL{session: s, commands: commands, renderer: cli.NewInteractiveRenderer(s.Out)}
}

// Run drives the read loop until /exit, /quit, or end of input.
func (r *REPL) Run(ctx context.Context) error {
	r.renderer.Banner(r.session.Root)

	for {
		r.renderer.Prompt()
		line, readErr := r.session.In.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("session: read input: %w", readErr)
		}

		if done := r.handleLine(ctx, line); done {
			return nil
		}
		if readErr == io.EOF {
			return nil
		}
	}
}

// handleLine processes one line already read from input. It returns
// done=true if the REPL should stop after this line (an explicit
// /exit or /quit).
func (r *REPL) handleLine(ctx context.Context, line string) (done bool) {
	if strings.TrimSpace(line) == "" {
		return false
	}

	cmd, isSlash := ParseLine(line)
	if !isSlash {
		r.renderer.Error(errors.New("plain-text intents are not yet supported; use a slash command, e.g. /feature \"...\""))
		return false
	}
	if cmd.Name == "exit" || cmd.Name == "quit" {
		return true
	}

	if err := r.commands.Dispatch(ctx, r.session, cmd.Name, cmd.Args); err != nil {
		r.renderer.Error(err)
	}
	return false
}

// DefaultCommandRegistry returns the CommandRegistry this build of
// Foundry ships by default: /init, /help, plus one RunPipelineCommand
// per slash command a fresh project resolves out of the box — /feature,
// /bug, /review, /release — matching the Pipeline names
// project.ProjectLoader.Scaffold writes starters for ("feature",
// "bugfix", "release") plus the "review" built-in. A project that
// authors its own additional Pipeline document can back a matching
// slash command the same way; this registry is the default set, not
// the only possible one.
func DefaultCommandRegistry() *CommandRegistry {
	registry := NewCommandRegistry()
	must := func(name string, h CommandHandler) {
		if err := registry.Register(name, h); err != nil {
			panic(fmt.Sprintf("session: DefaultCommandRegistry: %v", err))
		}
	}
	must("init", InitCommand{})
	must("feature", RunPipelineCommand{PipelineName: "feature"})
	must("bug", RunPipelineCommand{PipelineName: "bugfix"})
	must("review", RunPipelineCommand{PipelineName: "review"})
	must("release", RunPipelineCommand{PipelineName: "release"})
	// help is registered last so its own Registry pointer already sees
	// every command above (registry is a pointer; Register mutates it
	// in place, and help.Run only reads it later, at dispatch time).
	must("help", HelpCommand{Registry: registry})
	return registry
}
