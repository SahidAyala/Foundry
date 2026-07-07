// Package engine runs the machine-owned steps of the Act lifecycle: gather,
// execute, and verify. It produces an Act carrying a machine verdict. The
// accountable steps that follow — an Authority's acceptance, applying the
// Outcome, and recording the Act — happen at the trust boundary above the
// Engine (see docs/02-architecture/execution.md steps 5–8).
package engine

import (
	"context"
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
}

// NewEngine wires the ports an Engine needs to produce an Act. workspace is
// the directory the Verifier checks; for M0.0 this is the repository path.
func NewEngine(gatherer Gatherer, executor Executor, verifier Verifier, workspace string) *Engine {
	return &Engine{
		gatherer:  gatherer,
		executor:  executor,
		verifier:  verifier,
		workspace: workspace,
		reporter:  noopReporter{},
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

// RunBudgeted is Run under an explicit Budget: every Executor.Execute call is
// charged against the Budget's iteration and cost ceilings before it runs.
// A failed verification triggers at most one repair attempt (repair.go),
// itself charged against the same Budget; the repair's judgment is final.
// If a call would exceed the Budget, the Engine halts and returns the Act —
// its verdict set to VerdictBudgetExceeded and its usage recorded — together
// with an error wrapping ErrBudgetExceeded. This is the one case where both
// return values are non-nil.
func (e *Engine) RunBudgeted(ctx context.Context, intent *domain.Intent, budget *domain.Budget) (*domain.Act, error) {
	act := domain.NewAct(intent.Text)
	spent := &tracker{budget: budget}

	e.reporter.Gathering()
	considered, err := e.gatherer.Gather(ctx, intent)
	if err != nil {
		return nil, fmt.Errorf("engine: gather: %w", err)
	}
	act.ConsideredFiles = considered

	if err := spent.charge(executeCostEstimateUSD); err != nil {
		e.reporter.BudgetExceeded(err.Error())
		act.JudgmentVerdict = VerdictBudgetExceeded
		act.Iterations = spent.iterations
		act.CostEstimateUSD = spent.costUSD
		return act, fmt.Errorf("engine: execute: %w", err)
	}
	e.reporter.Executing(spent.iterations)
	outcome, err := e.executor.Execute(ctx, intent, considered)
	if err != nil {
		return nil, fmt.Errorf("engine: execute: %w", err)
	}
	act.Patch = outcome.Patch
	act.Iterations = spent.iterations
	act.CostEstimateUSD = spent.costUSD

	e.reporter.Verifying(spent.iterations)
	judgment, err := e.verifier.Verify(ctx, outcome, e.workspace)
	if err != nil {
		return nil, fmt.Errorf("engine: verify: %w", err)
	}
	e.reporter.Verified(spent.iterations, judgment)

	// Bounded repair (M0.2): a failed verification earns exactly one more
	// Execute, budget permitting, with the findings fed back as context.
	if judgment.Verdict == verdictFail {
		e.reporter.Repairing()
		considered, outcome, judgment, err = e.repairOnce(ctx, intent, considered, outcome, judgment, spent)
		if err != nil {
			return nil, err
		}
		act.ConsideredFiles = considered
		act.Patch = outcome.Patch
		act.Iterations = spent.iterations
		act.CostEstimateUSD = spent.costUSD
	}
	act.JudgmentVerdict = judgment.Verdict

	return act, nil
}
