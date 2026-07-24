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

// TestSupportedModels_ExposeCapabilitiesLimitsQuality covers ADR-0013's
// third increment (Proposed): every Gemini entry carries real
// (hand-curated, non-zero) Capabilities/Limits/Quality metadata.
func TestSupportedModels_ExposeCapabilitiesLimitsQuality(t *testing.T) {
	for _, m := range geminicli.SupportedModels() {
		if !m.Capabilities.Multimodal {
			t.Errorf("model %q: Capabilities.Multimodal = false, want true", m.ID)
		}
		if m.Limits.MaxContext == 0 {
			t.Errorf("model %q: Limits.MaxContext = 0, want a real context window", m.ID)
		}
		if m.Quality.Reasoning == 0 {
			t.Errorf("model %q: Quality.Reasoning = 0, want a real rating", m.ID)
		}
	}
}
