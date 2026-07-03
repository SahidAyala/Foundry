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

// M0.1 budget policy: hardcoded until budgets become configurable (M0.3).
const (
	defaultMaxIterations = 2
	defaultMaxCostUSD    = 1.00

	// executeCostEstimateUSD is charged against Budget.MaxCostUSD for each
	// Executor.Execute call. The Claude Code subprocess exposes no real cost
	// signal, so a flat conservative estimate keeps the cap enforceable
	// until Executors report actual cost.
	executeCostEstimateUSD = 0.50
)

// DefaultBudget returns the hardcoded M0.1 Budget: at most 2 Executor
// iterations and $1.00 of estimated cost per Act.
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
