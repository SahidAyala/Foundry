package copilotcli_test

import (
	"testing"

	"foundry/executor/copilotcli"
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
