package engine

import (
	"context"
	"fmt"
	"time"

	"foundry/domain"
)

// Strategy is the pluggable means by which an Act's Outcome and Judgment
// are produced, once Context has been gathered and a Budget tracker is in
// place (docs/02-architecture/execution.md). The Engine owns control flow
// up to invoking a Strategy; a Strategy decides how the work described by
// its Pipeline actually runs — no model ever decides what runs next.
type Strategy interface {
	Produce(ctx context.Context, act *domain.Act, intent *domain.Intent, considered []string, rc runContext) error
}

// runContext bundles what a Strategy needs to produce an Act: the ports to
// call, the workspace to verify against, the Reporter to narrate progress,
// and the Budget tracker enforcing this Act's ceiling.
type runContext struct {
	executor  Executor
	verifier  Verifier
	workspace string
	reporter  Reporter
	spent     *tracker
}

// PipelineStrategy produces an Act by walking a Pipeline's Steps in order,
// re-running the Pipeline (bounded by its RepairPolicy) whenever the final
// verify Step's Judgment is "fail". It is the only Strategy today;
// DefaultPipeline makes it reproduce the Engine's original hardcoded
// lifecycle exactly.
type PipelineStrategy struct {
	Pipeline Pipeline
}

var _ Strategy = PipelineStrategy{}

// Produce runs s.Pipeline's Steps against act, attempting a repair re-run
// (bounded by s.Pipeline.Repair.MaxAttempts) whenever a verify Step's
// Judgment is "fail". A Budget refusal on the first attempt halts: act is
// marked VerdictBudgetExceeded and the refusal is returned as an error
// wrapping ErrBudgetExceeded. A Budget refusal on a repair attempt is not
// an error — the prior attempt's Judgment stands as final.
func (s PipelineStrategy) Produce(ctx context.Context, act *domain.Act, intent *domain.Intent, considered []string, rc runContext) error {
	var outcome *domain.Outcome
	var judgment *domain.Judgment

	for attempt := 0; ; attempt++ {
		if err := rc.spent.charge(executeCostEstimateUSD); err != nil {
			if attempt == 0 {
				rc.reporter.BudgetExceeded(err.Error())
				act.JudgmentVerdict = VerdictBudgetExceeded
				act.Iterations = rc.spent.iterations
				act.CostEstimateUSD = rc.spent.costUSD
				return err
			}
			rc.reporter.RepairSkipped(err.Error())
			break
		}

		if attempt > 0 {
			repaired := make([]string, 0, len(considered)+1)
			repaired = append(repaired, considered...)
			repaired = append(repaired, repairContext(judgment))
			considered = repaired
		}

		for _, step := range s.Pipeline.Steps {
			switch step.Kind {
			case domain.StepKindGenerate:
				rc.reporter.Executing(rc.spent.iterations)
				start := time.Now()
				o, err := rc.executor.Execute(ctx, intent, considered)
				if err != nil {
					return wrapStepError(attempt, "execute", err)
				}
				outcome = o
				act.Patch = outcome.Patch
				act.Iterations = rc.spent.iterations
				act.CostEstimateUSD = rc.spent.costUSD
				act.ConsideredFiles = considered
				recordStep(act, domain.StepKindGenerate, considered, producedPatch(outcome), nil, "", start)

			case domain.StepKindVerify:
				rc.reporter.Verifying(rc.spent.iterations)
				start := time.Now()
				j, err := rc.verifier.Verify(ctx, outcome, rc.workspace)
				if err != nil {
					return wrapStepError(attempt, "verify", err)
				}
				judgment = j
				rc.reporter.Verified(rc.spent.iterations, judgment)
				recordStep(act, domain.StepKindVerify, nil, nil, judgment.Checked, judgment.Verdict, start)
			}
		}

		if judgment.Verdict != verdictFail || attempt >= s.Pipeline.Repair.MaxAttempts {
			break
		}
		rc.reporter.Repairing()
	}

	act.JudgmentVerdict = judgment.Verdict
	act.CheckedFindings = judgment.Checked
	return nil
}

// wrapStepError renders a Step failure, prefixing it as a repair failure
// once the Pipeline is re-running (attempt > 0) — matching the distinct
// error strings ("engine: execute: ..." vs "engine: repair execute: ...")
// the Engine has always produced for the first attempt vs a repair attempt.
func wrapStepError(attempt int, op string, err error) error {
	if attempt > 0 {
		return fmt.Errorf("engine: repair %s: %w", op, err)
	}
	return fmt.Errorf("engine: %s: %w", op, err)
}
