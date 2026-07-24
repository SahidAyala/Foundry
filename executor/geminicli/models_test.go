package geminicli_test

import (
	"testing"

	"foundry/executor/geminicli"
)

func TestSupportedModels_AllBelongToGeminiExecutor(t *testing.T) {
	models := geminicli.SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels() returned no entries")
	}
	for _, m := range models {
		if m.Executor != "gemini" {
			t.Errorf("model %q has Executor %q, want %q", m.ID, m.Executor, "gemini")
		}
		if m.Provider != "Google" {
			t.Errorf("model %q has Provider %q, want %q", m.ID, m.Provider, "Google")
		}
	}
}
