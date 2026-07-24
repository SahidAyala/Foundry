package claude_test

import (
	"testing"

	"foundry/executor/claude"
)

func TestSupportedModels_AllBelongToClaudeExecutor(t *testing.T) {
	models := claude.SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels() returned no entries")
	}
	for _, m := range models {
		if m.Executor != "claude" {
			t.Errorf("model %q has Executor %q, want %q", m.ID, m.Executor, "claude")
		}
		if m.ID == "" {
			t.Error("model has an empty ID")
		}
	}
}
