package engine

import (
	"errors"
	"fmt"

	"foundry/domain"
)

// ErrBudgetExceeded reports that producing an Act would break its Budget.
// The Act returned alongside it carries VerdictBudgetExceeded.
var ErrBudgetExceeded = errors.New("budget exceeded")

// VerdictBudgetExceeded is the Judgment verdict recorded when an Act halts
// because its Budget was exhausted.
const VerdictBudgetExceeded = "budget-exceeded"

// Hardcoded until budgets become configurable (roadmap.md open decision 9,
// "cost as a first-class constraint"). Sized to cover a Pipeline with more
// than one generate Step per attempt now that Budget charges per Step, not
// per attempt (RFC-0004 §2.7): feature.json's worst case is plan +
// implement + two repair rounds of implement — 4 Executor calls — which
// must all fit under these ceilings at the flat executeCostEstimateUSD rate
// for the repair capability it declares (repair.max_attempts: 2) to ever be
// reachable.
const (
	defaultMaxIterations = 4
	defaultMaxCostUSD    = 2.00

	// executeCostEstimateUSD is the fallback charged against
	// Budget.MaxCostUSD for an Executor.Execute call whose Executor does
	// not implement CostEstimator (estimateExecuteCostUSD,
	// cost_estimator.go) — today, executor/claude.ClaudeExecutor and
	// executor.ScriptedExecutor. The Claude Code subprocess exposes no real
	// cost signal, so a flat conservative estimate keeps the cap
	// enforceable until it does.
	executeCostEstimateUSD = 0.50
)

// DefaultBudget returns the hardcoded default Budget: at most 4 Executor
// calls and $2.00 of estimated cost per Act.
func DefaultBudget() *domain.Budget {
	return &domain.Budget{
		MaxIterations: defaultMaxIterations,
		MaxCostUSD:    defaultMaxCostUSD,
	}
}

// tracker enforces one Act's Budget across its Executor.Execute calls.
type tracker struct {
	budget     *domain.Budget
	iterations int
	costUSD    float64
}

// charge accounts for one Executor.Execute call estimated at estimateUSD.
// It returns an error wrapping ErrBudgetExceeded — without consuming the
// budget — if the call would exceed the iteration or cost ceiling.
func (t *tracker) charge(estimateUSD float64) error {
	if t.iterations+1 > t.budget.MaxIterations {
		return fmt.Errorf("%w: iteration %d over limit %d",
			ErrBudgetExceeded, t.iterations+1, t.budget.MaxIterations)
	}
	if t.costUSD+estimateUSD > t.budget.MaxCostUSD {
		return fmt.Errorf("%w: estimated cost $%.2f over limit $%.2f",
			ErrBudgetExceeded, t.costUSD+estimateUSD, t.budget.MaxCostUSD)
	}
	t.iterations++
	t.costUSD += estimateUSD
	return nil
}
