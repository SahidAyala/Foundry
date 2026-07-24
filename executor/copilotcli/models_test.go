package copilotcli_test

import (
	"testing"

	"foundry/executor/copilotcli"
	"foundry/model"
)

func TestSupportedModels_RegistersDefaultUnderCopilotExecutor(t *testing.T) {
	models := copilotcli.SupportedModels()
	if len(models) != 1 {
		t.Fatalf("SupportedModels() returned %d entries, want 1", len(models))
	}
	if models[0].Executor != "copilot" {
		t.Errorf("Executor = %q, want %q", models[0].Executor, "copilot")
	}
	if models[0].ID == "" {
		t.Error("model has an empty ID")
	}
}

// TestSupportedModels_CapabilitiesLimitsQualityStayUnrated covers
// ADR-0013's third increment (Proposed): unlike executor/claude's,
// executor/geminicli's, and executor/openai's own confidently-rated
// entries, copilot-default's underlying model is not confirmed, so its
// Capabilities/Limits/Quality must stay at the zero "unrated" value
// rather than guess.
func TestSupportedModels_CapabilitiesLimitsQualityStayUnrated(t *testing.T) {
	got := copilotcli.SupportedModels()[0]
	if got.Capabilities != (model.Capabilities{}) {
		t.Errorf("Capabilities = %+v, want the zero value", got.Capabilities)
	}
	if got.Limits != (model.Limits{}) {
		t.Errorf("Limits = %+v, want the zero value", got.Limits)
	}
	if got.Quality != (model.Quality{}) {
		t.Errorf("Quality = %+v, want the zero value", got.Quality)
	}
}
