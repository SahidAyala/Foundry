package engine

import (
	"context"
	"fmt"

	"foundry/domain"
	"foundry/model"
)

// FailoverReporter is an optional Reporter extension (ADR-0013, Proposed,
// sixth increment): if the Reporter an Engine is using also implements
// this interface, ModelFailover is called every time automatic model
// failover switches to the next Preferred entry. Deliberately not
// embedded in Reporter itself — adding a new required method there would
// force every existing implementation (cli.ProgressReporter,
// SlogReporter, MultiReporter, noopReporter, and every test fake) to gain
// it just to keep compiling. A Reporter that doesn't implement
// FailoverReporter simply never receives this narration; nothing else
// about it changes.
type FailoverReporter interface {
	// ModelFailover is called after from has failed with a retryable
	// FailureClass and before to is tried, satisfying the requirement to
	// log every switch (e.g. "Claude Sonnet unavailable. Switching to
	// Gemini 3.1 Pro.").
	ModelFailover(stepID, from, to string, class model.FailureClass, cause error)
}

// reportFailover calls reporter.ModelFailover if reporter implements
// FailoverReporter, and does nothing otherwise.
func reportFailover(reporter Reporter, stepID, from, to string, class model.FailureClass, cause error) {
	if fr, ok := reporter.(FailoverReporter); ok {
		fr.ModelFailover(stepID, from, to, class, cause)
	}
}

// executeGenerateStep resolves and calls Execute for a generate Step,
// applying automatic model failover (ADR-0013, Proposed, sixth increment)
// when step.Preferred names more than one model.
//
// Behavior, precisely:
//   - step.Preferred is empty (or has exactly one entry with nothing left
//     to fail over to): a single Resolve/estimate/charge/Execute
//     sequence, byte-for-byte the same as before this increment existed
//     — "supported only when preferred[] is configured."
//   - step.Preferred has more than one entry: if the first entry's
//     Execute call fails with a FailureClass that Retryable() reports
//     true for (rate limit, temporary unavailable, timeout), the next
//     entry is resolved (via Router.ResolveModel) and tried instead,
//     after calling reportFailover to log the switch. Any other failure —
//     unclassified, or explicitly non-retryable (authentication, invalid
//     model, unsupported capability) — fails the Step immediately, never
//     trying a later entry. The last entry failing retryably still fails
//     the Step: there is nothing further to try.
//
// Cost estimation and Budget charging happen once per actual Execute call
// (including retries after a failover) — a different model can have a
// genuinely different cost, so each attempt is estimated and charged on
// its own terms, exactly as the pre-failover code already charged once
// per Execute call.
func executeGenerateStep(ctx context.Context, rc runContext, step Step, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
	resolved, err := rc.router.Resolve(step)
	if err != nil {
		return nil, wrapStepError(attempt, "route", err)
	}

	currentModel := ""
	var remaining []string
	if len(step.Preferred) > 0 {
		currentModel = step.Preferred[0]
		remaining = step.Preferred[1:]
	}

	for {
		estimate, err := estimateExecuteCostUSD(ctx, resolved, intent, considered)
		if err != nil {
			return nil, wrapStepError(attempt, "estimate", err)
		}
		if err := rc.spent.charge(estimate); err != nil {
			// Not wrapStepError: Produce reports this verbatim via
			// reporter.BudgetExceeded/RepairSkipped, and charge's own
			// message ("budget exceeded: ...") is already the whole
			// story. A Budget refusal is never a model failure, so it's
			// never eligible for failover either.
			return nil, err
		}
		rc.reporter.Executing(attempt + 1)

		outcome, execErr := resolved.Execute(ctx, intent, considered)
		if execErr == nil {
			rc.reporter.Executed(attempt+1, outcome.ActualCostUSD)
			return outcome, nil
		}

		if len(remaining) == 0 {
			return nil, wrapStepError(attempt, "execute", execErr)
		}
		class, classified := model.ClassifyFailure(execErr)
		if !classified || !class.Retryable() {
			return nil, wrapStepError(attempt, "execute", execErr)
		}

		next := remaining[0]
		nextResolved, resolveErr := rc.router.ResolveModel(next)
		if resolveErr != nil {
			return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, resolveErr))
		}

		reportFailover(rc.reporter, step.ID, currentModel, next, class, execErr)

		resolved = nextResolved
		currentModel = next
		remaining = remaining[1:]
	}
}
