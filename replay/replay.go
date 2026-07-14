// Package replay re-runs verification against a previously recorded Act's
// Step trace, without ever calling an Executor again, and reports whether
// each verify Step reproduces its recorded Judgment.
//
// This is a same-version replay guarantee only
// (docs/06-open-questions/OQ-003-replay-across-versions.md): it proves
// verification is reproducible under the Engine build that runs it today,
// not that it would reproduce identically under a future Engine version —
// that cross-version scope is still an open question, deliberately not
// decided here. Per execution.md's replay model, Verify is the
// deterministic work this package re-executes for real; a generate Step's
// Executor output is the non-deterministic part, replayed from the Record
// (its recorded Produced patch) and never re-derived.
package replay

import (
	"context"
	"fmt"

	"foundry/domain"
	"foundry/engine"
)

// StepResult is one verify Step's replay outcome.
type StepResult struct {
	StepID          string
	Reproduced      bool
	RecordedVerdict string
	ReplayedVerdict string
	RecordedChecked []string
	ReplayedChecked []string
}

// Result is a full Act's replay outcome: one StepResult per verify Step
// recorded in the Act's trace, in order. An Act with no recorded Steps
// (predating the trace, or never checkpointed) yields a Result with no
// Steps — nothing to replay is not a failure.
type Result struct {
	ActID string
	Steps []StepResult
}

// Reproduced reports whether every verify Step reproduced its recorded
// Judgment. An empty Result (nothing to replay) reports true: there is
// nothing to have diverged.
func (r Result) Reproduced() bool {
	for _, s := range r.Steps {
		if !s.Reproduced {
			return false
		}
	}
	return true
}

// Verify replays act: for each verify StepRecord in act.Steps, it takes the
// immediately preceding generate StepRecord's recorded Produced patch,
// builds a domain.Outcome from it, and calls verifier — a real Verifier,
// re-executed against workspace exactly as the original verify Step ran
// (docs/02-architecture/execution.md, step 4). The Executor is never
// invoked. A verify Step with no preceding generate Outcome to replay is a
// clear, named error, not a nil-outcome panic.
func Verify(ctx context.Context, act *domain.Act, verifier engine.Verifier, workspace string) (Result, error) {
	result := Result{ActID: act.ID}

	var lastPatch string
	haveOutcome := false

	for _, step := range act.Steps {
		switch step.Kind {
		case domain.StepKindGenerate:
			if len(step.Produced) == 0 {
				haveOutcome = false
				continue
			}
			lastPatch = step.Produced[0]
			haveOutcome = true

		case domain.StepKindVerify:
			if !haveOutcome {
				return Result{}, fmt.Errorf("replay: act %s: verify step %s has no preceding generate Outcome to replay", act.ID, step.StepID)
			}
			outcome := &domain.Outcome{Patch: lastPatch}
			judgment, err := verifier.Verify(ctx, outcome, workspace)
			if err != nil {
				return Result{}, fmt.Errorf("replay: act %s: verify step %s: %w", act.ID, step.StepID, err)
			}
			result.Steps = append(result.Steps, StepResult{
				StepID:          step.StepID,
				Reproduced:      judgment.Verdict == step.JudgmentVerdict,
				RecordedVerdict: step.JudgmentVerdict,
				ReplayedVerdict: judgment.Verdict,
				RecordedChecked: step.Checked,
				ReplayedChecked: judgment.Checked,
			})
		}
	}

	return result, nil
}
