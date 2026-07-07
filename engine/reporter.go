package engine

// Reporter observes an Act's lifecycle as the Engine runs it — pure
// narration, never control flow. Only the Engine decides what runs next
// (I1); a Reporter is told what already happened so a caller can show
// progress, and must never be able to influence the Judgment or the Act.
type Reporter interface {
	// Gathering is called once, before the Gatherer runs.
	Gathering()
	// Executing is called before an Executor.Execute call. iteration is
	// 1 for the first attempt, 2 for the bounded repair.
	Executing(iteration int)
	// Verifying is called before a Verifier.Verify call for the same
	// iteration just executed.
	Verifying(iteration int)
	// Verified is called after Verify returns a Judgment for iteration.
	Verified(iteration int, verdict string)
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

func (noopReporter) Gathering()                       {}
func (noopReporter) Executing(iteration int)          {}
func (noopReporter) Verifying(iteration int)          {}
func (noopReporter) Verified(iteration int, s string) {}
func (noopReporter) Repairing()                       {}
func (noopReporter) RepairSkipped(reason string)      {}
func (noopReporter) BudgetExceeded(reason string)     {}

var _ Reporter = noopReporter{}
