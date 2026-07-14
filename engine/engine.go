// Package engine drives the machine-owned production of an Act: it gathers
// Context, enforces Budget, and delegates producing the Act's Outcome and
// Judgment to a Strategy (strategy.go) walking a Pipeline (step.go) — the
// Engine no longer hardcodes repair as bespoke Go control flow, nor
// constructs or selects a Pipeline itself
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase 2). Which
// Pipeline to run is resolved by the Engine's caller — today, from a
// PipelineRegistry (registry.go) — and handed to NewEngine; the Engine
// depends only on the resulting Pipeline value, never on a name, a
// registry, or how one was built. The accountable steps described in
// docs/02-architecture/execution.md steps 5–8 — an Authority's acceptance,
// applying the Outcome, and recording the Act — were once entirely outside
// the Engine's knowledge, driven by the caller after Run returned. RFC-0002
// §9 Phase 4 moves them inside: a Pipeline that declares approve/apply
// Steps has the Engine call the Authority/Applier ports below at the point
// the Pipeline names, not only after the fact. A Pipeline that declares
// none of them behaves exactly as before — the caller still drives that
// trust boundary itself.
package engine

import (
	"context"
	"errors"
	"fmt"

	"foundry/domain"
)

// Engine produces machine-judged Acts.
type Engine struct {
	gatherer     Gatherer
	executor     Executor
	verifier     Verifier
	workspace    string // directory the Verifier checks
	reporter     Reporter
	authority    Authority
	applier      Applier
	checkpointer Checkpointer
	checkpoints  CheckpointSaver
	strategy     Strategy
	pipelineName string
}

// NewEngine wires the ports an Engine needs to produce an Act, using
// PipelineStrategy over pipeline — today's only Strategy. The caller
// resolves pipeline (today, always DefaultPipeline via a PipelineRegistry
// lookup made by the composition root, e.g. cmd/foundry/commands/do.go);
// NewEngine neither constructs nor selects one, so which Pipeline a future
// caller passes — driven by CLI configuration or otherwise — never
// requires an Engine change. workspace is the directory the Verifier
// checks; for M0.0 this is the repository path.
func NewEngine(gatherer Gatherer, executor Executor, verifier Verifier, workspace string, pipeline Pipeline) *Engine {
	return &Engine{
		gatherer:     gatherer,
		executor:     executor,
		verifier:     verifier,
		workspace:    workspace,
		reporter:     noopReporter{},
		authority:    noAuthority{},
		applier:      noApplier{},
		checkpointer: noCheckpointer{},
		checkpoints:  noCheckpointSaver{},
		strategy:     PipelineStrategy{Pipeline: pipeline},
		pipelineName: pipeline.Name,
	}
}

// SetReporter attaches r as the Engine's progress observer, replacing the
// default no-op. Reporter is optional and additive: it never changes what
// Run or RunBudgeted does, only what a caller can observe while it runs.
func (e *Engine) SetReporter(r Reporter) {
	e.reporter = r
}

// SetAuthority attaches a as the Engine's approve Step decision-maker,
// replacing the default noAuthority. Only a Pipeline that declares an
// approve Step ever calls it — every other Pipeline's behavior is
// unaffected whether or not SetAuthority is called.
func (e *Engine) SetAuthority(a Authority) {
	e.authority = a
}

// SetApplier attaches a as the Engine's apply Step mechanism, replacing the
// default noApplier. Only a Pipeline that declares an apply Step ever calls
// it — every other Pipeline's behavior is unaffected whether or not
// SetApplier is called.
func (e *Engine) SetApplier(a Applier) {
	e.applier = a
}

// SetCheckpointer attaches c as the Engine's record Step mechanism,
// replacing the default noCheckpointer. Only a Pipeline that declares a
// record Step ever calls it — every other Pipeline's behavior is
// unaffected whether or not SetCheckpointer is called.
func (e *Engine) SetCheckpointer(c Checkpointer) {
	e.checkpointer = c
}

// SetCheckpointSaver attaches c as the Engine's in-progress checkpoint
// sink, replacing the default noCheckpointSaver. Unlike SetCheckpointer
// (which only a declared record Step calls), the Engine calls c after
// every Step of every Pipeline, so a crash mid-Pipeline leaves state
// `foundry resume` can continue (docs/06-open-questions/OQ-008-in-progress-act-persistence.md).
func (e *Engine) SetCheckpointSaver(c CheckpointSaver) {
	e.checkpoints = c
}

// Run gathers context, executes the work, and verifies the outcome under
// the default M0.1 Budget, returning an Act with its considered context,
// proposed patch, and machine verdict. A Pipeline that declares approve,
// apply, or record Steps has already sought approval, applied the Outcome,
// or persisted the Act by the time Run returns, via whichever
// Authority/Applier/Checkpointer the caller configured; a Pipeline that
// declares none of them leaves all three to the caller, exactly as before
// those Step kinds existed.
func (e *Engine) Run(ctx context.Context, intent *domain.Intent) (*domain.Act, error) {
	return e.RunBudgeted(ctx, intent, DefaultBudget())
}

