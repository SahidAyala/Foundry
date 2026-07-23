package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"foundry/cli"
	"foundry/engine"
	"foundry/gatherer"
	"foundry/knowledge"
	"foundry/project"
	"foundry/record"
	"foundry/vcs"
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

	// Interactive reports whether In and Out were both a real terminal
	// character device at construction, before In was wrapped above —
	// computed once here since bufio.Reader loses that information.
	// REPL.Run uses it to choose ADR-0012's rich, completion-aware line
	// editor over the plain line-at-a-time read every non-interactive
	// caller (every existing test, piped input, `foundry < script`) still
	// gets, unchanged.
	Interactive bool

	registry    *engine.PipelineRegistry
	recorder    record.Recorder
	checkpoints *record.CheckpointStore
	gatherer    engine.Gatherer
	verifier    engine.Verifier
	executor    engine.Executor
	executors   *engine.ExecutorRegistry
	appliers    *engine.ApplierRegistry
	cfg         project.Config
}

// NewSession resolves root's full Pipeline registry (built-in plus
// project-local, via project.ProjectLoader) and wires the Engine
// dependencies every slash command shares for the rest of the process.
//
// newNamedExecutor is the vendor-dispatch seam a composition root
// (cmd/foundry/main.go) may supply to construct named, project-configured
// Executors from root's .foundry/executors.json (ADR-0005 Decision 5,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md) — it is
// variadic and optional (pass zero or one) so every existing caller that
// never configures a named Executor keeps compiling and behaving
// identically.
func NewSession(ctx context.Context, root string, in io.Reader, out io.Writer, newExecutor NewExecutor, newNamedExecutor ...project.ExecutorConstructor) (*Session, error) {
	cfg, err := project.LoadConfig(root)
	if err != nil {
		return nil, fmt.Errorf("session: load config: %w", err)
	}

	registry, err := (project.ProjectLoader{}).LoadRegistry(ctx, root, cfg)
	if err != nil {
		return nil, fmt.Errorf("session: load pipelines: %w", err)
	}

	actsDir := filepath.Join(root, ".foundry", "acts")
	recorder, err := record.NewFileStore(actsDir)
	if err != nil {
		return nil, fmt.Errorf("session: open record: %w", err)
	}
	checkpoints, err := record.NewCheckpointStore(actsDir)
	if err != nil {
		return nil, fmt.Errorf("session: open checkpoint store: %w", err)
	}

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(root)...)
	if err != nil {
		return nil, fmt.Errorf("session: build verification gate: %w", err)
	}

	var construct project.ExecutorConstructor
	if len(newNamedExecutor) > 0 {
		construct = newNamedExecutor[0]
	}
	executors, err := project.BuildExecutorRegistry(root, construct)
	if err != nil {
		return nil, fmt.Errorf("session: build executor registry: %w", err)
	}

	appliers, err := buildApplierRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("session: build applier registry: %w", err)
	}

	return &Session{
		Root:        root,
		In:          bufio.NewReader(in),
		Out:         out,
		Interactive: cli.IsInteractiveTerminal(in, out),
		registry:    registry,
		recorder:    recorder,
		checkpoints: checkpoints,
		gatherer:    gatherer.Compose(gatherer.NewNaiveGatherer(root), knowledge.NewGatherer(root)),
		verifier:    workspace.NewStagedVerifier(gate),
		executor:    newExecutor(root),
		executors:   executors,
		appliers:    appliers,
		cfg:         cfg,
	}, nil
}

// buildApplierRegistry registers cfg's Knowledge-lite capture and VCS/PR
// apply targets (RFC-0004 §2.6 Piece 4; ADR-0010 Piece 6, both of
// docs/04-guides/multi-executor-router-implementation-plan.md):
// ApplyTargetKnowledgeNote unconditionally, ApplyTargetProjectDoc only if
// cfg names a DocsPath, and ApplyTargetRemotePR only if cfg names a
// RemotePublishTokenEnv — a project that never opts in registers none of
// the last two and sees no change, exactly as an apply Step with no
// Target already behaves. Mirrors cmd/foundry/commands/do.go's own
// buildApplierRegistry.
func buildApplierRegistry(cfg project.Config) (*engine.ApplierRegistry, error) {
	appliers := engine.NewApplierRegistry()
	if err := appliers.Register(engine.ApplyTargetKnowledgeNote, workspace.KnowledgeNoteApplier{}); err != nil {
		return nil, err
	}
	if cfg.DocsPath != "" {
		if err := appliers.Register(engine.ApplyTargetProjectDoc, workspace.ProjectDocApplier{DocsPath: cfg.DocsPath}); err != nil {
			return nil, err
		}
	}
	if cfg.RemotePublishTokenEnv != "" {
		if err := appliers.Register(engine.ApplyTargetRemotePR, vcs.GitHubPRApplier{TokenEnv: cfg.RemotePublishTokenEnv, RequestCopilotReview: cfg.RequestCopilotReview}); err != nil {
			return nil, err
		}
	}
	return appliers, nil
}

// ReloadPipelines re-resolves the session's Pipeline registry from disk
// — built-in plus project-local, via project.ProjectLoader — so a
// command that changes what a project has authored (/init foremost)
// takes effect immediately, without restarting the session. It reuses the
// Config NewSession already loaded rather than re-reading
// .foundry/config.json — the same session-lifetime treatment executors
// and appliers already get.
func (s *Session) ReloadPipelines(ctx context.Context) error {
	registry, err := (project.ProjectLoader{}).LoadRegistry(ctx, s.Root, s.cfg)
	if err != nil {
		return fmt.Errorf("session: reload pipelines: %w", err)
	}
	s.registry = registry
	return nil
}

// Recorder returns the session's Record, so a CommandHandler can read or
// write Acts (e.g. a future /history or /show) without Session exposing
// any other internals.
func (s *Session) Recorder() record.Recorder {
	return s.recorder
}

// Checkpoints returns the session's CheckpointStore, so a CommandHandler
// can wire it as an Engine's in-progress checkpoint sink
// (engine.Engine.SetCheckpointSaver) — without this, a crash or kill
// mid-Pipeline during an interactive session leaves no checkpoint for
// `foundry resume` to continue from, unlike the one-shot `foundry do`
// path (cmd/foundry/commands/do.go's wireEngine), which always wired one.
func (s *Session) Checkpoints() *record.CheckpointStore {
	return s.checkpoints
}

// Initialized reports whether /init has already scaffolded this project
// — a project.PipelinesDir directory on disk, the same marker
// session_test.go's own end-to-end tests already check for after running
// /init. REPL.Run's banner (ADR-0012) uses this so a user opening a
// fresh checkout is told plainly to run /init rather than guessing.
func (s *Session) Initialized() bool {
	info, err := os.Stat(filepath.Join(s.Root, project.PipelinesDir))
	return err == nil && info.IsDir()
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
	eng := engine.NewEngine(s.gatherer, s.executor, s.verifier, s.Root, pipeline)
	eng.SetRouter(engine.NewRouter(s.executors, s.executor))
	eng.SetApplierRegistry(s.appliers)
	return eng, nil
}
