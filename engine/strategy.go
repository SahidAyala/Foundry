package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SahidAyala/Foundry/domain"
)

// verdictFail is the Gate verdict that triggers a Pipeline's bounded
// repair: PipelineStrategy treats it as a signal to re-run the Pipeline,
// not as an error. The architecture reserves a distinct "repair" verdict
// (docs/02-architecture/execution.md step 6); M0's Gate emits only
// pass/fail, so fail is the trigger (backlog PR-011).
const verdictFail = "fail"

// verificationFailedReason and generateFailedReason are the two causes
// that spend a repair round, passed to Reporter.Repairing so narration
// states which one actually happened. Before Repairing carried a reason,
// every round was announced as a verification failure — including the
// rounds where no verify Step ever ran because the generate Step itself
// failed (a timed-out Executor call, output with no parseable diff), which
// is the single most confusing thing a Foundry run could print: the
// reported cause was not the real one, and the real one appeared nowhere
// until the Act's final error.
const (
	verificationFailedReason = "verification failed"
	generateFailedReason     = "generate step failed"
)

// VerdictExecuteFailed is the Act-level JudgmentVerdict recorded when a
// generate Step failed on every attempt its Pipeline allowed, so no
// Outcome was ever produced to judge. It is deliberately distinct from
// verdictFail ("verification looked at an Outcome and rejected it") and
// from VerdictBudgetExceeded ("the Budget stopped us"): all three end an
// Act without an applied patch, but only this one means Foundry never got
// a candidate at all — a materially different thing to find in the Record
// weeks later, and the difference between a model that proposed something
// wrong and a call that never came back.
const VerdictExecuteFailed = "execute-failed"

// ErrExecuteFailed marks the error Produce returns once a generate Step
// has failed on every attempt allowed. Callers use it the same way they
// already use ErrBudgetExceeded: it means the Engine is handing back a
// usable Act alongside the error — one whose Judgment records what
// happened — rather than the bare error every other Step failure returns.
// Without this distinction the Act was discarded outright
// (Engine.RunBudgeted returned nil for it), so a run that spent fifteen
// minutes across three timed-out rounds left nothing on disk at all: no
// Record, and no checkpoint either, since a failed generate Step never
// completes and so is never checkpointed.
var ErrExecuteFailed = errors.New("engine: generate step failed on every attempt")

// generateStepFailure marks an error returned by a generate Step's own
// attempt to produce an Outcome (routing, cost estimation, or the
// Executor call itself — e.g. a CLI exiting non-zero, or a model
// response executor.ParsePatch cannot find a unified diff in) as
// eligible for the Pipeline's own bounded repair, symmetric to a failed
// verify Judgment: both mean "this attempt did not produce a usable
// Outcome," and Produce already knows how to spend remaining repair
// budget retrying that. Before this type existed, any generate Step
// error ended the Act immediately, even with repair budget left unspent
// — unlike a failing verify Judgment, which already retried. Any other
// Step kind's error (verify infrastructure, approve, apply, record,
// checkpoint) is deliberately left as a plain error and still ends the
// Act immediately: those failures are not "the model produced a bad
// Outcome," so retrying via the same repair mechanism would not be
// safe or meaningful for them.
type generateStepFailure struct {
	err error
}

func (g *generateStepFailure) Error() string { return g.err.Error() }
func (g *generateStepFailure) Unwrap() error { return g.err }

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
	router          Router
	verifier        Verifier
	workspace       string
	reporter        Reporter
	authority       Authority
	applier         Applier
	applierRegistry *ApplierRegistry
	checkpointer    Checkpointer
	checkpoints     CheckpointSaver
	spent           *tracker
}

