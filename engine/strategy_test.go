package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

// TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges pins RFC-0002
// §9 Phase 3's exit criterion: a Pipeline shaped differently from
// DefaultPipeline (here, two verify Steps run back-to-back against the same
// Outcome) executes correctly through the unmodified Engine and
// PipelineStrategy — no Engine or Strategy code was written for this
// specific shape.
func TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges(t *testing.T) {
	custom := engine.Pipeline{
		Name: "review",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "verify-again", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 0},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", custom)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	// One generate Step and two verify Steps were recorded, in order.
	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindVerify}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestPipelineStrategy_UnrecognizedStepKindFails verifies a Pipeline
// referencing a Step Kind PipelineStrategy does not recognize fails with a
// clear, named error instead of silently skipping the Step.
func TestPipelineStrategy_UnrecognizedStepKindFails(t *testing.T) {
	malformed := engine.Pipeline{
		Name: "malformed",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "mystery", Kind: "transmute"},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", malformed)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unrecognized step kind returned nil error")
	}
	if !strings.Contains(err.Error(), "transmute") || !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error = %q, want it to name the unrecognized kind %q and step %q", err.Error(), "transmute", "mystery")
	}
}

// TestPipelineStrategy_RepairJumpsToNamedTargetOnly verifies a repair round
// re-runs only from RepairPolicy.Target onward, not the whole Pipeline: a
// "plan" generate Step that already ran successfully must not re-run when
// "implement" is repaired.
func TestPipelineStrategy_RepairJumpsToNamedTargetOnly(t *testing.T) {
	targeted := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "plan", Kind: domain.StepKindGenerate},
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1, Target: "implement"},
	}

	exec := &captureExecutor{patches: []string{"plan-patch", "implement-patch-1", "implement-patch-2"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", targeted)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if len(exec.calls) != 3 {
		t.Fatalf("Executor called %d times, want 3 (plan once, implement twice)", len(exec.calls))
	}

	wantKinds := []string{
		domain.StepKindGenerate, domain.StepKindGenerate, domain.StepKindVerify,
		domain.StepKindGenerate, domain.StepKindVerify,
	}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}

	for _, considered := range exec.calls[2] {
		if strings.Contains(considered, "verification findings from the failed previous attempt") {
			return
		}
	}
	t.Errorf("implement's repair call considered = %v, want it to include the repair findings", exec.calls[2])
}

// TestPipelineStrategy_UnsetRepairTargetRestartsFromTop verifies a Pipeline
// with no RepairPolicy.Target keeps today's exact behavior: a repair round
// restarts from Pipeline.Steps[0], re-running every generate Step again.
func TestPipelineStrategy_UnsetRepairTargetRestartsFromTop(t *testing.T) {
	untargeted := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "plan", Kind: domain.StepKindGenerate},
			{ID: "implement", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1},
	}

	exec := &captureExecutor{patches: []string{"plan-1", "implement-1", "plan-2", "implement-2"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", untargeted)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if len(exec.calls) != 4 {
		t.Errorf("Executor called %d times, want 4 (plan+implement, twice each)", len(exec.calls))
	}
}

// fakeCheckpointer records each Write call and returns a canned error, if
// any — RFC-0002 §9 Phase 4's Step-kind vocabulary is now fully executable
// (generate, verify, approve, apply, record); there is no longer a "not yet
// executable" kind to test against.
type fakeCheckpointer struct {
	err   error
	calls []*domain.Act
}

func (c *fakeCheckpointer) Write(ctx context.Context, act *domain.Act) error {
	c.calls = append(c.calls, act)
	return c.err
}

// TestPipelineStrategy_RecordCheckspointsAct verifies a record Step calls
// the configured Checkpointer with the Act as it stands so far, and records
// its own StepRecord.
func TestPipelineStrategy_RecordCheckspointsAct(t *testing.T) {
	withRecord := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply},
			{ID: "record", Kind: domain.StepKindRecord},
		},
	}
	checkpointer := &fakeCheckpointer{}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withRecord)
	eng.SetAuthority(&fakeAuthority{authority: "alice", approved: true})
	eng.SetApplier(&fakeApplier{})
	eng.SetCheckpointer(checkpointer)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(checkpointer.calls) != 1 {
		t.Fatalf("Checkpointer.Write called %d times, want 1", len(checkpointer.calls))
	}
	if checkpointer.calls[0].ID != act.ID {
		t.Errorf("checkpointed Act ID = %q, want %q", checkpointer.calls[0].ID, act.ID)
	}

	wantKinds := []string{
		domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindApprove,
		domain.StepKindApply, domain.StepKindRecord,
	}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestPipelineStrategy_RecordWithoutConfiguredCheckpointerFails verifies a
// Pipeline reaching a record Step on an Engine that never had
// SetCheckpointer called fails with a clear, named error
// (engine.ErrNoCheckpointer) instead of silently no-oping.
func TestPipelineStrategy_RecordWithoutConfiguredCheckpointerFails(t *testing.T) {
	withRecord := engine.Pipeline{
		Name: "rich",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "record", Kind: domain.StepKindRecord},
		},
	}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withRecord)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unconfigured Checkpointer returned nil error")
	}
	if !errors.Is(err, engine.ErrNoCheckpointer) {
		t.Errorf("error = %v, want it to wrap engine.ErrNoCheckpointer", err)
	}
}

