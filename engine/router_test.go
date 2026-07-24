package engine_test

import (
	"strings"
	"testing"

	"foundry/engine"
	"foundry/model"
)

func TestRouter_UnpinnedStepResolvesToDefault(t *testing.T) {
	registry := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(registry, def)

	got, err := router.Resolve(engine.Step{ID: "generate", Kind: "generate"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != def {
		t.Error("Resolve on an unpinned step did not return the default Executor")
	}
}

func TestRouter_PinnedStepResolvesToRegisteredExecutor(t *testing.T) {
	registry := engine.NewExecutorRegistry()
	pinned := &captureExecutor{}
	def := &captureExecutor{}
	if err := registry.Register("openai-gpt5", pinned); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	router := engine.NewRouter(registry, def)

	got, err := router.Resolve(engine.Step{ID: "generate", Kind: "generate", Executor: "openai-gpt5"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != pinned {
		t.Error("Resolve on a pinned step did not return the pinned Executor")
	}
	if got == def {
		t.Error("Resolve on a pinned step returned the default Executor instead of the pin")
	}
}

func TestRouter_PinnedStepUnregisteredFailsWithoutFallback(t *testing.T) {
	registry := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(registry, def)

	_, err := router.Resolve(engine.Step{ID: "generate", Kind: "generate", Executor: "nonexistent"})
	if err == nil {
		t.Fatal("Resolve with a pin naming an unregistered Executor returned nil error")
	}
}

// TestRouter_ModelResolvesThroughRegistryToExecutor covers ADR-0013
// (Proposed): a Step naming Model is resolved by looking it up in the
// attached model.Registry to find its Executor, then resolving that name
// against the same ExecutorRegistry Executor pins already use — no new
// routing decision, the same registry.Get call as before.
func TestRouter_ModelResolvesThroughRegistryToExecutor(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	pinned := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("planner", pinned); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.Resolve(engine.Step{ID: "plan", Kind: "generate", Model: "gemini-3.5-flash"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != pinned {
		t.Error("Resolve on a Model-pinned step did not return the Executor the Model Registry names")
	}
}

// TestRouter_ModelWinsOverExecutorWhenBothSet covers the explicit
// precedence rule: if both Model and Executor are set, Model wins.
func TestRouter_ModelWinsOverExecutorWhenBothSet(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	fromModel := &captureExecutor{}
	fromExecutorPin := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("planner", fromModel); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("openai-gpt5", fromExecutorPin); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.Resolve(engine.Step{ID: "plan", Kind: "generate", Executor: "openai-gpt5", Model: "gemini-3.5-flash"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != fromModel {
		t.Error("Resolve with both Model and Executor set did not let Model win")
	}
}

// TestRouter_UnknownModelFailsValidationWithoutFallback covers the
// explicit requirement: an unknown model fails, it does not fall back to
// Executor or the default.
func TestRouter_UnknownModelFailsValidationWithoutFallback(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	models := model.NewRegistry()
	router := engine.NewRouter(executors, def).WithModels(models)

	_, err := router.Resolve(engine.Step{ID: "plan", Kind: "generate", Model: "does-not-exist"})
	if err == nil {
		t.Fatal("Resolve with an unregistered Model returned nil error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the unknown model", err.Error())
	}
}

// TestRouter_ModelSetWithoutRegistryFails covers a Router that never had
// WithModels called on it: Model can never resolve, so it must fail
// clearly rather than silently falling back to Executor or the default.
func TestRouter_ModelSetWithoutRegistryFails(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def)

	_, err := router.Resolve(engine.Step{ID: "plan", Kind: "generate", Model: "gemini-3.5-flash"})
	if err == nil {
		t.Fatal("Resolve with Model set but no model.Registry configured returned nil error")
	}
}

// TestRouter_UnknownExecutorBehaviorUnchangedByModelFeature covers the
// explicit requirement: unknown executor continues existing behavior —
// the Model feature's addition to Resolve must not alter this pre-existing
// error path at all.
func TestRouter_UnknownExecutorBehaviorUnchangedByModelFeature(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def).WithModels(model.NewRegistry())

	_, err := router.Resolve(engine.Step{ID: "generate", Kind: "generate", Executor: "nonexistent"})
	if err == nil {
		t.Fatal("Resolve with a pin naming an unregistered Executor returned nil error")
	}
}
