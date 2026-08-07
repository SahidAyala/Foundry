package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

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
// executeGenerateStep, with one addition (the tenth increment,
// preferHealthyCandidates): step.Preferred is empty runs a single
// Resolve/estimate/charge/Execute sequence, byte-for-byte the same as
// before automatic failover existed ("supported only when preferred[] is
// configured"); step.Preferred with two or more entries fails over on a
// retryable Execute failure, exactly as documented on runExecuteAttempts,
// after first being reordered so any entry known to be currently
// Unavailable sorts behind every other entry — connecting HealthManager
// to failover so a known-down model is skipped preemptively, not only
// after a real call to it fails.
func executeGenerateStepWithoutCapabilityCheck(ctx context.Context, rc runContext, step Step, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
	if len(step.Preferred) == 0 {
		resolved, err := rc.router.Resolve(step)
		if err != nil {
			return nil, wrapStepError(attempt, "route", err)
		}
		return runExecuteAttempts(ctx, rc, step.ID, resolved, "", executorLabel(step.Model, step.Executor), nil, intent, considered, attempt)
	}

	ordered := preferHealthyCandidates(rc.router, step.Preferred)
	resolved, err := rc.router.ResolveModel(ordered[0])
	if err != nil {
		return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, err))
	}
	return runExecuteAttempts(ctx, rc, step.ID, resolved, ordered[0], executorLabel(step.Model, step.Executor), ordered[1:], intent, considered, attempt)
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
		return runExecuteAttempts(ctx, rc, step.ID, resolved, "", executorLabel(step.Model, step.Executor), nil, intent, considered, attempt)
	}

	capable, err := filterCapableCandidates(rc.router, candidates, step.RequireCapabilities)
	if err != nil {
		return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, err))
	}
	capable = preferHealthyCandidates(rc.router, capable)

	resolved, err := rc.router.ResolveModel(capable[0])
	if err != nil {
		return nil, wrapStepError(attempt, "route", fmt.Errorf("step %q: %w", step.ID, err))
	}
	return runExecuteAttempts(ctx, rc, step.ID, resolved, capable[0], executorLabel(step.Model, step.Executor), capable[1:], intent, considered, attempt)
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

// preferHealthyCandidates reorders candidates (the tenth increment,
// connecting model.HealthManager to failover and capability filtering —
// the gap every earlier increment's own Consequences named) so that any
// candidate router.ModelHealth reports Unavailable() sorts behind every
// candidate it doesn't, preserving each group's own relative order — a
// stable partition, not a filter. Unlike filterCapableCandidates, this
// never drops a candidate entirely: a HealthManager report is one
// Executor's own observation, which can go stale (a model marked down an
// hour ago may well be back), unlike static Capabilities data, which is a
// hard, load-bearing requirement. If every candidate is currently
// Unavailable, this is a no-op — Foundry still attempts the Step's first
// declared candidate rather than refusing outright over health data that
// might no longer be true. Called once, up front, the same "no dynamic
// re-evaluation mid-loop" shape filterCapableCandidates already
// established — not re-run after each failover switch.
func preferHealthyCandidates(router Router, candidates []string) []string {
	if len(candidates) < 2 {
		return candidates
	}
	ordered := make([]string, 0, len(candidates))
	var unavailable []string
	for _, id := range candidates {
		if router.ModelHealth(id).Unavailable() {
			unavailable = append(unavailable, id)
			continue
		}
		ordered = append(ordered, id)
	}
	return append(ordered, unavailable...)
}

// executorLabel names, for narration only, what a generate Step's Execute
// call is about to run against: the Model ID when one was resolved
// (ADR-0013's Model/Preferred routing), otherwise the Step's own Executor
// pin, otherwise the Engine's default Executor — which has no name to
// report, since it is wired in Go by the composition root rather than
// named by a Pipeline document. It never affects resolution; it exists
// because narration that hardcodes one vendor's name (as
// cli.ProgressReporter's "Asking Claude Code for a patch" did, for every
// Executor including Gemini and OpenAI ones) is worse than no name at all.
func executorLabel(modelID, pin string) string {
	switch {
	case modelID != "":
		return modelID
	case pin != "":
		return pin
	}
	return "default executor"
}

// timeoutAdvertiser is the optional seam an Executor may implement to
// report its own per-call deadline — every CLI-backed Executor in this
// repository does, via the SetTimeout/Timeout pair. It is an assertion
// rather than part of the Executor port because a deadline is not
// something the Engine needs in order to run a Step: it is used purely to
// tell a human how much of the budgeted wait has elapsed.
type timeoutAdvertiser interface {
	Timeout() time.Duration
}

// executorTimeout returns the per-call deadline e advertises, or 0 if it
// advertises none.
func executorTimeout(e Executor) time.Duration {
	if t, ok := e.(timeoutAdvertiser); ok {
		return t.Timeout()
	}
	return 0
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
// fallbackLabel names what is running (for narration only) whenever no
// Model ID applies to the current attempt — see executorLabel, which the
// callers use to build it. It plays no part in resolution, which already
// happened in the caller.
func runExecuteAttempts(ctx context.Context, rc runContext, stepID string, resolved Executor, currentModel, fallbackLabel string, remaining []string, intent *domain.Intent, considered []string, attempt int) (*domain.Outcome, error) {
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

		// The Execute call below is where an Act spends nearly all of its
		// wall-clock time (minutes, for a CLI-backed Executor), and until
		// it was bracketed by these two events a caller had nothing to
		// show for that whole window: not which model was resolved, not
		// how long the call had been running, and — on failure — not the
		// cause, until it surfaced as the Act's own error much later.
		label := currentModel
		if label == "" {
			label = fallbackLabel
		}
		reportExecutorStarting(rc.reporter, stepID, label, executorTimeout(resolved))
		start := time.Now()
		outcome, execErr := resolved.Execute(ctx, intent, considered)
		reportExecutorFinished(rc.reporter, stepID, label, time.Since(start), execErr)
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
