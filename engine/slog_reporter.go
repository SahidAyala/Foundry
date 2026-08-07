package engine

import (
	"log/slog"
	"time"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/model"
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

func (r *SlogReporter) Repairing(reason string) {
	r.logger.Info("act.repair.start", "reason", reason)
}

// StepStarting, ExecutorStarting, and ExecutorFinished implement the
// optional Reporter extensions (engine/reporter.go), so the structured log
// stream carries the Pipeline walk and the Execute call's own boundaries —
// the two things that were previously invisible for the entire minutes-long
// window an Executor runs in. ModelFailover (FailoverReporter) is
// implemented for the same reason: it was already emitted by the Engine but
// no Reporter in this repository listened for it.
func (r *SlogReporter) IntentDeclared(text string) {
	r.logger.Info("act.intent", "text", text)
}

func (r *SlogReporter) ContextGathered(entries, bytes int) {
	r.logger.Info("act.gather.done", "entries", entries, "bytes", bytes)
}

func (r *SlogReporter) StepStarting(attempt, index, total int, stepID, kind string) {
	r.logger.Info("act.step.start",
		"attempt", attempt,
		"index", index,
		"total", total,
		"step_id", stepID,
		"kind", kind,
	)
}

func (r *SlogReporter) ExecutorStarting(stepID, executor string, timeout time.Duration) {
	args := []any{"step_id", stepID, "executor", executor}
	if timeout > 0 {
		args = append(args, "timeout", timeout.String())
	}
	r.logger.Info("act.executor.start", args...)
}

func (r *SlogReporter) ExecutorFinished(stepID, executor string, elapsed time.Duration, err error) {
	args := []any{"step_id", stepID, "executor", executor, "elapsed", elapsed.Round(time.Millisecond).String()}
	if err != nil {
		r.logger.Warn("act.executor.failed", append(args, "error", err.Error())...)
		return
	}
	r.logger.Info("act.executor.done", args...)
}

func (r *SlogReporter) ModelFailover(stepID, from, to string, class model.FailureClass, cause error) {
	r.logger.Warn("act.model.failover",
		"step_id", stepID,
		"from", from,
		"to", to,
		"class", string(class),
		"cause", cause.Error(),
	)
}

func (r *SlogReporter) RepairSkipped(reason string) {
	r.logger.Warn("act.repair.skipped", "reason", reason)
}

func (r *SlogReporter) BudgetExceeded(reason string) {
	r.logger.Warn("act.budget.exceeded", "reason", reason)
}
