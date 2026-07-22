package engine

import (
	"strconv"
	"time"

	"foundry/domain"
)

// recordStep appends one StepRecord to act's trace. kind identifies the
// unit of work (domain.StepKindGenerate, domain.StepKindVerify, or
// domain.StepKindApprove); considered/produced/checked/verdict/authority
// carry whatever that kind produced and are empty/nil where not applicable;
// started is when the underlying Executor, Verifier, or Authority call
// began. StepID is assigned sequentially from the act's current trace
// length, so repair rounds continue the sequence rather than restarting it.
// actualCostUSD is a generate Step's real, post-execution cost if its
// Executor reported one (ADR-0011); nil for every other Step kind. See
// docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §4.5.
func recordStep(act *domain.Act, kind string, considered, produced, checked []string, verdict, authority string, started time.Time, actualCostUSD *float64) {
	act.Steps = append(act.Steps, domain.StepRecord{
		StepID:          strconv.Itoa(len(act.Steps) + 1),
		Kind:            kind,
		Considered:      considered,
		Produced:        produced,
		Checked:         checked,
		JudgmentVerdict: verdict,
		Authority:       authority,
		StartedAt:       started,
		FinishedAt:      time.Now(),
		ActualCostUSD:   actualCostUSD,
	})
}

// accumulateActualCost adds cost onto act.ActualCostUSD if cost is
// non-nil (ADR-0011) — reported Evidence only, never consulted by Budget's
// enforcement (engine/budget.go's tracker.charge reads only pre-execution
// estimates). act.ActualCostUSD stays nil until the first non-nil cost is
// seen, so a caller can distinguish "no Executor has ever reported one"
// from "the reported total is zero."
func accumulateActualCost(act *domain.Act, cost *float64) {
	if cost == nil {
		return
	}
	total := *cost
	if act.ActualCostUSD != nil {
		total += *act.ActualCostUSD
	}
	act.ActualCostUSD = &total
}

// producedPatch renders outcome's patch as a StepRecord's Produced slice, or
// nil if the Executor produced no patch.
func producedPatch(outcome *domain.Outcome) []string {
	if outcome.Patch == "" {
		return nil
	}
	return []string{outcome.Patch}
}

// lastOutcomeAndJudgment reconstructs the Outcome and Judgment an
// interrupted attempt held in memory, by scanning steps for the most
// recent generate and verify StepRecords — the same technique
// replay/replay.go's Verify uses for its lastPatch. Engine.Resume uses this
// to seed runSteps exactly as if the attempt had never stopped.
func lastOutcomeAndJudgment(steps []domain.StepRecord) (*domain.Outcome, *domain.Judgment) {
	var outcome *domain.Outcome
	var judgment *domain.Judgment
	for _, step := range steps {
		switch step.Kind {
		case domain.StepKindGenerate:
			if len(step.Produced) > 0 {
				outcome = &domain.Outcome{Patch: step.Produced[0], ActualCostUSD: step.ActualCostUSD}
			}
		case domain.StepKindVerify:
			judgment = &domain.Judgment{Verdict: step.JudgmentVerdict, Checked: step.Checked}
		}
	}
	return outcome, judgment
}