// resolveApplier returns the Applier a Step's apply Target names:
// rc.applier — the Engine's single configured Applier — if target is empty
// or ApplyTargetLocal (every apply Step before Target existed, and every
// one that still declares none), or the named Applier from
// rc.applierRegistry otherwise, erroring clearly if that target isn't
// registered (mirrors Router.Resolve's unresolved-pin behavior: a Target
// that can't be honored is never silently ignored in favor of the default).
func (rc runContext) resolveApplier(target string) (Applier, error) {
	if target == "" || target == ApplyTargetLocal {
		return rc.applier, nil
	}
	return rc.applierRegistry.Get(target)
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
// every Pipeline did before Target existed. Budget is charged per generate
// Step as runSteps executes it (RFC-0004 §2.7, Piece 5 of
// docs/04-guides/multi-executor-router-implementation-plan.md), not once
// per attempt — a Pipeline with more than one generate Step per attempt
// (e.g. feature.json's plan + implement) is charged for every Executor call
// it actually makes. A Budget refusal anywhere in the first attempt halts:
// act is marked VerdictBudgetExceeded and the refusal is returned as an
// error wrapping ErrBudgetExceeded. A Budget refusal anywhere in a repair
// attempt is not an error — the prior attempt's Judgment stands as final.
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
	act.DeclaresApproveStep = declaresApproveStep(s.Pipeline.Steps)

	var outcome *domain.Outcome
	var judgment *domain.Judgment
	// genFailures accumulates one entry per attempt whose generate Step
	// failed outright, so the Act carries every round's cause rather than
	// only the last one. A three-round run that timed out every time used
	// to surface exactly one error message — the third — and discard the
	// first two entirely.
	var genFailures []string

	for attempt := 0; ; attempt++ {
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
			if errors.Is(err, ErrBudgetExceeded) {
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
			var genErr *generateStepFailure
			if !errors.As(err, &genErr) {
				return err
			}
			// A generate Step that failed outright (rather than producing
			// an Outcome a verify Step could judge) is treated exactly
			// like a failed verify Judgment below: spend remaining repair
			// budget retrying it, and only return the error once that
			// budget is exhausted — see generateStepFailure's own doc
			// comment for why this is safe to do for a generate Step's
			// own failure specifically, unlike any other Step kind's.
			// judgment carries only this attempt's own failure: it is what
			// repairContext feeds the next attempt, and `considered`
			// already accumulates one repairContext per round, so putting
			// the whole history here would repeat every earlier round in
			// every later prompt.
			judgment = &domain.Judgment{Verdict: verdictFail, Checked: []string{"generate: " + genErr.Error()}}
			genFailures = append(genFailures, fmt.Sprintf("generate step failed (attempt %d): %s", attempt+1, genErr.Error()))
			act.JudgmentVerdict = judgment.Verdict
			act.CheckedFindings = genFailures
			if attempt >= s.Pipeline.Repair.MaxAttempts {
				// Two genuinely different endings share this branch, and
				// only one of them is terminal.
				//
				// If some Step of some attempt already completed, a
				// checkpoint exists on disk (rc.checkpoints.Save runs after
				// every completed Step), so this is an *interruption*
				// `foundry resume` can continue — the reading
				// session.TestRunPipelineCommand_SavesCheckpointOnInterruption
				// deliberately pins for a mid-repair Executor crash. Return
				// the plain error, leaving both the checkpoint and the
				// caller's behavior exactly as they were: recording a Record
				// for an Act that is still resumable would give the same Act
				// ID two terminal destinies, and the second Write would fail
				// outright against the write-once Record (ADR-0002).
				if len(act.Steps) > 0 {
					return err
				}
				// Otherwise nothing completed, so nothing exists anywhere:
				// no Outcome to judge, no Step recorded, and no checkpoint —
				// a failed generate Step never completes, so it is never
				// checkpointed. This Act consumed real time and budget and
				// would vanish entirely (I8), exactly as the run that
				// motivated this did: three timed-out rounds, fifteen
				// minutes, and nothing on disk afterwards. ErrExecuteFailed
				// tells the caller to record it; see Engine.RunBudgeted and
				// cli.CLI.reportFailedRun. Nothing needs deleting here, so
				// ADR-0002's "a checkpoint lives only until an Act is
				// terminal" holds without a delete.
				//
				// Deliberately no StepRecord is appended for the failed
				// generate Step either: act.Steps doubles as the resume
				// position (Engine.Resume uses len(act.Steps) as the index
				// of the first not-yet-completed Step), so recording a Step
				// that did not complete would make a later resume skip it —
				// and would break the len(act.Steps) test just above.
				act.JudgmentVerdict = VerdictExecuteFailed
				return fmt.Errorf("%w: %w", ErrExecuteFailed, err)
			}
			rc.reporter.Repairing(generateFailedReason + ": " + genErr.Error())
			continue
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
		rc.reporter.Repairing(verificationFailedReason)
	}

	// act.JudgmentVerdict/CheckedFindings are already set — by the verify
	// case inside runSteps, the moment the last verify Step actually ran,
	// not here. Setting them again would be redundant; it would also be
	// too late for a Pipeline whose own record Step already persisted act
	// earlier in this same runSteps call (see the verify case's comment).
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
	for i, step := range steps {
		if judgment != nil && judgment.Verdict == verdictFail && stopsShortOnFailure(step.Kind) {
			break
		}
		// Announced after the stopsShortOnFailure guard, never before: a
		// Step the attempt deliberately skips was not started, and saying
		// it was would be exactly the kind of narration this reports
		// against.
		reportStepStarting(rc.reporter, attempt+1, i+1, len(steps), step.ID, step.Kind)
		switch step.Kind {
		case domain.StepKindGenerate:
			stepConsidered := considered
			if step.FeedsForward && len(act.Steps) > 0 {
				stepConsidered = appendFeedsForward(considered, act.Steps[len(act.Steps)-1])
			}
			start := time.Now()
			o, err := executeGenerateStep(ctx, rc, step, intent, stepConsidered, attempt)
			if err != nil {
				return outcome, judgment, false, &generateStepFailure{err: err}
			}
			outcome = o
			// Intent is set here, by the Engine, rather than by any
			// Executor — so a later verify Step's Verifier (e.g.
			// verify/aireview) can judge the Patch against what was
			// actually asked for without every existing Executor
			// implementation needing to change to populate it.
			outcome.Intent = intent.Text
			act.Patch = outcome.Patch
			act.Iterations = rc.spent.iterations
			act.CostEstimateUSD = rc.spent.costUSD
			act.ConsideredFiles = stepConsidered
			accumulateActualCost(act, outcome.ActualCostUSD)
			recordStep(act, domain.StepKindGenerate, stepConsidered, producedPatch(outcome), nil, "", "", start, outcome.ActualCostUSD)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindVerify:
			if outcome == nil {
				return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: verify has no Outcome to check — no generate step ran before it", pipelineName, step.ID)
			}
			rc.reporter.Verifying(attempt + 1)
			start := time.Now()
			j, err := rc.verifier.Verify(ctx, outcome, rc.workspace)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "verify", err)
			}
			judgment = j
			rc.reporter.Verified(attempt+1, judgment)
			// act's flat JudgmentVerdict/CheckedFindings are set here, not
			// only after the whole Pipeline attempt finishes (Produce/
			// Resume used to set them post-hoc): a Pipeline declaring its
			// own record Step (RFC-0002 §9 Phase 4) can call
			// rc.checkpointer.Write later in this same runSteps call,
			// persisting act — and until this fix, that write happened
			// before Produce/Resume ever got a chance to set these flat
			// fields, permanently recording an empty JudgmentVerdict and
			// nil CheckedFindings despite Steps[i]'s own trace already
			// carrying the correct verdict. Setting them synchronously
			// here keeps them correct for every later Step in the same
			// attempt, not just for a caller that inspects act afterward.
			act.JudgmentVerdict = judgment.Verdict
			act.CheckedFindings = judgment.Checked
			recordStep(act, domain.StepKindVerify, nil, nil, judgment.Checked, judgment.Verdict, "", start, nil)
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
				recordStep(act, domain.StepKindApprove, nil, nil, nil, stepVerdictReject, "", start, nil)
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
			recordStep(act, domain.StepKindApprove, nil, nil, nil, stepVerdictAccept, authority, start, nil)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindApply:
			if act.ApprovedAt == nil {
				return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: apply requires an accepted approve step first", pipelineName, step.ID)
			}
			start := time.Now()
			applier, err := rc.resolveApplier(step.Target)
			if err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "route", err)
			}
			if err := applier.Apply(ctx, rc.workspace, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "apply", err)
			}
			recordStep(act, domain.StepKindApply, nil, producedPatch(outcome), nil, "", "", start, nil)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		case domain.StepKindRecord:
			start := time.Now()
			if err := rc.checkpointer.Write(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "record", err)
			}
			recordStep(act, domain.StepKindRecord, nil, nil, nil, "", "", start, nil)
			if err := rc.checkpoints.Save(ctx, act); err != nil {
				return outcome, judgment, false, wrapStepError(attempt, "checkpoint", err)
			}

		default:
			return outcome, judgment, false, fmt.Errorf("engine: pipeline %q step %q: unrecognized step kind %q", pipelineName, step.ID, step.Kind)
		}
	}
	return outcome, judgment, false, nil
}