// fakeAuthority returns a canned decision from Decide, recording whether it
// was called at all.
type fakeAuthority struct {
	authority string
	approved  bool
	err       error
	called    bool
}

func (a *fakeAuthority) Decide(ctx context.Context, act *domain.Act) (string, bool, error) {
	a.called = true
	if a.err != nil {
		return "", false, a.err
	}
	return a.authority, a.approved, nil
}

// TestPipelineStrategy_ApproveAcceptedSetsApprovalAndContinues verifies an
// accepted approve Step records ApprovedBy/ApprovedAt on the Act, labels its
// own StepRecord "accept" with the deciding Authority, and lets the
// Pipeline continue to its remaining Steps.
func TestPipelineStrategy_ApproveAcceptedSetsApprovalAndContinues(t *testing.T) {
	withApprove := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}
	authority := &fakeAuthority{authority: "alice", approved: true}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withApprove)
	eng.SetAuthority(authority)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !authority.called {
		t.Fatal("Authority.Decide was never called")
	}
	if act.ApprovedBy != "alice" {
		t.Errorf("ApprovedBy = %q, want %q", act.ApprovedBy, "alice")
	}
	if act.ApprovedAt == nil {
		t.Error("ApprovedAt is nil, want a timestamp")
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q (the Pipeline continued to verify)", act.JudgmentVerdict, "pass")
	}

	wantKinds := []string{domain.StepKindGenerate, domain.StepKindApprove, domain.StepKindVerify}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
	if act.Steps[1].JudgmentVerdict != "accept" {
		t.Errorf("approve Step's JudgmentVerdict = %q, want %q", act.Steps[1].JudgmentVerdict, "accept")
	}
	if act.Steps[1].Authority != "alice" {
		t.Errorf("approve Step's Authority = %q, want %q", act.Steps[1].Authority, "alice")
	}
}

// TestPipelineStrategy_ApproveRejectedStopsPipeline verifies a rejected
// approve Step halts the Pipeline immediately: act.JudgmentVerdict becomes
// VerdictRejected, ApprovedBy/ApprovedAt stay unset, and no Step after the
// rejection — including one PipelineStrategy cannot yet execute at all,
// like apply — ever runs.
func TestPipelineStrategy_ApproveRejectedStopsPipeline(t *testing.T) {
	withApproveThenApply := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply},
		},
	}
	authority := &fakeAuthority{approved: false}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withApproveThenApply)
	eng.SetAuthority(authority)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v (a rejection is not an error)", err)
	}
	if act.JudgmentVerdict != engine.VerdictRejected {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, engine.VerdictRejected)
	}
	if act.ApprovedBy != "" {
		t.Errorf("ApprovedBy = %q, want empty (rejected)", act.ApprovedBy)
	}
	if act.ApprovedAt != nil {
		t.Error("ApprovedAt is set, want nil (rejected)")
	}

	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindApprove}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries (apply must never run after a rejection)", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
	if act.Steps[2].JudgmentVerdict != "reject" {
		t.Errorf("approve Step's JudgmentVerdict = %q, want %q", act.Steps[2].JudgmentVerdict, "reject")
	}
}

// TestPipelineStrategy_ApproveWithoutConfiguredAuthorityFails verifies a
// Pipeline reaching an approve Step on an Engine that never had
// SetAuthority called fails with a clear, named error (engine.ErrNoAuthority)
// instead of silently approving or rejecting on the caller's behalf.
func TestPipelineStrategy_ApproveWithoutConfiguredAuthorityFails(t *testing.T) {
	withApprove := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "approve", Kind: domain.StepKindApprove},
		},
	}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withApprove)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unconfigured Authority returned nil error")
	}
	if !errors.Is(err, engine.ErrNoAuthority) {
		t.Errorf("error = %v, want it to wrap engine.ErrNoAuthority", err)
	}
}

// TestPipelineStrategy_ApproveWithoutPrecedingGenerateFails verifies an
// approve Step with no generate Step before it fails with a clear error
// instead of calling the Authority over a nonexistent Outcome.
func TestPipelineStrategy_ApproveWithoutPrecedingGenerateFails(t *testing.T) {
	approveOnly := engine.Pipeline{
		Name:  "approve-only",
		Steps: []engine.Step{{ID: "approve", Kind: domain.StepKindApprove}},
	}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", approveOnly)
	eng.SetAuthority(&fakeAuthority{approved: true})

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an approve-only pipeline returned nil error")
	}
	if !strings.Contains(err.Error(), "approve") {
		t.Errorf("error = %q, want it to mention the approve step", err.Error())
	}
}

// fakeApplier records each Apply call and returns a canned error, if any.
type fakeApplier struct {
	err   error
	calls int
}

func (a *fakeApplier) Apply(ctx context.Context, workspace string, act *domain.Act) error {
	a.calls++
	return a.err
}

