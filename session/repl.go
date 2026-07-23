package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
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
	history  *cli.PromptHistory
}

// NewREPL wires a REPL over s and commands, rendering to s.Out via a new
// InteractiveRenderer.
func NewREPL(s *Session, commands *CommandRegistry) *REPL {
	return &REPL{session: s, commands: commands, renderer: cli.NewInteractiveRenderer(s.Out), history: cli.NewPromptHistory()}
}

// Run drives the read loop until /exit, /quit, or end of input.
func (r *REPL) Run(ctx context.Context) error {
	r.renderer.Banner(r.session.Root)

	for {
		line, readErr := r.readLine()
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

// readLine acquires the next line of input. A real interactive terminal
// (session.Interactive, ADR-0012) gets bubbletea's rich line editor —
// Tab-completion over registered slash-command names and Up/Down history
// recall within this process — which renders its own prompt; everything
// else (every existing test's strings.Reader/bytes.Buffer, piped input,
// `foundry < script`) falls back to the exact plain
// bufio.Reader.ReadString read this loop always used, so none of that
// needs to change. cli.ErrPromptEOF (Ctrl-C/Ctrl-D at the rich prompt) is
// normalized to io.EOF, the meaning Run already handles from ReadString.
func (r *REPL) readLine() (string, error) {
	if r.session.Interactive {
		line, err := cli.ReadInteractiveLine("foundry> ", r.candidateNames(), r.history)
		if err != nil {
			if errors.Is(err, cli.ErrPromptEOF) {
				return "", io.EOF
			}
			return "", err
		}
		return line, nil
	}
	r.renderer.Prompt()
	return r.session.In.ReadString('\n')
}

// candidateNames lists every slash command ReadInteractiveLine should
// offer as a completion: r.commands.List() (the same vocabulary /help
// renders), plus /exit and /quit — handleLine special-cases those two
// rather than routing them through commands.Dispatch, so they are not
// registered handlers and would otherwise be missing from completion.
func (r *REPL) candidateNames() []string {
	infos := r.commands.List()
	names := make([]string, 0, len(infos)+2)
	for _, info := range infos {
		names = append(names, "/"+info.Name)
	}
	return append(names, "/exit", "/quit")
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

	if err := r.dispatchRecovered(ctx, cmd); err != nil {
		r.renderer.Error(err)
	}
	return false
}

// dispatchRecovered runs r.commands.Dispatch, converting a panic anywhere
// in that call chain (RunPipelineCommand -> cli.CLI.Do -> Engine ->
// Gatherer/Executor/Verifier/Applier) into a returned error instead of
// crashing the whole interactive session. Run's own doc comment already
// promises "one failed slash command must never end the session" — that
// guarantee held only for a returned error before this; a panic bypassed
// it entirely, taking the whole process down mid-session. The panic's
// full value and stack trace are still written to stderr, so a real
// programming bug remains loud and debuggable during development — only
// the interactive session itself survives it, exactly as it already
// survives a returned error.
func (r *REPL) dispatchRecovered(ctx context.Context, cmd Command) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "session: recovered panic in /%s: %v\n%s\n", cmd.Name, rec, debug.Stack())
			err = fmt.Errorf("session: /%s panicked and was recovered: %v (see stderr for the full stack trace)", cmd.Name, rec)
		}
	}()
	return r.commands.Dispatch(ctx, r.session, cmd.Name, cmd.Args)
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
