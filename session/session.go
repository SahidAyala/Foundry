package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SahidAyala/Foundry/cli"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/executor/claude"
	"github.com/SahidAyala/Foundry/gatherer"
	"github.com/SahidAyala/Foundry/knowledge"
	"github.com/SahidAyala/Foundry/model"
	"github.com/SahidAyala/Foundry/project"
	"github.com/SahidAyala/Foundry/record"
	"github.com/SahidAyala/Foundry/ticket"
	"github.com/SahidAyala/Foundry/vcs"
	"github.com/SahidAyala/Foundry/verify"
	"github.com/SahidAyala/Foundry/verify/aireview"
	"github.com/SahidAyala/Foundry/workspace"
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

	registry      *engine.PipelineRegistry
	recorder      record.Recorder
	checkpoints   *record.CheckpointStore
	gatherer      engine.Gatherer
	verifier      engine.Verifier
	executor      engine.Executor
	executors     *engine.ExecutorRegistry
	appliers      *engine.ApplierRegistry
	cfg           project.Config
	ticketFetcher ticket.Fetcher
	models        *model.Registry

	// trace routes the Executors' own live output to whichever Reporter
	// the running command created. Built once, with the Executors; each
	// command points it at its own Reporter for the duration of one Act.
	trace *cli.TraceRelay
}

// Trace returns the relay a command points at its own Reporter while it
// runs an Act, so the session's long-lived Executors narrate into the
// short-lived Reporter actually on screen (opt-in via FOUNDRY_VERBOSE; see
// cli.TraceRelay).
func (s *Session) Trace() *cli.TraceRelay {
	return s.trace
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

	gate, err := verify.NewGate("all-pass", buildValidators(root, cfg)...)
	if err != nil {
		return nil, fmt.Errorf("session: build verification gate: %w", err)
	}
	var verifier engine.Verifier = workspace.NewStagedVerifier(gate)

	// AIReviewModel is a supplementary, non-deterministic verify Step
	// composed alongside the deterministic Gate above — never replacing
	// it (docs/02-architecture/trust.md's stated preference for
	// deterministic checks first). Empty means this feature is entirely
	// off, exactly as if it did not exist.
	if cfg.AIReviewModel != "" {
		if cfg.AIReviewBaseURL == "" {
			return nil, fmt.Errorf("session: ai_review_model is set but ai_review_base_url is not, in .foundry/config.json")
		}
		verifier = verify.Compose(verifier, aireview.NewVerifier(cfg.AIReviewModel, os.Getenv(cfg.AIReviewAPIKeyEnv), cfg.AIReviewBaseURL))
	}

	// AIReviewClaudeModel is the same kind of supplementary layer as
	// AIReviewModel above, but runs Claude Code's own CLI (the caller's
	// existing subscription) instead of an HTTP call — see its own doc
	// comment in project.Config for the same-model-review tradeoff this
	// implies.
	if cfg.AIReviewClaudeModel != "" {
		verifier = verify.Compose(verifier, claude.NewReviewer(cfg.AIReviewClaudeModel))
	}

	var construct project.ExecutorConstructor
	if len(newNamedExecutor) > 0 {
		construct = newNamedExecutor[0]
	}

	// Executors are built once here but narrate through a Reporter created
	// per slash command, so their live output (opt-in via FOUNDRY_VERBOSE)
	// is routed through a relay each command points at its own Reporter —
	// see cli.TraceRelay for why the indirection is needed.
	trace := cli.NewTraceRelay()
	executors, err := project.BuildExecutorRegistry(root, project.WrapConstructor(construct, func(e engine.Executor) {
		cli.TraceExecutorTo(e, trace.Line)
	}))
	if err != nil {
		return nil, fmt.Errorf("session: build executor registry: %w", err)
	}

	def := newExecutor(root)
	cli.TraceExecutorTo(def, trace.Line)

	appliers, err := buildApplierRegistry(root, cfg)
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
		verifier:    verifier,
		executor:    def,
		executors:   executors,
		trace:       trace,
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
// Target already behaves. root is only needed for GitHubPRApplier's
// optional Summarizer (claude.NewSummarizer's own workspace argument);
// every other Applier here is workspace-agnostic. Mirrors
// cmd/foundry/commands/do.go's own buildApplierRegistry.
func buildApplierRegistry(root string, cfg project.Config) (*engine.ApplierRegistry, error) {
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
		applier := vcs.GitHubPRApplier{TokenEnv: cfg.RemotePublishTokenEnv, RequestCopilotReview: cfg.RequestCopilotReview}
		if cfg.PRSummaryModel != "" {
			applier.Summarizer = claude.NewSummarizer(root, cfg.PRSummaryModel)
		}
		if err := appliers.Register(engine.ApplyTargetRemotePR, applier); err != nil {
			return nil, err
		}
	}
	return appliers, nil
}

// buildValidators returns cfg.Validators, converted to []*verify.Validator,
// when the project has declared any — replacing verify.DefaultValidators'
// own root-only auto-detection entirely, for a project it can't detect
// correctly (e.g. a polyglot monorepo with no root go.mod/package.json).
// An empty Validators list (the default) falls back to auto-detection
// exactly as before this field existed. Mirrors
// cmd/foundry/commands/do.go's own buildValidators.
func buildValidators(root string, cfg project.Config) []*verify.Validator {
	if len(cfg.Validators) == 0 {
		return verify.DefaultValidators(root)
	}
	validators := make([]*verify.Validator, len(cfg.Validators))
	for i, vc := range cfg.Validators {
		validators[i] = &verify.Validator{Name: vc.Name, Cmd: vc.Cmd}
	}
	return validators
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

// SetTicketFetcher attaches fetcher as /issue's ticket-fetching backend
// (the composition root builds one from project.Config.TicketProvider via
// a project.TicketFetcherConstructor, the same seam SetRouter/SetApplier
// mirror for Engine). Optional: if never called, IssueCommand.Run reports
// a clear "no ticket provider configured" error rather than a nil-pointer
// panic.
func (s *Session) SetTicketFetcher(fetcher ticket.Fetcher) {
	s.ticketFetcher = fetcher
}

// SetModelRegistry attaches models as the Model Registry (ADR-0013,
// Proposed) every Engine built by Engine below resolves a Step's "model"
// field against, mirroring SetTicketFetcher's own optional, post-
// construction attachment pattern. Optional: if never called (or called
// with nil), s.models stays nil and Engine's Router behaves exactly as it
// did before Model existed — a Step naming "model" then fails with a
// clear, named error at Resolve time instead of silently falling back.
func (s *Session) SetModelRegistry(models *model.Registry) {
	s.models = models
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
	eng.SetRouter(engine.NewRouter(s.executors, s.executor).WithModels(s.models))
	eng.SetApplierRegistry(s.appliers)
	return eng, nil
}
