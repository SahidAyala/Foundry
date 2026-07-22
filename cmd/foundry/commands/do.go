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
	"foundry/knowledge"
	"foundry/project"
	"foundry/record"
	"foundry/vcs"
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
//
// newNamedExecutor constructs a named, project-configured Executor from a
// decoded project.ExecutorConfig — the vendor-dispatch seam Do's caller
// (cmd/foundry/main.go, Foundry's true composition root) supplies, so this
// package stays vendor-agnostic (ADR-0005 Decision 5,
// docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md).
func Do(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor) int {
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
	eng, store, _, err := wireEngine(ctx, repoPath, stdin, stdout, newExecutor, newNamedExecutor, pipelineName)
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
func wireEngine(ctx context.Context, repoPath string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor, pipelineName string) (*engine.Engine, *record.FileStore, *record.CheckpointStore, error) {
	actsDir := filepath.Join(repoPath, ".foundry", "acts")

	store, err := record.NewFileStore(actsDir)
	if err != nil {
		return nil, nil, nil, err
	}
	checkpoints, err := record.NewCheckpointStore(actsDir)
	if err != nil {
		return nil, nil, nil, err
	}

	gate, err := verify.NewGate("all-pass", verify.DefaultValidators(repoPath)...)
	if err != nil {
		return nil, nil, nil, err
	}

	// Validators judge the proposed patch, not the developer's checkout:
	// the Gate runs inside a staged worktree with the patch applied.
	verifier := workspace.NewStagedVerifier(gate)

	cfg, err := project.LoadConfig(repoPath)
	if err != nil {
		return nil, nil, nil, err
	}

	pipelines, err := (project.ProjectLoader{}).LoadRegistry(ctx, repoPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	pipeline, err := pipelines.Get(pipelineName)
	if err != nil {
		return nil, nil, nil, err
	}

	def := newExecutor(repoPath)
	repoGatherer := gatherer.Compose(gatherer.NewNaiveGatherer(repoPath), knowledge.NewGatherer(repoPath))
	eng := engine.NewEngine(repoGatherer, def, verifier, repoPath, pipeline)

	registry, err := project.BuildExecutorRegistry(repoPath, newNamedExecutor)
	if err != nil {
		return nil, nil, nil, err
	}
	eng.SetRouter(engine.NewRouter(registry, def))

	appliers, err := buildApplierRegistry(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	eng.SetApplierRegistry(appliers)

	eng.SetReporter(cli.NewReporter(stdout))
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
// already behaves. Mirrors session.Session's own buildApplierRegistry.
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
		if err := appliers.Register(engine.ApplyTargetRemotePR, vcs.GitHubPRApplier{TokenEnv: cfg.RemotePublishTokenEnv}); err != nil {
			return nil, err
		}
	}
	return appliers, nil
}
