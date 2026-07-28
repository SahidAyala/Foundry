package engine_test

import (
	"strings"
	"testing"

	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
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

// TestRouter_PreferredResolvesFirstEntryWithNoAvailabilityCheck covers
// ADR-0013's fourth increment (Proposed): a Step naming Preferred resolves
// through Preferred[0] only — the requirement is "simply select the first
// item," with no probing of whether it's actually reachable.
func TestRouter_PreferredResolvesFirstEntryWithNoAvailabilityCheck(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	first := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("opus-vendor", first); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-opus-4-8", Executor: "opus-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "opus-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.Resolve(engine.Step{
		ID: "plan", Kind: "generate",
		Preferred: []string{"claude-opus-4-8", "claude-sonnet-5", "gemini-3.1-pro"},
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != first {
		t.Error("Resolve on a Preferred step did not resolve the first entry's Executor")
	}
}

// TestRouter_PreferredFirstEntryUnknownFailsWithoutTryingLaterEntries
// covers the explicit requirement: no availability check, no fallback to
// a later Preferred entry, no retries — the first entry being unregistered
// is a validation failure, not a reason to try the second.
func TestRouter_PreferredFirstEntryUnknownFailsWithoutTryingLaterEntries(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	second := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("sonnet-vendor", second); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "sonnet-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	// Deliberately no "claude-opus-4-8" registered — the first Preferred
	// entry — even though the second entry is registered and would
	// otherwise resolve fine.

	router := engine.NewRouter(executors, def).WithModels(models)

	_, err := router.Resolve(engine.Step{
		ID: "plan", Kind: "generate",
		Preferred: []string{"claude-opus-4-8", "claude-sonnet-5"},
	})
	if err == nil {
		t.Fatal("Resolve with an unregistered first Preferred entry returned nil error")
	}
	if !strings.Contains(err.Error(), "claude-opus-4-8") {
		t.Errorf("error = %q, want it to name the unregistered first entry", err.Error())
	}
}

// TestRouter_PreferredWinsOverModelWhenBothSet covers the precedence rule:
// Preferred[0] wins over Model when both are set on the same Step.
func TestRouter_PreferredWinsOverModelWhenBothSet(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	fromPreferred := &captureExecutor{}
	fromModel := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("opus-vendor", fromPreferred); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("planner", fromModel); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-opus-4-8", Executor: "opus-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.Resolve(engine.Step{
		ID: "plan", Kind: "generate",
		Model:     "gemini-3.5-flash",
		Preferred: []string{"claude-opus-4-8"},
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != fromPreferred {
		t.Error("Resolve with both Preferred and Model set did not let Preferred win")
	}
}

// TestRouter_EmptyPreferredFallsThroughToModel covers backward
// compatibility for an empty (but present, e.g. "preferred": []) Preferred
// list — it must be treated exactly like an absent one, falling through
// to Model.
func TestRouter_EmptyPreferredFallsThroughToModel(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	fromModel := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("planner", fromModel); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.5-flash", Executor: "planner"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.Resolve(engine.Step{
		ID: "plan", Kind: "generate",
		Model:     "gemini-3.5-flash",
		Preferred: []string{},
	})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if got != fromModel {
		t.Error("Resolve with an empty Preferred list did not fall through to Model")
	}
}

// TestRouter_ResolveModel_ResolvesRegisteredModelToItsExecutor covers
// ADR-0013's sixth increment (Proposed): ResolveModel resolves a specific
// Model ID standalone, the same lookup Resolve performs internally — used
// by automatic model failover to resolve each subsequent Preferred entry.
func TestRouter_ResolveModel_ResolvesRegisteredModelToItsExecutor(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	pinned := &captureExecutor{}
	def := &captureExecutor{}
	if err := executors.Register("gemini-vendor", pinned); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.ResolveModel("gemini-3.1-pro")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if got != pinned {
		t.Error("ResolveModel did not return the Executor the Model Registry names")
	}
}

func TestRouter_ResolveModel_UnknownModelFails(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def).WithModels(model.NewRegistry())

	_, err := router.ResolveModel("does-not-exist")
	if err == nil {
		t.Fatal("ResolveModel with an unregistered model returned nil error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the unknown model", err.Error())
	}
}

func TestRouter_ResolveModel_NoRegistryConfiguredFails(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def)

	_, err := router.ResolveModel("gemini-3.1-pro")
	if err == nil {
		t.Fatal("ResolveModel with no model.Registry configured returned nil error")
	}
}

// TestRouter_ModelInfo_ReturnsRegisteredInfo covers ADR-0013's seventh
// increment (Proposed): ModelInfo lets a caller inspect a candidate's
// catalog Capabilities without resolving it to an Executor at all — used
// by capability-aware model resolution.
func TestRouter_ModelInfo_ReturnsRegisteredInfo(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}

	models := model.NewRegistry()
	want := model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor", Capabilities: model.Capabilities{ToolUse: true}}
	if err := models.Register(want); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)

	got, err := router.ModelInfo("claude-sonnet-5")
	if err != nil {
		t.Fatalf("ModelInfo failed: %v", err)
	}
	if got != want {
		t.Errorf("ModelInfo = %+v, want %+v", got, want)
	}
}

func TestRouter_ModelInfo_UnknownModelFails(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def).WithModels(model.NewRegistry())

	_, err := router.ModelInfo("does-not-exist")
	if err == nil {
		t.Fatal("ModelInfo with an unregistered model returned nil error")
	}
}

func TestRouter_ModelInfo_NoRegistryConfiguredFails(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def)

	_, err := router.ModelInfo("claude-sonnet-5")
	if err == nil {
		t.Fatal("ModelInfo with no model.Registry configured returned nil error")
	}
}

func TestRouter_ModelHealth_NoRegistryConfiguredReturnsUnknown(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def)

	got := router.ModelHealth("claude-sonnet-5")
	if got.Status != model.StatusUnknown {
		t.Errorf("ModelHealth with no model.Registry configured = %q, want %q", got.Status, model.StatusUnknown)
	}
}

func TestRouter_ModelHealth_NoHealthManagerAttachedReturnsUnknown(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	router := engine.NewRouter(executors, def).WithModels(model.NewRegistry())

	got := router.ModelHealth("claude-sonnet-5")
	if got.Status != model.StatusUnknown {
		t.Errorf("ModelHealth with a model.Registry but no HealthManager = %q, want %q", got.Status, model.StatusUnknown)
	}
}

func TestRouter_ModelHealth_DelegatesToAttachedHealthManager(t *testing.T) {
	executors := engine.NewExecutorRegistry()
	def := &captureExecutor{}
	registry := model.NewRegistry()
	health := model.NewHealthManager()
	if err := health.Report("claude-sonnet-5", model.Health{Status: model.StatusUnavailable, Reason: "rate limited"}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	registry.SetHealthManager(health)
	router := engine.NewRouter(executors, def).WithModels(registry)

	got := router.ModelHealth("claude-sonnet-5")
	if got.Status != model.StatusUnavailable {
		t.Errorf("ModelHealth = %q, want %q", got.Status, model.StatusUnavailable)
	}
	if got.Reason != "rate limited" {
		t.Errorf("ModelHealth Reason = %q, want %q", got.Reason, "rate limited")
	}
}
