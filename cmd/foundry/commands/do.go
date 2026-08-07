// Package commands implements foundry's subcommands.
package commands

import (
	"context"
	"errors"
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
	"github.com/SahidAyala/Foundry/vcs"
	"github.com/SahidAyala/Foundry/verify"
	"github.com/SahidAyala/Foundry/verify/aireview"
	"github.com/SahidAyala/Foundry/workspace"
)

// Do implements the `foundry do` command: parse its arguments, wire the Act
// lifecycle for the requested repository, run it through approval, and return
// the process exit code.
//
// newExecutor builds the Executor for the resolved workspace. Production
// injects the Claude Code executor; the deterministic golden/integration
// tests inject a scripted fixture, so this command never requires Claude Code
// to be present under test.
//
// newNamedExecutor constructs a named, project-configured Executor from a
// decoded project.ExecutorConfig — the vendor-dispatch seam Do's caller
// (cmd/foundry/main.go, Foundry's true composition root) supplies, so this
// package stays vendor-agnostic (ADR-0005 Decision 5,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md).
//
// models is the optional Model Registry (ADR-0013, Proposed) a Step's
// "model" field resolves against, in the same variadic zero-or-one shape
// session.NewSession already established for newNamedExecutor — omitted
// entirely, every existing caller (and every existing Pipeline document,
// which never sets Model) keeps resolving Executor-only Steps exactly as
// before Model existed.
func Do(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor, models ...*model.Registry) int {
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

	// pipelineName is the one place `foundry do` selects which Pipeline
	// runs. It is hardcoded today; a future --pipeline flag replaces this
	// literal with a parsed value — no change to engine.go required.
	const pipelineName = "default"
	eng, store, _, err := wireEngine(ctx, repoPath, stdin, stdout, newExecutor, newNamedExecutor, pipelineName, models...)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	c := cli.NewCLI(eng, store, stdin, stdout)

	if err := c.Do(ctx, intent, repoPath); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// wireEngine builds the Engine `foundry do` and `foundry resume` both need:
// a filesystem Record and CheckpointStore rooted at repoPath's .foundry/acts,
// a staged Gate-backed Verifier, the named Pipeline, and every port an
// interactive run drives (Reporter, Authority, Applier, Checkpointer, and
// CheckpointSaver — the last so a crash mid-Pipeline leaves a checkpoint
// `foundry resume` can continue). It also registers repoPath's project-local
// Executor configuration (project.BuildExecutorRegistry, .foundry/executors.json)
// into a Router, falling back to newExecutor's default Executor — a project
// with no such file sees byte-for-byte the same routing as before this
// existed. buildApplierRegistry similarly registers repoPath's Knowledge-lite
// capture and VCS/PR apply targets (RFC-0004 §2.6, ADR-0010) into an
// ApplierRegistry. The Engine's Gatherer is gatherer.Compose'd from
// NaiveGatherer (repository files) and knowledge.Gatherer (Authored
// Knowledge notes under .foundry/knowledge/, RFC-0005) — a project with no
// such directory yet sees byte-for-byte the same considered Context as
// before this existed.
//
// pipelineName is resolved from the project's full registry
// (project.ProjectLoader.LoadRegistry — every built-in Pipeline plus every
// Pipeline the project has authored under .foundry/pipelines/), not only
// Foundry's built-ins. This matters concretely for `foundry resume`: an
// interactive session (session.NewSession) already runs project-local
// Pipelines like "feature"/"bugfix"/"release", and since session wires
// SetCheckpointSaver too, a checkpoint left by an interrupted interactive
// Act names one of those Pipelines — resolving it from only
// engine.NewDefaultRegistry() (built-ins alone, as this function used to)
// would fail resume for exactly the Pipelines a real project actually
// uses. `foundry do` itself only ever asks for "default" (a built-in), so
// this is a strict superset for that caller — no behavior change there.
//
// models is the optional Model Registry (ADR-0013, Proposed) attached to
// the Router via Router.WithModels when present — a Step's "model" field
// resolves against it instead of "executor" when both are set. Omitted (or
// nil), the Router behaves exactly as it did before Model existed.
func wireEngine(ctx context.Context, repoPath string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor, pipelineName string, models ...*model.Registry) (*engine.Engine, *record.FileStore, *record.CheckpointStore, error) {
	actsDir := filepath.Join(repoPath, ".foundry", "acts")

	store, err := record.NewFileStore(actsDir)
	if err != nil {
		return nil, nil, nil, err
	}
	checkpoints, err := record.NewCheckpointStore(actsDir)
	if err != nil {
		return nil, nil, nil, err
	}

	cfg, err := project.LoadConfig(repoPath)
	if err != nil {
		return nil, nil, nil, err
	}

	gate, err := verify.NewGate("all-pass", buildValidators(repoPath, cfg)...)
	if err != nil {
		return nil, nil, nil, err
	}

	// Validators judge the proposed patch, not the developer's checkout:
	// the Gate runs inside a staged worktree with the patch applied.
	var verifier engine.Verifier = workspace.NewStagedVerifier(gate)

	// AIReviewModel is a supplementary, non-deterministic verify Step
	// composed alongside the deterministic Gate above — never replacing
	// it (docs/02-architecture/trust.md's stated preference for
	// deterministic checks first). Empty means this feature is entirely
	// off, exactly as if it did not exist.
	if cfg.AIReviewModel != "" {
		if cfg.AIReviewBaseURL == "" {
			return nil, nil, nil, fmt.Errorf("foundry: ai_review_model is set but ai_review_base_url is not, in .foundry/config.json")
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

	pipelines, err := (project.ProjectLoader{}).LoadRegistry(ctx, repoPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	pipeline, err := pipelines.Get(pipelineName)
	if err != nil {
		return nil, nil, nil, err
	}

	// The Reporter is built before the Executors so each one can be wired
	// to narrate its own live output through it (cli.TraceExecutor, opt-in
	// via FOUNDRY_VERBOSE) — an Executor has no other way to reach the
	// terminal, since it sits below the Engine and is handed no writer.
	reporter := cli.NewReporter(stdout)

	def := newExecutor(repoPath)
	cli.TraceExecutor(def, reporter)
	repoGatherer := gatherer.Compose(gatherer.NewNaiveGatherer(repoPath), knowledge.NewGatherer(repoPath))
	eng := engine.NewEngine(repoGatherer, def, verifier, repoPath, pipeline)

	// The project-configured Executors (.foundry/executors.json) get the
	// same live narration as the default one above; BuildExecutorRegistry
	// hands back a registry rather than its contents, so this is the only
	// point they can be reached.
	traced := project.WrapConstructor(newNamedExecutor, func(e engine.Executor) { cli.TraceExecutor(e, reporter) })
	registry, err := project.BuildExecutorRegistry(repoPath, traced)
	if err != nil {
		return nil, nil, nil, err
	}
	router := engine.NewRouter(registry, def)
	if len(models) > 0 {
		router = router.WithModels(models[0])
	}
	eng.SetRouter(router)

	appliers, err := buildApplierRegistry(repoPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	eng.SetApplierRegistry(appliers)

	eng.SetReporter(reporter)
	eng.SetAuthority(cli.InteractiveAuthority{In: stdin, Out: stdout})
	eng.SetApplier(workspace.GitApplier{})
	eng.SetCheckpointer(store)
	eng.SetCheckpointSaver(checkpoints)

	return eng, store, checkpoints, nil
}

// buildApplierRegistry registers cfg's Knowledge-lite capture and VCS/PR
// apply targets (RFC-0004 §2.6, Piece 4; ADR-0010, Piece 6 — both of
// docs/04-guides/multi-executor-router-implementation-plan.md):
// ApplyTargetKnowledgeNote unconditionally, ApplyTargetProjectDoc only if
// cfg names a DocsPath, and ApplyTargetRemotePR only if cfg names a
// RemotePublishTokenEnv — a project that never opts in registers none of
// the last two and sees no change, exactly as an apply Step with no Target
// already behaves. root is only needed for GitHubPRApplier's optional
// Summarizer (claude.NewSummarizer's own workspace argument). Mirrors
// session.Session's own buildApplierRegistry.
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
// exactly as before this field existed. Mirrors session.Session's own
// buildValidators.
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
