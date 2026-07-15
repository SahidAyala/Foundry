package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"foundry/domain"
)

// verdictFail is the Gate verdict that triggers a Pipeline's bounded
// repair: PipelineStrategy treats it as a signal to re-run the Pipeline,
// not as an error. The architecture reserves a distinct "repair" verdict
// (docs/02-architecture/execution.md step 6); M0's Gate emits only
// pass/fail, so fail is the trigger (backlog PR-011).
const verdictFail = "fail"

// Strategy is the pluggable means by which an Act's Outcome and Judgment
// are produced, once Context has been gathered and a Budget tracker is in
// place (docs/02-architecture/execution.md). The Engine owns control flow
// up to invoking a Strategy; a Strategy decides how the work described by
// its Pipeline actually runs — no model ever decides what runs next.
type Strategy interface {
	Produce(ctx context.Context, act *domain.Act, intent *domain.Intent, considered []string, rc runContext) error
}

// runContext bundles what a Strategy needs to produce an Act: the ports to
// call, the workspace to verify against, the Reporter to narrate progress,
// and the Budget tracker enforcing this Act's ceiling.
type runContext struct {
	router       Router
	verifier     Verifier
	workspace    string
	reporter     Reporter
	authority    Authority
	applier      Applier
	checkpointer Checkpointer
	checkpoints  CheckpointSaver
	spent        *tracker
}

// PipelineStrategy produces an Act by walking a Pipeline's Steps in order,
// re-running the Pipeline (bounded by its RepairPolicy) whenever the final
// verify Step's Judgment is "fail". It is the only Strategy today;
// DefaultPipeline makes it reproduce the Engine's original hardcoded
// lifecycle exactly.
type PipelineStrategy struct {
	Pipeline Pipeline
}

var _ Strategy = PipelineStrategy{}

