package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
)

// observingReporter records every event of every Reporter interface the
// Engine can call — the required one plus all three optional extensions —
// so these tests can assert on what a caller is actually able to observe
// during a run, not only on what the Engine decided.
type observingReporter struct {
	events        []string
	repairReasons []string
	timeouts      []time.Duration

	intent          string
	gatheredEntries int
	gatheredBytes   int
}

func (r *observingReporter) IntentDeclared(text string) {
	r.events = append(r.events, "intent")
	r.intent = text
}

func (r *observingReporter) ContextGathered(entries, bytes int) {
	r.events = append(r.events, "gathered")
	r.gatheredEntries, r.gatheredBytes = entries, bytes
}

func (r *observingReporter) Gathering()                              {}
func (r *observingReporter) Executing(iteration int)                 {}
func (r *observingReporter) Verifying(iteration int)                 {}
func (r *observingReporter) Executed(iteration int, actual *float64) {}
func (r *observingReporter) Verified(i int, j *domain.Judgment)      {}
func (r *observingReporter) RepairSkipped(reason string)             {}
func (r *observingReporter) BudgetExceeded(reason string)            {}
func (r *observingReporter) Repairing(reason string) {
	r.events = append(r.events, "repairing")
	r.repairReasons = append(r.repairReasons, reason)
}

func (r *observingReporter) StepStarting(attempt, index, total int, stepID, kind string) {
	r.events = append(r.events, "step:"+stepID+":"+kind)
}

func (r *observingReporter) ExecutorStarting(stepID, executor string, timeout time.Duration) {
	r.events = append(r.events, "executor-start:"+stepID+":"+executor)
	r.timeouts = append(r.timeouts, timeout)
}

func (r *observingReporter) ExecutorFinished(stepID, executor string, elapsed time.Duration, err error) {
	if err != nil {
		r.events = append(r.events, "executor-failed:"+executor+":"+err.Error())
		return
	}
	r.events = append(r.events, "executor-done:"+executor)
}

func (r *observingReporter) ModelFailover(stepID, from, to string, class model.FailureClass, cause error) {
	r.events = append(r.events, "failover:"+from+"->"+to)
}

var (
	_ engine.Reporter         = (*observingReporter)(nil)
	_ engine.InputReporter    = (*observingReporter)(nil)
	_ engine.StepReporter     = (*observingReporter)(nil)
	_ engine.ExecutorReporter = (*observingReporter)(nil)
	_ engine.FailoverReporter = (*observingReporter)(nil)
)

// timeoutExecutor is a ScriptedExecutor-shaped Executor that also advertises
// a per-call deadline, the way every CLI-backed Executor in this repository
// does — the seam engine's executorTimeout asserts on so narration can show
// how much of the deadline a long call has consumed.
type timeoutExecutor struct {
	patch   string
	err     error
	timeout time.Duration
}

func (e *timeoutExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	if e.err != nil {
		return nil, e.err
	}
	return &domain.Outcome{Patch: e.patch}, nil
}

func (e *timeoutExecutor) Timeout() time.Duration { return e.timeout }

// TestReporter_RepairingCarriesGenerateFailureAsItsReason is the point of
// Repairing taking a reason: a repair round earned by a generate Step
// failing outright must not be narrated as a verification failure, since no
// verification ran at all. Before this, a three-round run whose Executor
// timed out every time printed "Verification failed" three times and never
// showed the timeout.
func TestReporter_RepairingCarriesGenerateFailureAsItsReason(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1, Target: "implement"},
	}

	exec := &captureExecutor{
		patches: []string{"", "repaired-patch"},
		errs:    []error{errors.New("claude: timed out after 5m0s"), nil},
	}
	reporter := &observingReporter{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(reporter.repairReasons) != 1 {
		t.Fatalf("Repairing called %d times, want 1: %v", len(reporter.repairReasons), reporter.repairReasons)
	}
	reason := reporter.repairReasons[0]
	if !strings.Contains(reason, "timed out after 5m0s") {
		t.Errorf("repair reason = %q, want it to carry the real generate failure", reason)
	}
	if strings.Contains(reason, "verification") {
		t.Errorf("repair reason = %q, must not blame verification: no verify step ran this attempt", reason)
	}
}