// TestPipelineStrategy_ApplyRunsAfterAcceptedApprove verifies an apply Step
// calls the configured Applier once an approve Step has accepted, and
// records its own StepRecord.
func TestPipelineStrategy_ApplyRunsAfterAcceptedApprove(t *testing.T) {
	withApplyAfterApprove := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply},
		},
	}
	applier := &fakeApplier{}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "the-workspace", withApplyAfterApprove)
	eng.SetAuthority(&fakeAuthority{authority: "alice", approved: true})
	eng.SetApplier(applier)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if applier.calls != 1 {
		t.Errorf("Applier.Apply called %d times, want 1", applier.calls)
	}

	wantKinds := []string{
		domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindApprove, domain.StepKindApply,
	}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestPipelineStrategy_ApplyWithoutAcceptedApproveFails verifies a Pipeline
// reaching an apply Step with no preceding accepted approve Step fails with
// a clear error instead of applying an unapproved Outcome.
func TestPipelineStrategy_ApplyWithoutAcceptedApproveFails(t *testing.T) {
	applyWithNoApprove := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "apply", Kind: domain.StepKindApply},
		},
	}
	applier := &fakeApplier{}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", applyWithNoApprove)
	eng.SetApplier(applier)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an apply step and no preceding approve returned nil error")
	}
	if !strings.Contains(err.Error(), "apply") || !strings.Contains(err.Error(), "approve") {
		t.Errorf("error = %q, want it to mention both the apply step and the missing approve", err.Error())
	}
	if applier.calls != 0 {
		t.Errorf("Applier.Apply called %d times, want 0 (never called without an accepted approve)", applier.calls)
	}
}

// TestPipelineStrategy_ApplyWithoutConfiguredApplierFails verifies a
// Pipeline reaching an apply Step on an Engine that never had SetApplier
// called fails with a clear, named error (engine.ErrNoApplier) instead of
// silently no-oping.
func TestPipelineStrategy_ApplyWithoutConfiguredApplierFails(t *testing.T) {
	withApply := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply},
		},
	}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", withApply)
	eng.SetAuthority(&fakeAuthority{approved: true})

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with an unconfigured Applier returned nil error")
	}
	if !errors.Is(err, engine.ErrNoApplier) {
		t.Errorf("error = %v, want it to wrap engine.ErrNoApplier", err)
	}
}

// TestPipelineStrategy_TrustStepsSkippedAfterFailingVerify verifies a
// failing verify Step stops the current attempt before any approve, apply,
// or record Step — a Pipeline must never seek approval for, apply, or
// record an Outcome its own Verify Step just rejected, even with no repair
// configured to retry it.
func TestPipelineStrategy_TrustStepsSkippedAfterFailingVerify(t *testing.T) {
	richWithNoRepair := engine.Pipeline{
		Name: "feature",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "approve", Kind: domain.StepKindApprove},
			{ID: "apply", Kind: domain.StepKindApply},
			{ID: "record", Kind: domain.StepKindRecord},
		},
	}
	authority := &fakeAuthority{approved: true}
	applier := &fakeApplier{}
	checkpointer := &fakeCheckpointer{}
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "fail"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", richWithNoRepair)
	eng.SetAuthority(authority)
	eng.SetApplier(applier)
	eng.SetCheckpointer(checkpointer)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "fail" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "fail")
	}
	if authority.called {
		t.Error("Authority.Decide was called after a failing verify, want it never called")
	}
	if applier.calls != 0 {
		t.Errorf("Applier.Apply called %d times, want 0", applier.calls)
	}
	if len(checkpointer.calls) != 0 {
		t.Errorf("Checkpointer.Write called %d times, want 0", len(checkpointer.calls))
	}

	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries (approve/apply/record never ran)", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestPipelineStrategy_VerifyWithoutPrecedingGenerateFails verifies a
// verify Step with no generate Step before it fails with a clear error
// instead of handing a nil Outcome to the Verifier port.
func TestPipelineStrategy_VerifyWithoutPrecedingGenerateFails(t *testing.T) {
	verifyOnly := engine.Pipeline{
		Name:  "verify-only",
		Steps: []engine.Step{{ID: "verify", Kind: domain.StepKindVerify}},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", verifyOnly)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with a verify-only pipeline returned nil error")
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Errorf("error = %q, want it to mention the verify step", err.Error())
	}
}

// TestPipelineStrategy_NoVerifyStepFails verifies a Pipeline with no verify
// Step at all — which can never produce a Judgment — fails with a clear
// error instead of a nil-pointer panic.
func TestPipelineStrategy_NoVerifyStepFails(t *testing.T) {
	generateOnly := engine.Pipeline{
		Name:  "generate-only",
		Steps: []engine.Step{{ID: "generate", Kind: domain.StepKindGenerate}},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", generateOnly)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run with a generate-only pipeline returned nil error")
	}
	if !strings.Contains(err.Error(), "generate-only") {
		t.Errorf("error = %q, want it to name the pipeline %q", err.Error(), "generate-only")
	}
}
