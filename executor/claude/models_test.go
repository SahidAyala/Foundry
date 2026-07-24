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

// TestSupportedModels_ExposeCapabilitiesLimitsQuality covers ADR-0013's
// third increment (Proposed): every entry carries real (hand-curated,
// non-zero) Capabilities/Limits/Quality metadata, not left at the zero
// "unrated" value — unlike executor/copilotcli's own deliberately unrated
// single entry, Claude's models are well enough known to rate honestly.
func TestSupportedModels_ExposeCapabilitiesLimitsQuality(t *testing.T) {
	for _, m := range claude.SupportedModels() {
		if !m.Capabilities.ToolUse {
			t.Errorf("model %q: Capabilities.ToolUse = false, want true", m.ID)
		}
		if m.Limits.MaxContext == 0 {
			t.Errorf("model %q: Limits.MaxContext = 0, want a real context window", m.ID)
		}
		if m.Quality.Coding == 0 {
			t.Errorf("model %q: Quality.Coding = 0, want a real rating", m.ID)
		}
	}
}