// Produce runs s.Pipeline's Steps against act, attempting a repair re-run
// (bounded by s.Pipeline.Repair.MaxAttempts) whenever a verify Step's
// Judgment is "fail". A repair re-run jumps to s.Pipeline.Repair.Target
// (RFC-0002 §4.3's "named earlier Step") and replays only that Step onward,
// not the whole Pipeline; an unset Target replays from Pipeline.Steps[0], as
// every Pipeline did before Target existed. A Budget refusal on the first
// attempt halts: act is marked VerdictBudgetExceeded and the refusal is
// returned as an error wrapping ErrBudgetExceeded. A Budget refusal on a
// repair attempt is not an error — the prior attempt's Judgment stands as
// final.
//
// Produce executes s.Pipeline purely from its Steps and RepairPolicy — any
// well-formed sequence of generate/verify Steps runs unmodified, with no
// Engine or Strategy code change required to add a differently-shaped
// Pipeline. A Pipeline is well-formed only if every verify Step is preceded
// by a generate Step in the same attempt, at least one verify Step exists,
// and every Step's Kind is one Produce recognizes; Produce returns a clear
// error instead of a nil-pointer panic or a silently skipped Step when a
// Pipeline violates any of these (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md
// §5: Step kinds are "a closed set, extensible only by adding a new kind
// deliberately, not by the Pipeline document inventing arbitrary behavior").
// An approve Step calls rc.authority.Decide: on acceptance, act.ApprovedBy/
// ApprovedAt are set and the Pipeline continues; on rejection, Produce stops
// immediately with act.JudgmentVerdict set to VerdictRejected — no further
// Step runs, and no repair is attempted, since a human decision is not
// something a bounded repair round can fix. An apply Step calls
// rc.applier.Apply, but only once act.ApprovedBy/ApprovedAt are set by a
// preceding approve Step — a Pipeline that reaches apply without one
// declared and accepted is a configuration error, not a silently applied
// unapproved Outcome. A record Step calls rc.checkpointer.Write to persist
// act as it stands so far — RFC-0002 §9 Phase 4's last piece. Whenever the
// most recent Verify Step's Judgment is "fail", the current attempt stops
// before any approve/apply/record Step (stopsShortOnFailure) — a failing
// Outcome is never presented for approval, applied, or recorded, whether or
// not this attempt goes on to repair.
//
// The actual per-Step work is runSteps, below. Produce's attempt loop is
// the only caller today, but Engine.Resume (engine.go) is a second: it
// seeds runSteps with the Outcome/Judgment an interrupted attempt held in
// memory and continues from the first not-yet-completed Step, so a crash
// mid-Pipeline resumes through identical logic to a first attempt
// (docs/06-open-questions/OQ-008-in-progress-act-persistence.md). Every
// completed Step is checkpointed via rc.checkpoints.Save; once Produce (or
// Resume) reaches a genuine terminal disposition, the checkpoint is
// deleted — it exists only to survive an interruption, never past one.
func (s PipelineStrategy) Produce(ctx context.Context, act *domain.Act, intent *domain.Intent, considered []string, rc runContext) error {
	var outcome *domain.Outcome
	var judgment *domain.Judgment

	for attempt := 0; ; attempt++ {
		if err := rc.spent.charge(executeCostEstimateUSD); err != nil {
			if attempt == 0 {
				rc.reporter.BudgetExceeded(err.Error())
				act.JudgmentVerdict = VerdictBudgetExceeded
				act.Iterations = rc.spent.iterations
				act.CostEstimateUSD = rc.spent.costUSD
				return err
			}
			rc.reporter.RepairSkipped(err.Error())
			break
		}

		steps := s.Pipeline.Steps
		if attempt > 0 {
			repaired := make([]string, 0, len(considered)+1)
			repaired = append(repaired, considered...)
			repaired = append(repaired, repairContext(judgment))
			considered = repaired

			if idx, ok := stepIndex(s.Pipeline.Steps, s.Pipeline.Repair.Target); ok {
				steps = s.Pipeline.Steps[idx:]
			}
		}

		o, j, terminal, err := runSteps(ctx, s.Pipeline.Name, act, intent, steps, considered, outcome, judgment, attempt, rc)
		outcome, judgment = o, j
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}

		if judgment == nil {
			return fmt.Errorf("engine: pipeline %q declares no verify step: it can never produce a Judgment", s.Pipeline.Name)
		}
		if judgment.Verdict != verdictFail || attempt >= s.Pipeline.Repair.MaxAttempts {
			break
		}
		rc.reporter.Repairing()
	}

	act.JudgmentVerdict = judgment.Verdict
	act.CheckedFindings = judgment.Checked
	if err := rc.checkpoints.Delete(ctx, act.ID); err != nil {
		return fmt.Errorf("engine: checkpoint delete: %w", err)
	}
	return nil
}

