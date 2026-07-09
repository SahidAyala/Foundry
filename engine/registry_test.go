package engine_test

import (
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

func testPipeline(name string) engine.Pipeline {
	return engine.Pipeline{
		Name: name,
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 1},
	}
}

func TestPipelineRegistry_RegisterThenGet(t *testing.T) {
	registry := engine.NewPipelineRegistry()

	if err := registry.Register(testPipeline("plan")); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := registry.Get("plan")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "plan" {
		t.Errorf("Name = %q, want %q", got.Name, "plan")
	}
	if len(got.Steps) != 1 || got.Steps[0].ID != "generate" {
		t.Errorf("Steps = %v, want [generate]", got.Steps)
	}
}

func TestPipelineRegistry_DuplicateRegistrationFails(t *testing.T) {
	registry := engine.NewPipelineRegistry()

	if err := registry.Register(testPipeline("plan")); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	err := registry.Register(testPipeline("plan"))
	if err == nil {
		t.Fatal("second Register with the same name returned nil error")
	}
	if !strings.Contains(err.Error(), "plan") {
		t.Errorf("error = %q, want it to name the duplicate %q", err.Error(), "plan")
	}

	// The first registration must survive the refused second one.
	got, getErr := registry.Get("plan")
	if getErr != nil {
		t.Fatalf("Get after duplicate registration failed: %v", getErr)
	}
	if len(got.Steps) != 1 {
		t.Errorf("Steps = %v, want the original single-Step Pipeline untouched", got.Steps)
	}
}

func TestPipelineRegistry_LookupFailureForUnknownName(t *testing.T) {
	registry := engine.NewPipelineRegistry()
	if err := registry.Register(testPipeline("plan")); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err := registry.Get("does-not-exist")
	if err == nil {
		t.Fatal("Get for an unregistered name returned nil error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the missing pipeline %q", err.Error(), "does-not-exist")
	}
}

func TestPipelineRegistry_LookupFailureOnEmptyRegistry(t *testing.T) {
	registry := engine.NewPipelineRegistry()

	_, err := registry.Get("default")
	if err == nil {
		t.Fatal("Get on an empty registry returned nil error")
	}
}

// TestPipelineRegistry_ImmutableAfterConstruction verifies that neither
// Register nor Get leaks a mutable reference into the registry's stored
// state: mutating a Pipeline handed to Register, or a Pipeline handed back
// by Get, must never change what a later Get call returns.
func TestPipelineRegistry_ImmutableAfterConstruction(t *testing.T) {
	registry := engine.NewPipelineRegistry()

	original := testPipeline("plan")
	if err := registry.Register(original); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	// Mutate the caller's copy after registering it.
	original.Steps[0].Kind = "tampered"

	got, err := registry.Get("plan")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Steps[0].Kind == "tampered" {
		t.Error("Register stored a reference to the caller's Steps slice; mutating it after Register changed the registry")
	}

	// Mutate the Pipeline Get just returned.
	got.Steps[0].Kind = "also-tampered"
	got.Steps = append(got.Steps, engine.Step{ID: "extra", Kind: "extra"})

	again, err := registry.Get("plan")
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if len(again.Steps) != 1 {
		t.Errorf("Steps = %v, want the original single Step; Get leaked a mutable reference", again.Steps)
	}
	if again.Steps[0].Kind != domain.StepKindGenerate {
		t.Errorf("Steps[0].Kind = %q, want %q; a prior Get's mutation reached the registry", again.Steps[0].Kind, domain.StepKindGenerate)
	}
}

// TestPipelineRegistry_MultipleNamedPipelinesCoexist verifies the registry
// is not limited to a single entry: distinct names resolve independently,
// and looking one up leaves the others (including the built-in "default")
// exactly as registered.
func TestPipelineRegistry_MultipleNamedPipelinesCoexist(t *testing.T) {
	registry := engine.NewDefaultRegistry()

	plan := engine.Pipeline{
		Name:   "plan",
		Steps:  []engine.Step{{ID: "plan", Kind: domain.StepKindGenerate}},
		Repair: engine.RepairPolicy{MaxAttempts: 0},
	}
	review := engine.Pipeline{
		Name: "review",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
			{ID: "verify-again", Kind: domain.StepKindVerify},
		},
		Repair: engine.RepairPolicy{MaxAttempts: 2},
	}
	if err := registry.Register(plan); err != nil {
		t.Fatalf("Register(plan) failed: %v", err)
	}
	if err := registry.Register(review); err != nil {
		t.Fatalf("Register(review) failed: %v", err)
	}

	gotDefault, err := registry.Get("default")
	if err != nil {
		t.Fatalf("Get(default) failed: %v", err)
	}
	if len(gotDefault.Steps) != len(engine.DefaultPipeline().Steps) {
		t.Errorf("default Pipeline changed shape after registering others: Steps = %v", gotDefault.Steps)
	}

	gotPlan, err := registry.Get("plan")
	if err != nil {
		t.Fatalf("Get(plan) failed: %v", err)
	}
	if len(gotPlan.Steps) != 1 || gotPlan.Steps[0].ID != "plan" {
		t.Errorf("plan Pipeline = %+v, want its own single Step", gotPlan)
	}
	if gotPlan.Repair.MaxAttempts != 0 {
		t.Errorf("plan Repair.MaxAttempts = %d, want 0", gotPlan.Repair.MaxAttempts)
	}

	gotReview, err := registry.Get("review")
	if err != nil {
		t.Fatalf("Get(review) failed: %v", err)
	}
	if len(gotReview.Steps) != 3 {
		t.Errorf("review Pipeline Steps = %v, want 3", gotReview.Steps)
	}
	if gotReview.Repair.MaxAttempts != 2 {
		t.Errorf("review Repair.MaxAttempts = %d, want 2", gotReview.Repair.MaxAttempts)
	}

	// Every name registered under this run is independently resolvable.
	for _, name := range []string{"default", "plan", "review"} {
		if _, err := registry.Get(name); err != nil {
			t.Errorf("Get(%q) failed after registering all three: %v", name, err)
		}
	}
}

func TestNewDefaultRegistry_RegistersDefaultPipeline(t *testing.T) {
	registry := engine.NewDefaultRegistry()

	got, err := registry.Get("default")
	if err != nil {
		t.Fatalf("Get(\"default\") failed: %v", err)
	}

	want := engine.DefaultPipeline()
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("Steps = %v, want %v", got.Steps, want.Steps)
	}
	for i := range got.Steps {
		if got.Steps[i] != want.Steps[i] {
			t.Errorf("Steps[%d] = %+v, want %+v", i, got.Steps[i], want.Steps[i])
		}
	}
	if got.Repair != want.Repair {
		t.Errorf("Repair = %+v, want %+v", got.Repair, want.Repair)
	}
}
