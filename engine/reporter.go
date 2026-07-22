package engine

import "foundry/domain"

// Reporter observes an Act's lifecycle as the Engine runs it — pure
// narration, never control flow. Only the Engine decides what runs next
// (I1); a Reporter is told what already happened so a caller can show
// progress, and must never be able to influence the Judgment or the Act.
type Reporter interface {
	// Gathering is called once, before the Gatherer runs.
	Gathering()
	// Executing is called before an Executor.Execute call. iteration is
	// 1 for the first attempt, 2 for the bounded repair — the Pipeline
	// attempt number, not a count of Executor calls (a single attempt may
	// call Execute more than once, e.g. feature.json's plan + implement).
	Executing(iteration int)
	// Verifying is called before a Verifier.Verify call for the same
	// attempt just executed.
	Verifying(iteration int)
	// Verified is called with the Judgment Verify returned for the same
	// attempt — including its Checked findings, so a caller can show why a
	// verdict failed, not only that it did.
	Verified(iteration int, judgment *domain.Judgment)
	// Executed is called once an Executor.Execute call succeeds, with the
	// real, post-execution cost it reported (ADR-0011,
	// docs/03-adrs/ADR-0011-cost-as-a-first-class-constraint.md) — nil if
	// the Executor could not report one. Purely additional reporting; the
	// Budget decision to allow the call already happened in Executing's
	// pre-execution estimate.
	Executed(iteration int, actualCostUSD *float64)
	// Repairing is called once, when a failed first verification earns
	// the bounded repair attempt.
	Repairing()
	// RepairSkipped is called instead of a second Executing/Verifying
	// round when the Budget refuses the repair attempt.
	RepairSkipped(reason string)
	// BudgetExceeded is called when the Budget halts the Act before its
	// first Execute call.
	BudgetExceeded(reason string)
}

// noopReporter discards every event. It is the Engine's default so a
// Reporter is optional: nothing observes an Act unless SetReporter is
// called.
type noopReporter struct{}

func (noopReporter) Gathering()                                        {}
func (noopReporter) Executing(iteration int)                           {}
func (noopReporter) Verifying(iteration int)                           {}
func (noopReporter) Verified(iteration int, judgment *domain.Judgment) {}
func (noopReporter) Executed(iteration int, actualCostUSD *float64)    {}
func (noopReporter) Repairing()                                        {}
func (noopReporter) RepairSkipped(reason string)                       {}
func (noopReporter) BudgetExceeded(reason string)                      {}

var _ Reporter = noopReporter{}

// MultiReporter fans every event out to each of Reporters, in order — the
// same small composition seam already established for one port at a time
// by ExecutorRegistry/Router, ApplierRegistry, and gatherer.Compose, applied
// here so more than one Reporter (e.g. a human-facing ProgressReporter and a
// structured SlogReporter) can observe the same Engine run without
// SetReporter's single-field contract changing. A Reporter is pure
// narration (I1) — fanning out to several changes nothing about what the
// Engine decides.
type MultiReporter struct {
	Reporters []Reporter
}

func (m MultiReporter) Gathering() {
	for _, r := range m.Reporters {
		r.Gathering()
	}
}

func (m MultiReporter) Executing(iteration int) {
	for _, r := range m.Reporters {
		r.Executing(iteration)
	}
}

func (m MultiReporter) Verifying(iteration int) {
	for _, r := range m.Reporters {
		r.Verifying(iteration)
	}
}

func (m MultiReporter) Verified(iteration int, judgment *domain.Judgment) {
	for _, r := range m.Reporters {
		r.Verified(iteration, judgment)
	}
}

func (m MultiReporter) Executed(iteration int, actualCostUSD *float64) {
	for _, r := range m.Reporters {
		r.Executed(iteration, actualCostUSD)
	}
}

func (m MultiReporter) Repairing() {
	for _, r := range m.Reporters {
		r.Repairing()
	}
}

func (m MultiReporter) RepairSkipped(reason string) {
	for _, r := range m.Reporters {
		r.RepairSkipped(reason)
	}
}

func (m MultiReporter) BudgetExceeded(reason string) {
	for _, r := range m.Reporters {
		r.BudgetExceeded(reason)
	}
}

var _ Reporter = MultiReporter{}
