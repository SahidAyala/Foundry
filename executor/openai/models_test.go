package openai_test

import (
	"testing"

	"foundry/executor/openai"
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
