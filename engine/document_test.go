package engine_test

import (
	"context"
	"reflect"
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
		if !reflect.DeepEqual(got.Steps[i], want) {
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

func TestDecodePipelineDocument_ApproveApplyRecordKindsDecode(t *testing.T) {
	data := []byte(`{
		"name": "rich",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "approve", "kind": "approve"},
			{"id": "verify", "kind": "verify"},
			{"id": "apply", "kind": "apply"},
			{"id": "record", "kind": "record"}
		]
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	wantKinds := []string{
		domain.StepKindGenerate, domain.StepKindApprove, domain.StepKindVerify,
		domain.StepKindApply, domain.StepKindRecord,
	}
	if len(got.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries", got.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if got.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, got.Steps[i].Kind, want)
		}
	}
}

func TestDecodePipelineDocument_OmittedRouterFieldsDecodeToZeroValues(t *testing.T) {
	data := []byte(`{
		"name": "no-router-fields",
		"steps": [{"id": "generate", "kind": "generate"}]
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	step := got.Steps[0]
	if step.Capability != nil {
		t.Errorf("Capability = %#v, want nil", step.Capability)
	}
	if step.Executor != "" {
		t.Errorf("Executor = %q, want empty string", step.Executor)
	}
	if step.FeedsForward {
		t.Error("FeedsForward = true, want false")
	}
	if step.Target != "" {
		t.Errorf("Target = %q, want empty string", step.Target)
	}
}

func TestDecodePipelineDocument_CapabilityExecutorFeedsForwardDecode(t *testing.T) {
	data := []byte(`{
		"name": "routed",
		"steps": [
			{
				"id": "generate",
				"kind": "generate",
				"capability": {"vendor": "openai"},
				"executor": "openai-gpt5",
				"feeds_forward": true
			}
		]
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	step := got.Steps[0]
	wantCapability := map[string]string{"vendor": "openai"}
	if !reflect.DeepEqual(step.Capability, wantCapability) {
		t.Errorf("Capability = %#v, want %#v", step.Capability, wantCapability)
	}
	if step.Executor != "openai-gpt5" {
		t.Errorf("Executor = %q, want %q", step.Executor, "openai-gpt5")
	}
	if !step.FeedsForward {
		t.Error("FeedsForward = false, want true")
	}
}

func TestDecodePipelineDocument_ApplyTargetDecode(t *testing.T) {
	data := []byte(`{
		"name": "knowledge-capture",
		"steps": [
			{
				"id": "note",
				"kind": "apply",
				"target": "knowledge-note"
			}
		]
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	if got.Steps[0].Target != "knowledge-note" {
		t.Errorf("Target = %q, want %q", got.Steps[0].Target, "knowledge-note")
	}
}

func TestDecodePipelineDocument_RepairTargetNamingDeclaredStepDecodes(t *testing.T) {
	data := []byte(`{
		"name": "feature",
		"steps": [
			{"id": "plan", "kind": "generate"},
			{"id": "implement", "kind": "generate"},
			{"id": "verify", "kind": "verify"}
		],
		"repair": {"max_attempts": 1, "target": "implement"}
	}`)

	got, err := engine.DecodePipelineDocument(data)
	if err != nil {
		t.Fatalf("DecodePipelineDocument failed: %v", err)
	}
	if got.Repair.Target != "implement" {
		t.Errorf("Repair.Target = %q, want %q", got.Repair.Target, "implement")
	}
}

func TestDecodePipelineDocument_RepairTargetNamingUndeclaredStepFails(t *testing.T) {
	data := []byte(`{
		"name": "bad",
		"steps": [{"id": "generate", "kind": "generate"}],
		"repair": {"max_attempts": 1, "target": "nonexistent"}
	}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with repair.target naming an undeclared step returned nil error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want it to name the undeclared target %q", err.Error(), "nonexistent")
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
// same document BuiltinPipelineSource embeds for "default" must produce a
// Pipeline identical to engine.DefaultPipeline(), byte for byte. It finds
// "default" by name rather than assuming it is the only Pipeline
// BuiltinPipelineSource loads, so a later built-in Pipeline never breaks this
// assertion about "default" specifically.
func TestDecodePipelineDocument_MatchesBuiltinDefault(t *testing.T) {
	pipelines, err := (engine.BuiltinPipelineSource{}).Load(context.Background())
	if err != nil {
		t.Fatalf("BuiltinPipelineSource.Load failed: %v", err)
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
		t.Fatalf("BuiltinPipelineSource.Load() = %+v, want it to include a Pipeline named %q", pipelines, "default")
	}

	want := engine.DefaultPipeline()
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("Steps = %v, want %v", got.Steps, want.Steps)
	}
	for i := range want.Steps {
		if !reflect.DeepEqual(got.Steps[i], want.Steps[i]) {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], want.Steps[i])
		}
	}
	if got.Repair != want.Repair {
		t.Errorf("Repair = %+v, want %+v", got.Repair, want.Repair)
	}
}

// TestBuiltinPipelineSource_LoadedPipelineRepairsIdenticallyToDefaultPipeline
// proves execution through the document-loaded Pipeline preserves repair
// semantics, not just static field equality: a failing verdict on the
// first attempt triggers exactly one bounded repair round (the embedded
// document's declared max_attempts: 1), exactly as DefaultPipeline's
// repair-path tests already pin for the hand-constructed Pipeline
// (engine_test.go).
func TestBuiltinPipelineSource_LoadedPipelineRepairsIdenticallyToDefaultPipeline(t *testing.T) {
	pipelines, err := (engine.BuiltinPipelineSource{}).Load(context.Background())
	if err != nil {
		t.Fatalf("BuiltinPipelineSource.Load failed: %v", err)
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

// The following tests cover ADR-0004 Decision 4: an unknown field anywhere
// in a Pipeline document is a decode-time error naming the field, the
// enclosing Step (when there is one), and pointing the author at
// docs/04-guides/pipelines.md — never silently dropped.

func TestDecodePipelineDocument_UnknownTopLevelFieldFails(t *testing.T) {
	data := []byte(`{"nmae": "bad", "steps": [{"id": "generate", "kind": "generate"}]}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with an unknown top-level field returned nil error")
	}
	if !strings.Contains(err.Error(), `"nmae"`) {
		t.Errorf("error = %q, want it to name the unknown field %q", err.Error(), "nmae")
	}
	if !strings.Contains(err.Error(), "top-level") {
		t.Errorf("error = %q, want it to say the field is top-level", err.Error())
	}
	if !strings.Contains(err.Error(), "docs/04-guides/pipelines.md") {
		t.Errorf("error = %q, want it to point at the pipelines guide", err.Error())
	}
}

func TestDecodePipelineDocument_UnknownStepFieldFailsAndNamesTheStep(t *testing.T) {
	data := []byte(`{
		"name": "bad",
		"steps": [{"id": "generate", "kind": "generate", "capabilty": {"vendor": "x"}}]
	}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with an unknown step field returned nil error")
	}
	if !strings.Contains(err.Error(), `"capabilty"`) {
		t.Errorf("error = %q, want it to name the unknown field %q", err.Error(), "capabilty")
	}
	if !strings.Contains(err.Error(), `"generate"`) {
		t.Errorf("error = %q, want it to name the enclosing step %q", err.Error(), "generate")
	}
	if !strings.Contains(err.Error(), "docs/04-guides/pipelines.md") {
		t.Errorf("error = %q, want it to point at the pipelines guide", err.Error())
	}
}

func TestDecodePipelineDocument_UnknownStepFieldWithoutIDFallsBackToIndex(t *testing.T) {
	data := []byte(`{"name": "bad", "steps": [{"kynd": "generate"}]}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with an unknown field on an id-less step returned nil error")
	}
	if !strings.Contains(err.Error(), `"kynd"`) {
		t.Errorf("error = %q, want it to name the unknown field %q", err.Error(), "kynd")
	}
	if !strings.Contains(err.Error(), "index 0") {
		t.Errorf("error = %q, want it to fall back to the step's index", err.Error())
	}
}

func TestDecodePipelineDocument_UnknownRepairFieldFails(t *testing.T) {
	data := []byte(`{
		"name": "bad",
		"steps": [{"id": "generate", "kind": "generate"}],
		"repair": {"max_attemtps": 1}
	}`)

	_, err := engine.DecodePipelineDocument(data)
	if err == nil {
		t.Fatal("DecodePipelineDocument with an unknown repair field returned nil error")
	}
	if !strings.Contains(err.Error(), `"max_attemtps"`) {
		t.Errorf("error = %q, want it to name the unknown field %q", err.Error(), "max_attemtps")
	}
	if !strings.Contains(err.Error(), "repair") {
		t.Errorf("error = %q, want it to say the field is in the repair block", err.Error())
	}
}
