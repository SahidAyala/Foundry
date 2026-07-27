package openai_test

import (
	"testing"

	"github.com/SahidAyala/Foundry/executor/openai"
)

func TestSupportedModels_CoversBothOpenAIVendors(t *testing.T) {
	models := openai.SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels() returned no entries")
	}

	byExecutor := map[string]int{}
	for _, m := range models {
		if m.ID == "" {
			t.Error("model has an empty ID")
		}
		if m.Executor != "openai" && m.Executor != "openai-compatible" {
			t.Errorf("model %q has unexpected Executor %q", m.ID, m.Executor)
		}
		byExecutor[m.Executor]++
	}
	if byExecutor["openai"] == 0 {
		t.Error("no model registered under the \"openai\" executor")
	}
	if byExecutor["openai-compatible"] == 0 {
		t.Error("no model registered under the \"openai-compatible\" executor")
	}
}

// TestSupportedModels_ExposeCapabilitiesLimitsQuality covers ADR-0013's
// third increment (Proposed): every entry carries real (hand-curated,
// non-zero) Limits/Quality metadata — Capabilities varies genuinely by
// model here (e.g. llama3 has no confirmed tool_use support), so this
// test checks Limits/Quality only, not every Capabilities field.
func TestSupportedModels_ExposeCapabilitiesLimitsQuality(t *testing.T) {
	for _, m := range openai.SupportedModels() {
		if m.Limits.MaxContext == 0 {
			t.Errorf("model %q: Limits.MaxContext = 0, want a real context window", m.ID)
		}
		if m.Quality.Coding == 0 {
			t.Errorf("model %q: Quality.Coding = 0, want a real rating", m.ID)
		}
	}
}
