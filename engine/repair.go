package engine

import (
	"context"
	"fmt"
	"strings"

	"foundry/domain"
)

// verdictFail is the Gate verdict that triggers the bounded repair attempt.
// The architecture reserves a distinct "repair" verdict
// (docs/02-architecture/execution.md step 6); M0's Gate emits only
// pass/fail, so fail is the trigger (backlog PR-011).
const verdictFail = "fail"

// repairOnce is M0's single bounded repair attempt after a failed
// verification: it charges the Budget for one more Executor.Execute call,
// feeds the failed Judgment's findings back to the Executor as additional
// considered context, and re-verifies the new Outcome. Its results replace
// the first attempt's. If the Budget refuses the attempt, the first
// attempt's considered context, outcome, and judgment are returned
// unchanged — exhaustion here is normal control flow, not an error.
func (e *Engine) repairOnce(
	ctx context.Context,
	intent *domain.Intent,
	considered []string,
	outcome *domain.Outcome,
	judgment *domain.Judgment,
	spent *tracker,
) ([]string, *domain.Outcome, *domain.Judgment, error) {
	if err := spent.charge(executeCostEstimateUSD); err != nil {
		return considered, outcome, judgment, nil
	}

	repaired := make([]string, 0, len(considered)+1)
	repaired = append(repaired, considered...)
	repaired = append(repaired, repairContext(judgment))

	newOutcome, err := e.executor.Execute(ctx, intent, repaired)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("engine: repair execute: %w", err)
	}
	newJudgment, err := e.verifier.Verify(ctx, newOutcome, e.workspace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("engine: repair verify: %w", err)
	}
	return repaired, newOutcome, newJudgment, nil
}

// repairContext renders a failed Judgment's checked findings as one
// considered-context entry, so the Executor sees what failed on the
// previous attempt and the Act's Evidence records what the repair saw.
func repairContext(judgment *domain.Judgment) string {
	return "verification findings from the failed previous attempt:\n" +
		strings.Join(judgment.Checked, "\n")
}
