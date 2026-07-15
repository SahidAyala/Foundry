package engine_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// TestGoldenFeaturePipeline_Decodes pins the target shape for a realistic
// "feature" Pipeline — RFC-0002 §4.3's own worked example, Plan → Approval →
// Implementation → Verification → Repair → Approval → Apply → Record —
// against the actual decoder, so a change to DecodePipelineDocument's
// accepted vocabulary or RepairPolicy's named repair target is caught here
// first. All five Step kinds are executable by PipelineStrategy as of PR D
// (RFC-0002 §9 Phase 4's Engine-side work is complete); this test only
// proves the document decodes to the right static shape — a full run
// diffed against testdata/golden/feature-act.json is a separate test, not
// this one, since decoding and executing are independent concerns.
func TestGoldenFeaturePipeline_Decodes(t *testing.T) {
	data, err := os.ReadFile("testdata/golden/feature-pipeline.json")
	if err != nil {
		t.Fatalf("read golden feature pipeline: %v", err)
	}

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}

	if got.Name != "feature" {
		t.Errorf("Name = %q, want %q", got.Name, "feature")
	}
	wantSteps := []engine.Step{
		{ID: "plan", Kind: domain.StepKindGenerate},
		{ID: "approve-plan", Kind: domain.StepKindApprove},
		{ID: "implement", Kind: domain.StepKindGenerate},
		{ID: "verify", Kind: domain.StepKindVerify},
		{ID: "approve-outcome", Kind: domain.StepKindApprove},
		{ID: "apply", Kind: domain.StepKindApply},
		{ID: "record", Kind: domain.StepKindRecord},
	}
	if len(got.Steps) != len(wantSteps) {
		t.Fatalf("Steps = %+v, want %d entries", got.Steps, len(wantSteps))
	}
	for i, want := range wantSteps {
		if !reflect.DeepEqual(got.Steps[i], want) {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], want)
		}
	}
	if got.Repair.MaxAttempts != 2 {
		t.Errorf("Repair.MaxAttempts = %d, want 2", got.Repair.MaxAttempts)
	}
	if got.Repair.Target != "implement" {
		t.Errorf("Repair.Target = %q, want %q", got.Repair.Target, "implement")
	}
}

// TestGoldenFeaturePipeline_ExecutesFullLifecycleWithOneRepairRound runs the
// golden "feature" Pipeline end to end — the same document
// .foundry/pipelines/feature.json ships in this repo — through
// PipelineStrategy with fakes standing in for every port, simulating one
// failing verify Step that repairs and then passes. It pins the exact Step
// trace testdata/golden/feature-act.json illustrates: plan, approve-plan,
// implement (fails), verify (fail), implement (repaired), verify (pass),
// approve-outcome, apply, record. This is the test that would have caught
// PR E's real bug: without stopsShortOnFailure (strategy.go), the first
// failing verify would have let approve-outcome/apply/record run against a
// rejected Outcome before ever reaching repair.
func TestGoldenFeaturePipeline_ExecutesFullLifecycleWithOneRepairRound(t *testing.T) {
	data, err := os.ReadFile("testdata/golden/feature-pipeline.json")
	if err != nil {
		t.Fatalf("read golden feature pipeline: %v", err)
	}
	pipeline, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}

	gatherer := &fakeGatherer{files: []string{"reports/handler.go", "reports/handler_test.go"}}
	exec := &captureExecutor{patches: []string{
		"diff --git a/reports/plan.md b/reports/plan.md\n+plan: add a CSV export endpoint\n",
		"diff --git a/reports/handler.go b/reports/handler.go\n+// export as CSV (attempt 1)\n",
		"diff --git a/reports/handler.go b/reports/handler.go\n+// export as CSV\n",
	}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: undefined: csv.NewExporter"}},
		{Verdict: "pass", Checked: []string{"build: pass", "tests: pass"}},
	}}
	authority := &fakeAuthority{authority: "sahid.ayala@vtwo.co", approved: true}
	applier := &fakeApplier{}
	checkpointer := &fakeCheckpointer{}

	eng := engine.NewEngine(gatherer, exec, verifier, "the-workspace", pipeline)
	eng.SetAuthority(authority)
	eng.SetApplier(applier)
	eng.SetCheckpointer(checkpointer)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "Add CSV export to the reports page"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.ApprovedBy != "sahid.ayala@vtwo.co" {
		t.Errorf("ApprovedBy = %q, want %q", act.ApprovedBy, "sahid.ayala@vtwo.co")
	}
	if act.ApprovedAt == nil {
		t.Error("ApprovedAt is nil, want a timestamp")
	}
	if act.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2 (one repair round)", act.Iterations)
	}
	if len(exec.calls) != 3 {
		t.Fatalf("Executor called %d times, want 3 (plan, implement attempt 1, implement attempt 2)", len(exec.calls))
	}
	if applier.calls != 1 {
		t.Errorf("Applier.Apply called %d times, want 1", applier.calls)
	}
	if len(checkpointer.calls) != 1 {
		t.Errorf("Checkpointer.Write called %d times, want 1", len(checkpointer.calls))
	}

	wantKinds := []string{
		domain.StepKindGenerate, // plan
		domain.StepKindApprove,  // approve-plan
		domain.StepKindGenerate, // implement, attempt 1 (fails)
		domain.StepKindVerify,   // verify (fail)
		domain.StepKindGenerate, // implement, repaired
		domain.StepKindVerify,   // verify (pass)
		domain.StepKindApprove,  // approve-outcome
		domain.StepKindApply,
		domain.StepKindRecord,
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
