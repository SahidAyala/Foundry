package engine

import (
	"log/slog"

	"foundry/domain"
)

// SlogReporter implements Reporter by emitting structured, leveled log
// events at Engine lifecycle boundaries — observability distinct from
// ProgressReporter's human narration (roadmap.md M5's named,
// previously-unstarted "observability" gap). Like every Reporter, it is
// pure narration (I1): it observes what the Engine already decided and
// never influences it. Compose it alongside a human-facing Reporter via
// MultiReporter; SlogReporter alone has no notion of a terminal, color, or
// a live prompt.
//
// It does not identify the Act being run — Reporter itself carries no Act
// ID (Gathering, Executing, Verifying, ... never receive one; see
// engine/reporter.go), so neither does ProgressReporter today. Attach one
// SlogReporter per Engine run (the same lifetime ProgressReporter already
// has) if per-Act correlation in the log stream matters to a caller; that
// is a composition-root concern, not one this type solves.
type SlogReporter struct {
	logger *slog.Logger
}

// NewSlogReporter returns a Reporter that logs each lifecycle event to
// logger: Info for the normal-path events, Warn for RepairSkipped and
// BudgetExceeded (both are the Engine declining to continue, worth a
// human's attention in a log stream even though neither is itself an
// error).
func NewSlogReporter(logger *slog.Logger) *SlogReporter {
	return &SlogReporter{logger: logger}
}

var _ Reporter = (*SlogReporter)(nil)

func (r *SlogReporter) Gathering() {
	r.logger.Info("act.gather.start")
}

func (r *SlogReporter) Executing(iteration int) {
	r.logger.Info("act.execute.start", "iteration", iteration)
}

func (r *SlogReporter) Executed(iteration int, actualCostUSD *float64) {
	if actualCostUSD == nil {
		r.logger.Info("act.execute.done", "iteration", iteration)
		return
	}
	r.logger.Info("act.execute.done", "iteration", iteration, "actual_cost_usd", *actualCostUSD)
}

func (r *SlogReporter) Verifying(iteration int) {
	r.logger.Info("act.verify.start", "iteration", iteration)
}

func (r *SlogReporter) Verified(iteration int, judgment *domain.Judgment) {
	r.logger.Info("act.verify.done",
		"iteration", iteration,
		"verdict", judgment.Verdict,
		"findings", len(judgment.Checked),
	)
}

func (r *SlogReporter) Repairing() {
	r.logger.Info("act.repair.start")
}

func (r *SlogReporter) RepairSkipped(reason string) {
	r.logger.Warn("act.repair.skipped", "reason", reason)
}

func (r *SlogReporter) BudgetExceeded(reason string) {
	r.logger.Warn("act.budget.exceeded", "reason", reason)
}
