package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/model"
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
// when step.Preferred names more than one model, and capability-aware
// model resolution (ADR-0013, Proposed, seventh increment) when
// step.RequireCapabilities is set.
//
// When step.RequireCapabilities is empty, this delegates to
// executeGenerateStepWithoutCapabilityCheck — byte-for-byte the sixth
// increment's own behavior, untouched. When it is set, capability
// filtering (see filterCapableCandidates) happens once, up front, against
// static Model Registry catalog data, before any Execute call — a Step
// never even attempts a model already known unable to do what it needs.
func executeGenerateStep(ctx context.Context, rc runContext, step Step, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
	if len(step.RequireCapabilities) == 0 {
		return executeGenerateStepWithoutCapabilityCheck(ctx, rc, step, intent, considered, attempt)
	}
	return executeGenerateStepWithCapabilityCheck(ctx, rc, step, intent, considered, attempt)
}

// executeGenerateStepWithoutCapabilityCheck is the sixth increment's own
// executeGenerateStep, unchanged: step.Preferred is empty (or has exactly
// one entry with nothing left to fail over to) runs a single
// Resolve/estimate/charge/Execute sequence, byte-for-byte the same as
// before automatic failover existed ("supported only when preferred[] is
// configured"); step.Preferred with more than one entry fails over on a
// retryable Execute failure, exactly as documented on runExecuteAttempts.
func executeGenerateStepWithoutCapabilityCheck(ctx context.Context, rc runContext, step Step, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
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

	return runExecuteAttempts(ctx, rc, step.ID, resolved, currentModel, remaining, intent, considered, attempt)
}

// executeGenerateStepWithCapabilityCheck implements capability-aware
// model resolution (ADR-0013, Proposed, seventh increment): instead of
// simply trying Model/Preferred's declared order, only a candidate whose
// registered Capabilities satisfy every name in step.RequireCapabilities
// may be selected at all. Capability compatibility is verified once,
// up front, using only static Model Registry catalog data (no network
// probe — no different in kind from the static "unknown model" check
// Resolve already performs) — if no candidate qualifies, this returns a
// validation error immediately, before any Execute call is attempted.
// Once at least one candidate qualifies, automatic failover
// (runExecuteAttempts) runs exactly the same way, but only ever moves to
// a later *capable* candidate — one already excluded for lacking a
// required capability is never tried, even as a failover target.
//
// A Step naming neither Model nor Preferred has nothing to check
// Capabilities against (Capabilities live on model.Info, keyed by Model
// ID, not on a plain Executor pin) — RequireCapabilities has no effect in
// that case, and resolution proceeds exactly as it always did for an
// Executor-only Step.
func executeGenerateStepWithCapabilityCheck(ctx context.Context, rc runContext, step Step, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
	candidates := step.Preferred
	if len(candidates) == 0 && step.Model != "" {
		candidates = []string{step.Model}
	}
	if len(candidates) == 0 {
		resolved, err := rc.router.Resolve(step)
		if err != nil {
			return nil, wrapStepError(attempt, "route", err)
		}
		return runExecuteAttempts(ctx, rc, step.ID, resolved, "", nil, intent, considered, attempt)
	}

	capable, err := filterCapableCandidates(rc.router, candidates, step.RequireCapabilities)
	if err != nil {
		return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, err))
	}

	resolved, err := rc.router.ResolveModel(capable[0])
	if err != nil {
		return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, err))
	}
	return runExecuteAttempts(ctx, rc, step.ID, resolved, capable[0], capable[1:], intent, considered, attempt)
}

// filterCapableCandidates returns the subset of candidates (in their
// given order) whose registered model.Info.Capabilities satisfies every
// name in required. A candidate whose Model ID isn't registered in the
// Model Registry at all is excluded too — its Capabilities can't be
// confirmed, so it can't be confirmed compatible either. Returns a clear,
// named validation error — before any Execute call is ever attempted — if
// no candidate qualifies.
func filterCapableCandidates(router Router, candidates []string, required []string) ([]string, error) {
	var capable []string
	for _, id := range candidates {
		info, err := router.ModelInfo(id)
		if err != nil {
			continue
		}
		if ok, _ := info.Capabilities.Supports(required); ok {
			capable = append(capable, id)
		}
	}
	if len(capable) == 0 {
		return nil, fmt.Errorf("no candidate model (%s) supports all required capabilities (%s)", strings.Join(candidates, ", "), strings.Join(required, ", "))
	}
	return capable, nil
}

// runExecuteAttempts runs the shared estimate/charge/Executing/Execute/
// failover loop given an already-resolved starting Executor (resolved),
// its own Model ID for narration (currentModel, "" if none applies), and
// the ordered list of further Model IDs to fail over to on a retryable
// failure (remaining, each resolved via Router.ResolveModel as it's
// tried). Cost estimation and Budget charging happen once per actual
// Execute call, including retries — a different model can have a
// genuinely different cost, so each attempt is estimated and charged on
// its own terms, exactly as the pre-failover code already charged once
// per Execute call.
func runExecuteAttempts(ctx context.Context, rc runContext, stepID string, resolved Executor, currentModel string, remaining []string, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
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
			return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", stepID, resolveErr))
		}

		reportFailover(rc.reporter, stepID, currentModel, next, class, execErr)

		resolved = nextResolved
		currentModel = next
		remaining = remaining[1:]
	}
}
