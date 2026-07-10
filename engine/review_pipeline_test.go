package engine_test

import (
	"context"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// This file validates that the architecture RFC-0002 §9 Phase 3 built is
// genuinely extensible: adding a second built-in Pipeline — "review"
// (generate, then two independent verify Steps, no bounded repair) —
// required no change to Engine, Strategy, Pipeline, PipelineProvider, or
// PipelineRegistry. Every assertion below runs against the unmodified
// production types; none of it depends on any code written for this file.

// wantReviewShape is "review"'s declared shape, mirrored here (not
// imported from builtin_provider.go) so a future accidental edit to
// pipelines/review.json is caught as a test failure rather than silently
// changing behavior no test pins.
func wantReviewShape() engine.Pipeline {
	return engine.Pipeline{
		Name: "review",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "verify-again", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 0},
	}
}

// TestBuiltinProvider_LoadsBothBuiltinPipelines verifies BuiltinProvider
// now returns two Pipelines — "default" first (preserving every existing
// test's index-0 assumption), then "review" — decoded from two separate
// embedded documents.
func TestBuiltinProvider_LoadsBothBuiltinPipelines(t *testing.T) {
	pipelines, err := (engine.BuiltinProvider{}).Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("Load() returned %d pipelines, want 2: %+v", len(pipelines), pipelines)
	}

	if pipelines[0].Name != "default" {
		t.Errorf("pipelines[0].Name = %q, want %q", pipelines[0].Name, "default")
	}
	if pipelines[1].Name != "review" {
		t.Errorf("pipelines[1].Name = %q, want %q", pipelines[1].Name, "review")
	}
}

// TestBuiltinProvider_ReviewDocumentMatchesProvenShape verifies the
// embedded review.json decodes to exactly the shape
// strategy_test.go's TestPipelineStrategy_CustomPipelineRunsWithoutEngineChanges
// already proved executes correctly through an unmodified Engine and
// PipelineStrategy — the new built-in document ships data, not a new
// execution path.
func TestBuiltinProvider_ReviewDocumentMatchesProvenShape(t *testing.T) {
	pipelines, err := (engine.BuiltinProvider{}).Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var got engine.Pipeline
	found := false
	for _, p := range pipelines {
		if p.Name == "review" {
			got = p
			found = true
		}
	}
	if !found {
		t.Fatalf("Load() = %+v, want it to include a Pipeline named %q", pipelines, "review")
	}

	want := wantReviewShape()
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("Steps = %+v, want %+v", got.Steps, want.Steps)
	}
	for i, w := range want.Steps {
		if got.Steps[i] != w {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], w)
		}
	}
	if got.Repair != want.Repair {
		t.Errorf("Repair = %+v, want %+v", got.Repair, want.Repair)
	}
}

// TestNewDefaultRegistry_RegistersReviewAlongsideDefault verifies both
// built-in Pipelines are independently resolvable from the same registry,
// and that resolving "review" leaves "default"'s registered shape
// untouched — Pipeline selection and coexistence, not just Load.
func TestNewDefaultRegistry_RegistersReviewAlongsideDefault(t *testing.T) {
	registry := engine.NewDefaultRegistry()

	gotDefault, err := registry.Get("default")
	if err != nil {
		t.Fatalf("Get(\"default\") failed: %v", err)
	}
	if !pipelinesEqual(gotDefault, engine.DefaultPipeline()) {
		t.Errorf("default Pipeline = %+v, want %+v", gotDefault, engine.DefaultPipeline())
	}

	gotReview, err := registry.Get("review")
	if err != nil {
		t.Fatalf("Get(\"review\") failed: %v", err)
	}
	if !pipelinesEqual(gotReview, wantReviewShape()) {
		t.Errorf("review Pipeline = %+v, want %+v", gotReview, wantReviewShape())
	}

	// Fetching "review" again after "default" must still return default's
	// original shape — proving one name's lookup does not disturb another's
	// stored entry.
	gotDefaultAgain, err := registry.Get("default")
	if err != nil {
		t.Fatalf("second Get(\"default\") failed: %v", err)
	}
	if !pipelinesEqual(gotDefaultAgain, engine.DefaultPipeline()) {
		t.Errorf("default Pipeline changed after fetching review: got %+v", gotDefaultAgain)
	}
}

// TestReviewPipeline_ExecutesThroughUnmodifiedEngine proves the
// declarative "review" Pipeline — fetched exactly as a composition root
// would — drives one generate call and two verify calls, in order, with
// the final Judgment taken from the last verify Step, purely by walking
// the unmodified PipelineStrategy loop. No code exists anywhere that
// special-cases the name "review" or a three-Step Pipeline.
func TestReviewPipeline_ExecutesThroughUnmodifiedEngine(t *testing.T) {
	review, err := engine.NewDefaultRegistry().Get("review")
	if err != nil {
		t.Fatalf("Get(\"review\") failed: %v", err)
	}

	exec := &captureExecutor{patches: []string{scriptedPatch}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", review)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "review this"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(exec.calls) != 1 {
		t.Errorf("Executor called %d times, want 1 (one generate Step, no repair)", len(exec.calls))
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q (the second verify Step's verdict)", act.JudgmentVerdict, "pass")
	}

	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindVerify}
	if len(act.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", act.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if act.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, act.Steps[i].Kind, want)
		}
	}
}