// RunBudgeted is Run under an explicit Budget. It gathers Context itself,
// then hands off to its Strategy (today, PipelineStrategy over
// DefaultPipeline) to produce the Act's Outcome and Judgment, charging
// every unit of work against the Budget's iteration and cost ceilings. If a
// call would exceed the Budget on the Strategy's first attempt, the Engine
// halts and returns the Act — its verdict set to VerdictBudgetExceeded and
// its usage recorded — together with an error wrapping ErrBudgetExceeded.
// This is the one case where both return values are non-nil.
func (e *Engine) RunBudgeted(ctx context.Context, intent *domain.Intent, budget *domain.Budget) (*domain.Act, error) {
	act := domain.NewAct(intent.Text)
	act.Pipeline = e.pipelineName
	spent := &tracker{budget: budget}

	e.reporter.Gathering()
	considered, err := e.gatherer.Gather(ctx, intent)
	if err != nil {
		return nil, fmt.Errorf("engine: gather: %w", err)
	}
	act.ConsideredFiles = considered

	rc := runContext{
		executor:     e.executor,
		verifier:     e.verifier,
		workspace:    e.workspace,
		reporter:     e.reporter,
		authority:    e.authority,
		applier:      e.applier,
		checkpointer: e.checkpointer,
		checkpoints:  e.checkpoints,
		spent:        spent,
	}
	if err := e.strategy.Produce(ctx, act, intent, considered, rc); err != nil {
		if errors.Is(err, ErrBudgetExceeded) {
			return act, err
		}
		return nil, err
	}
	return act, nil
}

// ErrCannotResume names why Resume refused to continue an Act.
var ErrCannotResume = errors.New("engine: cannot resume")

// Resume continues act — a checkpoint loaded from record.CheckpointStore,
// left behind by an attempt that was interrupted (crashed or killed)
// before reaching a terminal Judgment — from its first not-yet-completed
// Step, using the same runSteps logic Produce's first attempt used
// (docs/06-open-questions/OQ-008-in-progress-act-persistence.md). It does
// not re-gather Context (act.ConsideredFiles already holds what the
// original attempt gathered) and does not re-charge Budget for Steps that
// already ran.
//
// Resume is inherently Pipeline-shaped — "continue from Step N" only makes
// sense for a Strategy walking a declared Steps sequence — so it requires
// this Engine's configured Strategy to be a PipelineStrategy, wrapping
// ErrCannotResume with a clear reason otherwise. Resuming across a repair
// boundary is out of scope: if the interrupted Step was a failed verify
// Step that would have earned a repair round, Resume's runSteps call simply
// re-confirms that failing verdict via stopsShortOnFailure, exactly as
// Produce's own attempt would, rather than attempting a fresh repair round.
func (e *Engine) Resume(ctx context.Context, act *domain.Act) (*domain.Act, error) {
	strategy, ok := e.strategy.(PipelineStrategy)
	if !ok {
		return nil, fmt.Errorf("%w: act %s: Engine's Strategy is not a PipelineStrategy", ErrCannotResume, act.ID)
	}

	startIdx := len(act.Steps)
	if startIdx >= len(strategy.Pipeline.Steps) {
		return nil, fmt.Errorf("%w: act %s: already reached its last declared step — nothing to resume", ErrCannotResume, act.ID)
	}

	spent := &tracker{budget: DefaultBudget(), iterations: act.Iterations, costUSD: act.CostEstimateUSD}
	rc := runContext{
		executor:     e.executor,
		verifier:     e.verifier,
		workspace:    e.workspace,
		reporter:     e.reporter,
		authority:    e.authority,
		applier:      e.applier,
		checkpointer: e.checkpointer,
		checkpoints:  e.checkpoints,
		spent:        spent,
	}

	outcome, judgment := lastOutcomeAndJudgment(act.Steps)
	intent := &domain.Intent{Text: act.Intent}

	o, j, terminal, err := runSteps(ctx, strategy.Pipeline.Name, act, intent, strategy.Pipeline.Steps[startIdx:], act.ConsideredFiles, outcome, judgment, 0, rc)
	outcome, judgment = o, j
	if err != nil {
		return nil, err
	}
	if terminal {
		return act, nil
	}

	if judgment == nil {
		return nil, fmt.Errorf("engine: pipeline %q declares no verify step: it can never produce a Judgment", strategy.Pipeline.Name)
	}
	act.JudgmentVerdict = judgment.Verdict
	act.CheckedFindings = judgment.Checked
	if err := rc.checkpoints.Delete(ctx, act.ID); err != nil {
		return nil, fmt.Errorf("engine: checkpoint delete: %w", err)
	}
	return act, nil
}
