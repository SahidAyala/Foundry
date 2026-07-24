package main

import "testing"

func TestBuildModelRegistry_RegistersEveryExecutorsModelsWithoutConflict(t *testing.T) {
	registry, err := buildModelRegistry()
	if err != nil {
		t.Fatalf("buildModelRegistry failed: %v", err)
	}

	for _, executor := range []string{"claude", "gemini", "openai", "openai-compatible", "copilot"} {
		if models := registry.ByExecutor(executor); len(models) == 0 {
			t.Errorf("ByExecutor(%q) returned no models", executor)
		}
	}

	for _, id := range []string{"claude-sonnet-5", "gemini-3.5-flash", "gpt-5.1", "llama3", "copilot-default"} {
		if _, err := registry.Get(id); err != nil {
			t.Errorf("Get(%q) failed: %v", id, err)
		}
	}
}

func TestBuildModelRegistry_ListIsSortedAndNonEmpty(t *testing.T) {
	registry, err := buildModelRegistry()
	if err != nil {
		t.Fatalf("buildModelRegistry failed: %v", err)
	}

	models := registry.List()
	if len(models) == 0 {
		t.Fatal("List() returned no models")
	}
	for i := 1; i < len(models); i++ {
		if models[i-1].ID >= models[i].ID {
			t.Errorf("List() not sorted at index %d: %q >= %q", i, models[i-1].ID, models[i].ID)
		}
	}
}