// declaresApproveStep reports whether steps contains an approve Step —
// a static fact about a Pipeline document, recorded onto domain.Act by
// Produce/Resume so cli.CLI.finalize can tell "this Pipeline declares
// its own approve Step, but this attempt's repair exhausted before
// reaching it" apart from "this Pipeline never declares one at all"
// (the built-in "default"/"review", which still rely on finalize's own
// fallback prompt). See domain.Act.DeclaresApproveStep's own doc
// comment for why that distinction matters.
func declaresApproveStep(steps []Step) bool {
	for _, step := range steps {
		if step.Kind == domain.StepKindApprove {
			return true
		}
	}
	return false
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
	return renderStepRecord("verification findings from the failed previous attempt", domain.StepRecord{Checked: judgment.Checked})
}

// renderStepRecord renders step's most relevant recorded output — its
// Produced patch, if it generated one, otherwise its Checked verification
// findings — as one considered-context entry prefixed by label. It is the
// one rendering repairContext (repair-only, attempt > 0) and feeds_forward
// threading (runSteps' StepKindGenerate case, below) both reuse
// (RFC-0004 §3, docs/04-guides/multi-executor-router-implementation-plan.md
// Piece 1, Commit 5).
func renderStepRecord(label string, step domain.StepRecord) string {
	if len(step.Produced) > 0 {
		return label + ":\n" + strings.Join(step.Produced, "\n")
	}
	return label + ":\n" + strings.Join(step.Checked, "\n")
}

// feedsForwardLabel prefixes the rendering a FeedsForward Step's
// immediately preceding StepRecord gets, distinguishing it from a repair
// round's "verification findings from the failed previous attempt" in any
// considered-context list that contains both.
const feedsForwardLabel = "output from the immediately preceding step"

// appendFeedsForward returns a new slice holding considered's entries plus
// prev rendered as one more entry, leaving considered itself untouched —
// the augmentation is scoped to the one Step that declared
// FeedsForward: true, not threaded into any later Step's Context
// (RFC-0004 §3: "the one immediately before," applied once).
func appendFeedsForward(considered []string, prev domain.StepRecord) []string {
	augmented := make([]string, len(considered), len(considered)+1)
	copy(augmented, considered)
	return append(augmented, renderStepRecord(feedsForwardLabel, prev))
}