// TestReporter_RepairingCarriesVerificationFailureAsItsReason is the other
// half: a round earned by a failing Judgment still says so.
func TestReporter_RepairingCarriesVerificationFailureAsItsReason(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1, Target: "implement"},
	}

	exec := &captureExecutor{patches: []string{"p1", "p2"}, errs: []error{nil, nil}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	reporter := &observingReporter{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", pipeline)
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(reporter.repairReasons) != 1 {
		t.Fatalf("Repairing called %d times, want 1: %v", len(reporter.repairReasons), reporter.repairReasons)
	}
	if got := reporter.repairReasons[0]; !strings.Contains(got, "verification failed") {
		t.Errorf("repair reason = %q, want it to name the failed verification", got)
	}
}

// TestReporter_NarratesEveryStepAndTheExecutorCall covers what a caller can
// see during the minutes-long window an Act spends inside Execute: which
// Step is running, which Executor was resolved, that deadline, and a
// definite end to the call whether it succeeded or failed.
func TestReporter_NarratesEveryStepAndTheExecutorCall(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	exec := &timeoutExecutor{patch: "a-patch", timeout: 90 * time.Second}
	reporter := &observingReporter{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	want := []string{
		"intent",
		"gathered",
		"step:implement:generate",
		"executor-start:implement:default executor",
		"executor-done:default executor",
		"step:verify:verify",
	}
	if got := strings.Join(reporter.events, "|"); got != strings.Join(want, "|") {
		t.Errorf("events = %v, want %v", reporter.events, want)
	}
	if len(reporter.timeouts) != 1 || reporter.timeouts[0] != 90*time.Second {
		t.Errorf("reported timeouts = %v, want [1m30s] from the Executor's own advertised deadline", reporter.timeouts)
	}
}

// TestReporter_ExecutorFailureIsNarratedWhenItHappens covers the case that
// left a run silent for fifteen minutes across three rounds: each failed
// call is reported at the moment it fails, so the cause of round 1 is
// visible even after round 3 has replaced it in the Act's final error.
func TestReporter_ExecutorFailureIsNarratedWhenItHappens(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1, Target: "implement"},
	}

	exec := &captureExecutor{
		patches: []string{"", ""},
		errs:    []error{errors.New("boom-1"), errors.New("boom-2")},
	}
	reporter := &observingReporter{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err == nil {
		t.Fatal("Run with a generate Step failing every attempt returned nil error")
	}

	joined := strings.Join(reporter.events, "|")
	for _, want := range []string{"executor-failed:default executor:boom-1", "executor-failed:default executor:boom-2"} {
		if !strings.Contains(joined, want) {
			t.Errorf("events = %v, want them to include %q", reporter.events, want)
		}
	}
}

// TestReporter_ModelIDNamesTheExecutorInNarration verifies narration names
// the model a Step actually routed to, rather than a hardcoded vendor.
func TestReporter_ModelIDNamesTheExecutorInNarration(t *testing.T) {
	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	registry := engine.NewExecutorRegistry()
	if err := registry.Register("gemini", &timeoutExecutor{patch: "a-patch"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Model: "gemini-3.1-pro"},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	reporter := &observingReporter{}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, &timeoutExecutor{patch: "unused"}, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(engine.NewRouter(registry, &timeoutExecutor{patch: "unused"}).WithModels(models))
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if got := strings.Join(reporter.events, "|"); !strings.Contains(got, "executor-start:implement:gemini-3.1-pro") {
		t.Errorf("events = %v, want the resolved Model ID named", reporter.events)
	}
}

// TestMultiReporter_FansOutTheOptionalExtensions guards the composition
// NewReporter builds whenever FOUNDRY_LOG is set: a MultiReporter satisfies
// only Reporter, so unless it forwards the extension events itself, every
// Step, Executor, and failover event is silently dropped for the exact
// caller that asked for more observability, not less.
func TestMultiReporter_FansOutTheOptionalExtensions(t *testing.T) {
	a, b := &observingReporter{}, &observingReporter{}
	m := engine.MultiReporter{Reporters: []engine.Reporter{a, b}}

	m.StepStarting(1, 1, 2, "implement", "generate")
	m.ExecutorStarting("implement", "opus", time.Minute)
	m.ExecutorFinished("implement", "opus", time.Second, nil)
	m.ModelFailover("implement", "opus", "sonnet", model.FailureClass("unavailable"), errors.New("down"))

	want := "step:implement:generate|executor-start:implement:opus|executor-done:opus|failover:opus->sonnet"
	for _, r := range []*observingReporter{a, b} {
		if got := strings.Join(r.events, "|"); got != want {
			t.Errorf("events = %q, want %q", got, want)
		}
	}
}

// TestMultiReporter_CloseReachesEveryCloser verifies the composed Reporter
// releases what its children hold open (a ProgressReporter's heartbeat
// goroutine) and tolerates children that hold nothing.
func TestMultiReporter_CloseReachesEveryCloser(t *testing.T) {
	closer := &closingReporter{}
	m := engine.MultiReporter{Reporters: []engine.Reporter{closer, &observingReporter{}}}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closer.closed {
		t.Error("Close did not reach the composed Reporter that implements io.Closer")
	}
}

type closingReporter struct {
	observingReporter
	closed bool
}

func (r *closingReporter) Close() error {
	r.closed = true
	return nil
}

// TestPipelineStrategy_ResumableGenerateFailureIsNotRecordedAsTerminal pins
// the boundary between the two endings a repair-exhausted generate Step can
// have. When an earlier attempt already completed a Step, a checkpoint
// exists and the Act is resumable — it must come back as a bare error with
// no Act, exactly as before, so it is never both recorded and resumable
// (the write-once Record would reject the second Write, and
// session.TestRunPipelineCommand_SavesCheckpointOnInterruption pins the
// checkpoint side of this deliberately). Only an Act where nothing
// completed at all is terminal and worth recording.
func TestPipelineStrategy_ResumableGenerateFailureIsNotRecordedAsTerminal(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1, Target: "implement"},
	}

	// Attempt 1 produces a patch that fails verification (both Steps
	// complete, so a checkpoint exists); attempt 2's generate Step errors.
	exec := &captureExecutor{
		patches: []string{"a-patch", ""},
		errs:    []error{nil, errors.New("claude: timed out after 5m0s")},
	}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", pipeline)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with a failing repair-round generate Step returned nil error")
	}
	if errors.Is(err, engine.ErrExecuteFailed) {
		t.Errorf("error = %v, must not claim a terminal disposition: this Act is still resumable from its checkpoint", err)
	}
	if act != nil {
		t.Errorf("Run returned an Act (%s) for a resumable interruption, want none: recording it would give one Act ID two terminal destinies", act.ID)
	}
}

// TestReporter_IntentAndGatheredContextAreNarrated covers what a run never
// showed while it was running: what it was asked to do, and how much
// Context was assembled around that Intent. The Intent used to surface only
// in the summary block a successful run prints at the end — so a failing
// run, the one worth reading, never restated it at all.
func TestReporter_IntentAndGatheredContextAreNarrated(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "bugfix",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	reporter := &observingReporter{}
	gatherer := &fakeGatherer{files: []string{"aaaa", "bb"}}
	eng := engine.NewEngine(gatherer, &timeoutExecutor{patch: "a-patch"}, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "add a health endpoint"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if reporter.intent != "add a health endpoint" {
		t.Errorf("declared Intent = %q, want the Intent's own text", reporter.intent)
	}
	if reporter.gatheredEntries != 2 || reporter.gatheredBytes != 6 {
		t.Errorf("gathered = %d entries / %d bytes, want 2 / 6", reporter.gatheredEntries, reporter.gatheredBytes)
	}
	// The Intent must be stated before anything is gathered or executed: it
	// is the frame for everything printed after it.
	if got := reporter.events[0]; got != "intent" {
		t.Errorf("first event = %q, want the Intent declared first", got)
	}
}
