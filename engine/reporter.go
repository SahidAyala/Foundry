package engine

import (
	"io"
	"time"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/model"
)

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
	// Repairing is called once per repair round the Engine grants, with
	// the reason the previous attempt did not produce a usable Outcome:
	// a failed verify Judgment is one cause, but a generate Step that
	// failed outright (a timed-out or unparseable Executor call) is
	// another, and both spend the same repair budget
	// (engine/strategy.go's generateStepFailure). The reason is passed
	// rather than assumed because a Reporter cannot otherwise tell the
	// two apart — every repair round used to be narrated as a
	// verification failure, including the rounds where verification
	// never ran at all.
	Repairing(reason string)
	// RepairSkipped is called instead of a second Executing/Verifying
	// round when the Budget refuses the repair attempt.
	RepairSkipped(reason string)
	// BudgetExceeded is called when the Budget halts the Act before its
	// first Execute call.
	BudgetExceeded(reason string)
}

// StepReporter is an optional Reporter extension: if the Reporter an
// Engine is using also implements it, StepStarting is called before every
// Step of every attempt, whatever its kind — so a caller can narrate the
// Pipeline walk itself ("step 2 of 5: verify"), not only the
// generate/verify events Reporter already carries. Like FailoverReporter
// (engine/failover.go), it is deliberately not embedded in Reporter: a
// Reporter that doesn't implement it simply never receives this
// narration.
type StepReporter interface {
	// StepStarting is called before a Step runs. attempt is the Pipeline
	// attempt number (1 for the first, 2 for the first repair round);
	// index is the Step's 1-based position within the Steps this attempt
	// is walking, out of total. A repair round that jumps to
	// RepairPolicy.Target walks fewer Steps than the first attempt, so
	// index/total describe this attempt, not the whole Pipeline document.
	StepStarting(attempt, index, total int, stepID, kind string)
}

// ExecutorReporter is an optional Reporter extension that brackets the one
// call an Act spends nearly all of its wall-clock time inside: an
// Executor's Execute. Reporter.Executing already announces that a call is
// about to happen, but it carries neither which Executor was resolved nor
// when the call ended, so a caller had no way to narrate a long-running
// call's progress, name the model actually being used, or report the
// failure at the moment it happened. ExecutorFinished is called on both
// success and failure, so a Reporter holding live state for the duration
// of the call (cli.ProgressReporter's elapsed-time heartbeat) always gets
// a definite end.
type ExecutorReporter interface {
	// ExecutorStarting is called immediately before Execute. executor
	// names what was resolved — a Model ID when the Step declared one,
	// otherwise the Step's Executor pin, otherwise the Engine's default.
	// timeout is the per-call deadline the resolved Executor advertises,
	// or 0 if it advertises none.
	ExecutorStarting(stepID, executor string, timeout time.Duration)
	// ExecutorFinished is called once Execute returns, with how long it
	// took and the error it returned (nil on success).
	ExecutorFinished(stepID, executor string, elapsed time.Duration, err error)
}

// reportStepStarting calls reporter.StepStarting if reporter implements
// StepReporter, and does nothing otherwise.
func reportStepStarting(reporter Reporter, attempt, index, total int, stepID, kind string) {
	if sr, ok := reporter.(StepReporter); ok {
		sr.StepStarting(attempt, index, total, stepID, kind)
	}
}

// reportExecutorStarting calls reporter.ExecutorStarting if reporter
// implements ExecutorReporter, and does nothing otherwise.
func reportExecutorStarting(reporter Reporter, stepID, executor string, timeout time.Duration) {
	if er, ok := reporter.(ExecutorReporter); ok {
		er.ExecutorStarting(stepID, executor, timeout)
	}
}

// reportExecutorFinished calls reporter.ExecutorFinished if reporter
// implements ExecutorReporter, and does nothing otherwise.
func reportExecutorFinished(reporter Reporter, stepID, executor string, elapsed time.Duration, err error) {
	if er, ok := reporter.(ExecutorReporter); ok {
		er.ExecutorFinished(stepID, executor, elapsed, err)
	}
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
func (noopReporter) Repairing(reason string)                           {}
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

func (m MultiReporter) Repairing(reason string) {
	for _, r := range m.Reporters {
		r.Repairing(reason)
	}
}

// StepStarting, ExecutorStarting, ExecutorFinished, and ModelFailover fan
// the optional Reporter extensions out too, to whichever Reporters
// implement each one. Without these, composing a ProgressReporter with a
// SlogReporter (cli.NewReporter, whenever FOUNDRY_LOG is set) would
// silently drop every event that lives on an extension interface rather
// than on Reporter itself — the MultiReporter satisfies only Reporter, so
// the type assertions in reportStepStarting/reportExecutorStarting/
// reportFailover would all fail against it and narrate nothing at all.
func (m MultiReporter) StepStarting(attempt, index, total int, stepID, kind string) {
	for _, r := range m.Reporters {
		reportStepStarting(r, attempt, index, total, stepID, kind)
	}
}

func (m MultiReporter) ExecutorStarting(stepID, executor string, timeout time.Duration) {
	for _, r := range m.Reporters {
		reportExecutorStarting(r, stepID, executor, timeout)
	}
}

func (m MultiReporter) ExecutorFinished(stepID, executor string, elapsed time.Duration, err error) {
	for _, r := range m.Reporters {
		reportExecutorFinished(r, stepID, executor, elapsed, err)
	}
}

func (m MultiReporter) ModelFailover(stepID, from, to string, class model.FailureClass, cause error) {
	for _, r := range m.Reporters {
		reportFailover(r, stepID, from, to, class, cause)
	}
}

// Close releases whatever any composed Reporter holds open — today, a
// ProgressReporter's live-elapsed heartbeat goroutine — for the
// Reporters that implement io.Closer, and ignores the rest. Returns the
// first error any of them reported, after closing all of them.
func (m MultiReporter) Close() error {
	var firstErr error
	for _, r := range m.Reporters {
		if c, ok := r.(io.Closer); ok {
			if err := c.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
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
