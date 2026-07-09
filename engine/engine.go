// Package engine drives the machine-owned production of an Act: it gathers
// Context, enforces Budget, and delegates producing the Act's Outcome and
// Judgment to a Strategy (strategy.go), which today always walks
// DefaultPipeline (step.go) — the Engine no longer hardcodes repair as
// bespoke Go control flow (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md
// §9 Phase 2). The accountable steps that follow — an Authority's
// acceptance, applying the Outcome, and recording the Act — happen at the
// trust boundary above the Engine (see docs/02-architecture/execution.md
// steps 5–8); the Engine has never known about them.
package engine

import (
	"context"
	"errors"
	"fmt"

	"foundry/domain"
)

// Engine produces machine-judged Acts.
type Engine struct {
	gatherer  Gatherer
	executor  Executor
	verifier  Verifier
	workspace string // directory the Verifier checks
	reporter  Reporter
	strategy  Strategy
}

// NewEngine wires the ports an Engine needs to produce an Act, using
// PipelineStrategy over DefaultPipeline — today's only Strategy and
// Pipeline. workspace is the directory the Verifier checks; for M0.0 this
// is the repository path.
func NewEngine(gatherer Gatherer, executor Executor, verifier Verifier, workspace string) *Engine {
	return &Engine{
		gatherer:  gatherer,
		executor:  executor,
		verifier:  verifier,
		workspace: workspace,
		reporter:  noopReporter{},
		strategy:  PipelineStrategy{Pipeline: DefaultPipeline()},
	}
}

// SetReporter attaches r as the Engine's progress observer, replacing the
// default no-op. Reporter is optional and additive: it never changes what
// Run or RunBudgeted does, only what a caller can observe while it runs.
func (e *Engine) SetReporter(r Reporter) {
	e.reporter = r
}

// Run gathers context, executes the work, and verifies the outcome under the
// default M0.1 Budget, returning an Act with its considered context, proposed
// patch, and machine verdict. Run does not seek approval, apply, or record;
// those are the caller's responsibility at the trust boundary.
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
	spent := &tracker{budget: budget}

	e.reporter.Gathering()
	considered, err := e.gatherer.Gather(ctx, intent)
	if err != nil {
		return nil, fmt.Errorf("engine: gather: %w", err)
	}
	act.ConsideredFiles = considered

	rc := runContext{
		executor:  e.executor,
		verifier:  e.verifier,
		workspace: e.workspace,
		reporter:  e.reporter,
		spent:     spent,
	}
	if err := e.strategy.Produce(ctx, act, intent, considered, rc); err != nil {
		if errors.Is(err, ErrBudgetExceeded) {
			return act, err
		}
		return nil, err
	}
	return act, nil
}
