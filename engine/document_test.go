package engine_test

import (
	"context"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

func TestDecodePipelineDocument_ValidDocument(t *testing.T) {
	data := []byte(`{
		"name": "review",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"}
		],
		"repair": {"max_attempts": 2}
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}

	if got.Name != "review" {
		t.Errorf("Name = %q, want %q", got.Name, "review")
	}
	wantSteps := []engine.Step{
		{ID: "generate", Kind: domain.StepKindGenerate},
		{ID: "verify", Kind: domain.StepKindVerify},
	}
	if len(got.Steps) != len(wantSteps) {
		t.Fatalf("Steps = %v, want %v", got.Steps, wantSteps)
	}
	for i, want := range wantSteps {
		if got.Steps[i] != want {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], want)
		}
	}
	if got.Repair.MaxAttempts != 2 {
		t.Errorf("Repair.MaxAttempts = %d, want 2", got.Repair.MaxAttempts)
	}
}

func TestDecodePipelineDocument_OmittedRepairDefaultsToNoRepair(t *testing.T) {
	data := []byte(`{
		"name": "no-repair",
		"steps": [{"id": "generate", "kind": "generate"}]
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	if got.Repair.MaxAttempts != 0 {
		t.Errorf("Repair.MaxAttempts = %d, want 0 (the zero-value default)", got.Repair.MaxAttempts)
	}
}

func TestDecodePipelineDocument_MalformedJSONFails(t *testing.T) {
	_, err := engine.DecodePipelineDocument([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("DecodePipelineDocument with malformed JSON returned nil error")
	}
}

func TestDecodePipelineDocument_MissingNameFails(t *testing.T) {
	data := []byte(`{"steps": [{"id": "generate", "kind": "generate"}]}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with no name returned nil error")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to mention the missing name", err.Error())
	}
}

func TestDecodePipelineDocument_NoStepsFails(t *testing.T) {
	data := []byte(`{"name": "empty"}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with no steps returned nil error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want it to name the pipeline %q", err.Error(), "empty")
	}
}

func TestDecodePipelineDocument_MissingStepIDFails(t *testing.T) {
	data := []byte(`{"name": "bad", "steps": [{"kind": "generate"}]}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with a step missing an id returned nil error")
	}
}

func TestDecodePipelineDocument_UnrecognizedStepKindFails(t *testing.T) {
	data := []byte(`{"name": "bad", "steps": [{"id": "mystery", "kind": "transmute"}]}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with an unrecognized step kind returned nil error")
	}
	if !strings.Contains(err.Error(), "transmute") || !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error = %q, want it to name the unrecognized kind %q and step %q", err.Error(), "transmute", "mystery")
	}
}

func TestDecodePipelineDocument_NegativeMaxAttemptsFails(t *testing.T) {
	data := []byte(`{
		"name": "bad",
		"steps": [{"id": "generate", "kind": "generate"}],
		"repair": {"max_attempts": -1}
	}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with a negative max_attempts returned nil error")
	}
}

// TestDecodePipelineDocument_MatchesBuiltinDefault pins RFC-0002 §9 Phase
// 3's compatibility requirement directly against the loader: decoding the
// same document BuiltinProvider embeds for "default" must produce a
// Pipeline identical to engine.DefaultPipeline(), byte for byte. It finds
// "default" by name rather than assuming it is the only Pipeline
// BuiltinProvider loads, so a later built-in Pipeline never breaks this
// assertion about "default" specifically.
func TestDecodePipelineDocument_MatchesBuiltinDefault(t *testing.T) {
	pipelines, err := (engine.BuiltinProvider{}).Load(context.Background())
	if err != nil {
		t.Fatalf("BuiltinProvider.Load failed: %v", err)
	}

	var got engine.Pipeline
	found := false
	for _, p := range pipelines {
		if p.Name == "default" {
			got = p
			found = true
		}
	}
	if !found {
		t.Fatalf("BuiltinProvider.Load() = %+v, want it to include a Pipeline named %q", pipelines, "default")
	}

	want := engine.DefaultPipeline()
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("Steps = %v, want %v", got.Steps, want.Steps)
	}
	for i := range want.Steps {
		if got.Steps[i] != want.Steps[i] {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], want.Steps[i])
		}
	}
	if got.Repair != want.Repair {
		t.Errorf("Repair = %+v, want %+v", got.Repair, want.Repair)
	}
}

// TestBuiltinProvider_LoadedPipelineRepairsIdenticallyToDefaultPipeline
// proves execution through the document-loaded Pipeline preserves repair
// semantics, not just static field equality: a failing verdict on the
// first attempt triggers exactly one bounded repair round (the embedded
// document's declared max_attempts: 1), exactly as DefaultPipeline's
// repair-path tests already pin for the hand-constructed Pipeline
// (engine_test.go).
func TestBuiltinProvider_LoadedPipelineRepairsIdenticallyToDefaultPipeline(t *testing.T) {
	pipelines, err := (engine.BuiltinProvider{}).Load(context.Background())
	if err != nil {
		t.Fatalf("BuiltinProvider.Load failed: %v", err)
	}
	loaded := pipelines[0]

	exec := &captureExecutor{patches: []string{"first-patch", "second-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "pass"},
	}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "", loaded)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if len(exec.calls) != 2 {
		t.Errorf("Executor called %d times, want 2 (one initial attempt, one bounded repair)", len(exec.calls))
	}
	if act.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", act.Iterations)
	}
	if act.CostEstimateUSD <= 0 {
		t.Errorf("CostEstimateUSD = %v, want budget usage to have been charged", act.CostEstimateUSD)
	}
}
