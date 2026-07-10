package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"foundry/engine"
	"foundry/gatherer"
	"foundry/project"
	"foundry/record"
	"foundry/verify"
	"foundry/workspace"
)

// NewExecutor builds the Executor a Session uses for every Pipeline it
// runs, given the project root. Production injects the Claude Code
// executor; tests inject a deterministic scripted fixture — the same
// injection point cmd/foundry/main.go already uses for its one-shot
// subcommands.
type NewExecutor func(root string) engine.Executor

// Session owns everything an interactive run against one project needs
// for its entire lifetime: the project root, the Pipeline registry
// resolved once at startup (built-in plus project-local Pipelines, via
// project.ProjectLoader), and the reusable Engine dependencies every
// slash command that runs a Pipeline shares. It knows nothing about
// slash-command syntax, terminal rendering, or the read loop — those
// belong to REPL and to CommandHandler implementations.
//
// Session deliberately does not hold a single *engine.Engine: a session
// runs many Pipelines over its lifetime ("default", "review", "feature",
// "bugfix", ...), and engine.NewEngine is cheap to call — the same
// construction cmd/foundry/commands/do.go already performs once per
// process, called here once per slash command instead. No change to
// engine.Engine was needed to support this.
type Session struct {
	// Root is the project directory this session runs against — always
	// the current working directory foundry was started in; there is no
	// --repo flag on the interactive surface.
	Root string

	// In and Out are the session's whole-lifetime input and output —
	// shared by every CommandHandler and, through it, every cli.CLI it
	// constructs, for both reading approval and writing output. In is
	// wrapped in exactly one *bufio.Reader for the session's entire
	// lifetime (NewSession does this once): cli.PromptForApproval reads
	// directly from an already-*bufio.Reader input instead of wrapping
	// it again, so state (how much of the stream has been consumed)
	// survives correctly across more than one approval prompt over the
	// session's life — see the note on cli.PromptForApproval.
	In  *bufio.Reader
	Out io.Writer

	registry *engine.PipelineRegistry
	recorder record.Recorder
	gatherer engine.Gatherer
	verifier engine.Verifier
	executor engine.Executor
}

// NewSession resolves root's full Pipeline registry (built-in plus
// project-local, via project.ProjectLoader) and wires the Engine
// dependencies every slash command shares for the rest of the process.
func NewSession(ctx context.Context, root string, in io.Reader, out io.Writer, newExecutor NewExecutor) (*Session, error) {
	registry, err := (project.ProjectLoader{}).LoadRegistry(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("session: load pipelines: %w", err)
	}

	recorder, err := record.NewFileStore(filepath.Join(root, ".foundry", "acts"))
	if err != nil {
		return nil, fmt.Errorf("session: open record: %w", err)
	}

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(root)...)
	if err != nil {
		return nil, fmt.Errorf("session: build verification gate: %w", err)
	}

	return &Session{
		Root:     root,
		In:       bufio.NewReader(in),
		Out:      out,
		registry: registry,
		recorder: recorder,
		gatherer: gatherer.NewNaiveGatherer(root),
		verifier: workspace.NewStagedVerifier(gate),
		executor: newExecutor(root),
	}, nil
}

// Recorder returns the session's Record, so a CommandHandler can read or
// write Acts (e.g. a future /history or /show) without Session exposing
// any other internals.
func (s *Session) Recorder() record.Recorder {
	return s.recorder
}

// Engine resolves pipelineName from the session's registry and returns a
// fresh *engine.Engine wired to run it, reusing every other dependency
// (Gatherer, Verifier, Executor) across the session's whole lifetime. An
// unresolved name is a clear, named error pointing at /init — never a
// silent fallback to any other Pipeline.
func (s *Session) Engine(pipelineName string) (*engine.Engine, error) {
	pipeline, err := s.registry.Get(pipelineName)
	if err != nil {
		return nil, fmt.Errorf("session: %w (run /init to scaffold a starter, or check %s)", err, project.PipelinesDir)
	}
	return engine.NewEngine(s.gatherer, s.executor, s.verifier, s.Root, pipeline), nil
}
