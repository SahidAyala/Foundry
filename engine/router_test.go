package engine_test

import (
	"testing"

	"foundry/engine"
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