// runSteps executes steps against act in order, threading outcome and
// judgment through Generate and Verify Steps exactly as Produce's attempt
// loop always has, and is the one place a Step actually runs — both
// Produce and Engine.Resume call it, so an interrupted attempt resumes
// through identical logic to a first attempt, checkpoint saves and
// stopsShortOnFailure guard included.
//
// It returns the updated outcome/judgment and, if an approve Step is
// declined, terminal=true: act.JudgmentVerdict and its checkpoint are
// already finalized in that case (a human decision is not something a
// bounded repair round, or a resume, can revisit), and the caller must
// simply return nil rather than process act any further. A non-nil error
// means a Step itself failed — the checkpoint saved by the last
// successfully completed Step survives on disk, exactly the state
// `foundry resume` needs.
func runSteps(ctx context.Context, pipelineName string, act *domain.Act, intent *domain.Intent, steps []Step, considered []string, outcome *domain.Outcome, judgment *domain.Judgment, attempt int, rc runContext) (*domain.Outcome, *domain.Judgment, bool, error) {
	for _, step := range steps {
		if judgment != nil && judgment.Verdict == verdictFail && stopsShortOnFailure(step.Kind) {
			break
		}
		switch step.Kind {
		case domain.StepKindGenerate:
			rc.reporter.Executing(rc.spent.iterations)
			start := time.Now()
			resolved, err := rc.router.Resolve(step)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "route", err)
			}
			o, err := resolved.Execute(ctx, intent, considered)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "execute", err)
			}
			outcome = o
			act.Patch = outcome.Patch
			act.Iterations = rc.spent.iterations
			act.CostEstimateUSD = rc.spent.costUSD
			act.ConsideredFiles = considered
			recordStep(act, domain.StepKindGenerate, considered, producedPatch(outcome), nil, "", "", start)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindVerify:
			if outcome == nil {
				return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: verify has no Outcome to check — no generate step ran before it", pipelineName, step.ID)
			}
			rc.reporter.Verifying(rc.spent.iterations)
			start := time.Now()
			j, err := rc.verifier.Verify(ctx, outcome, rc.workspace)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "verify", err)
			}
			judgment = j
			rc.reporter.Verified(rc.spent.iterations, judgment)
			recordStep(act, domain.StepKindVerify, nil, nil, judgment.Checked, judgment.Verdict, "", start)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindApprove:
			if outcome == nil {
				return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: approve has no Outcome to review — no generate step ran before it", pipelineName, step.ID)
			}
			start := time.Now()
			authority, approved, err := rc.authority.Decide(ctx, act)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "approve", err)
			}
			if !approved {
				recordStep(act, domain.StepKindApprove, nil, nil, nil, stepVerdictReject, "", start)
				act.JudgmentVerdict = VerdictRejected
				if judgment != nil {
					act.CheckedFindings = judgment.Checked
				}
				if err := rc.checkpoints.Delete(ctx, act.ID); err != nil {
					return outcome, judgment, false, fmt.Errorf("engine: checkpoint delete: %w", err)
				}
				return outcome, judgment, true, nil
			}
			now := time.Now()
			act.ApprovedBy = authority
			act.ApprovedAt = &now
			recordStep(act, domain.StepKindApprove, nil, nil, nil, stepVerdictAccept, authority, start)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindApply:
			if act.ApprovedAt == nil {
				return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: apply requires an accepted approve step first", pipelineName, step.ID)
			}
			start := time.Now()
			if err := rc.applier.Apply(ctx, rc.workspace, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "apply", err)
			}
			recordStep(act, domain.StepKindApply, nil, producedPatch(outcome), nil, "", "", start)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindRecord:
			start := time.Now()
			if err := rc.checkpointer.Write(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "record", err)
			}
			recordStep(act, domain.StepKindRecord, nil, nil, nil, "", "", start)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		default:
			return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: unrecognized step kind %q", pipelineName, step.ID, step.Kind)
		}
	}
	return outcome, judgment, false, nil
}

// stopsShortOnFailure reports whether kind is a trust-boundary Step
// (approve, apply, record) that must never run against a failing Judgment.
// Generate and Verify Steps always run regardless of an earlier Judgment —
// review.json's independent, sequential verify Steps rely on exactly that
// — but a Pipeline must never seek approval for, apply, or record an
// Outcome its own most recent Verify Step just rejected. Reaching one after
// a "fail" ends the attempt early; the outer repair-or-finalize decision in
// Produce is unaffected by whether the loop ran every Step or stopped short.
func stopsShortOnFailure(kind string) bool {
	switch kind {
	case domain.StepKindApprove, domain.StepKindApply, domain.StepKindRecord:
		return true
	default:
		return false
	}
}

// stepIndex returns the index of the first Step in steps whose ID is id,
// and whether one was found. An empty id never matches, so an unset
// RepairPolicy.Target falls through to Produce's "restart from the top"
// default rather than needing its own special case at the call site.
func stepIndex(steps []Step, id string) (int, bool) {
	if id == "" {
		return 0, false
	}
	for i, step := range steps {
		if step.ID == id {
			return i, true
		}
	}
	return 0, false
}

// wrapStepError renders a Step failure, prefixing it as a repair failure
// once the Pipeline is re-running (attempt > 0) — matching the distinct
// error strings ("engine: execute: ..." vs "engine: repair execute: ...")
// the Engine has always produced for the first attempt vs a repair attempt.
func wrapStepError(attempt int, op string, err error) error {
	if attempt > 0 {
		return fmt.Errorf("engine: repair %s: %w", op, err)
	}
	return fmt.Errorf("engine: %s: %w", op, err)
}

// repairContext renders a failed Judgment's checked findings as one
// considered-context entry, so the Executor sees what failed on the
// previous attempt and the Act's Evidence records what the repair saw.
func repairContext(judgment *domain.Judgment) string {
	return "verification findings from the failed previous attempt:\n" +
		strings.Join(judgment.Checked, "\n")
}