// TestReviewPipeline_NoRepairEvenWhenFinalVerdictFails proves RepairPolicy
// is read per-Pipeline, not enforced as a global Engine constant: with
// "review"'s repair.max_attempts: 0, a failing final verdict never
// triggers a second Executor call — unlike "default", which repairs once
// (engine_test.go's TestEngine_Run_RepairAfterFailPasses). If the Engine
// hardcoded "always attempt one repair," this test would fail.
func TestReviewPipeline_NoRepairEvenWhenFinalVerdictFails(t *testing.T) {
	review, err := engine.NewDefaultRegistry().Get("review")
	if err != nil {
		t.Fatalf("Get(\"review\") failed: %v", err)
	}

	exec := &captureExecutor{patches: []string{scriptedPatch}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "pass"},
		{Verdict: "fail", Checked: []string{"lint: fail"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "", review)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "review this"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(exec.calls) != 1 {
		t.Errorf("Executor called %d times, want 1 (review's RepairPolicy allows zero repair rounds)", len(exec.calls))
	}
	if act.JudgmentVerdict != "fail" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "fail")
	}
	if act.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", act.Iterations)
	}
}

// TestDefaultAndReviewPipelines_DoNotContaminateEachOther runs "default"
// and "review" through independent Engines built from the same registry,
// including a repair round on "default", and verifies each Pipeline's
// execution — call counts, iterations, final verdict — reflects only its
// own declared shape. It also proves mutating a Pipeline value obtained
// from the registry for one run can never leak into the other's Get.
func TestDefaultAndReviewPipelines_DoNotContaminateEachOther(t *testing.T) {
	registry := engine.NewDefaultRegistry()

	defaultPipeline, err := registry.Get("default")
	if err != nil {
		t.Fatalf("Get(\"default\") failed: %v", err)
	}
	// Tamper with the caller's own copy; this must never be visible to a
	// later Get of either name.
	defaultPipeline.Steps[0].Kind = "tampered"

	reviewPipeline, err := registry.Get("review")
	if err != nil {
		t.Fatalf("Get(\"review\") failed: %v", err)
	}

	freshDefault, err := registry.Get("default")
	if err != nil {
		t.Fatalf("re-Get(\"default\") failed: %v", err)
	}
	if freshDefault.Steps[0].Kind == "tampered" {
		t.Fatal("mutating a Pipeline returned for \"default\" affected the registry's stored copy")
	}

	defaultExec := &captureExecutor{patches: []string{"default-first", "default-repaired"}}
	defaultVerifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	defaultEngine := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, defaultExec, defaultVerifier, "", freshDefault)

	reviewExec := &captureExecutor{patches: []string{"review-only"}}
	reviewVerifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"lint: fail"}},
		{Verdict: "fail", Checked: []string{"security: fail"}},
	}}
	reviewEngine := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, reviewExec, reviewVerifier, "", reviewPipeline)

	defaultAct, err := defaultEngine.Run(context.Background(), &domain.Intent{Text: "default run"})
	if err != nil {
		t.Fatalf("default Run failed: %v", err)
	}
	reviewAct, err := reviewEngine.Run(context.Background(), &domain.Intent{Text: "review run"})
	if err != nil {
		t.Fatalf("review Run failed: %v", err)
	}

	if len(defaultExec.calls) != 2 {
		t.Errorf("default Executor called %d times, want 2 (initial + one bounded repair)", len(defaultExec.calls))
	}
	if defaultAct.JudgmentVerdict != "pass" {
		t.Errorf("default JudgmentVerdict = %q, want %q", defaultAct.JudgmentVerdict, "pass")
	}

	if len(reviewExec.calls) != 1 {
		t.Errorf("review Executor called %d times, want 1 (no repair, regardless of default's repair round)", len(reviewExec.calls))
	}
	if reviewAct.JudgmentVerdict != "fail" {
		t.Errorf("review JudgmentVerdict = %q, want %q", reviewAct.JudgmentVerdict, "fail")
	}
	if len(reviewAct.Steps) != 3 {
		t.Errorf("review Steps = %+v, want 3 entries (generate, verify, verify-again)", reviewAct.Steps)
	}
}

// pipelinesEqual compares two Pipelines field-by-field (Step slices via
// index), avoiding any dependency on Pipeline's internal representation
// beyond its exported fields.
func pipelinesEqual(a, b engine.Pipeline) bool {
	if a.Name != b.Name || a.Repair != b.Repair || len(a.Steps) != len(b.Steps) {
		return false
	}
	for i := range a.Steps {
		if a.Steps[i] != b.Steps[i] {
			return false
		}
	}
	return true
}
