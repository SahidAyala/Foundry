package main

import (
	"strings"
	"testing"

	"foundry/executor/openai"
	"foundry/project"
)

func TestNamedExecutor_OpenAIVendorConstructsOpenAIExecutor(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_OPENAI_KEY", "test-key-value")

	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:    "openai",
		Model:     "gpt-5.1",
		APIKeyEnv: "FOUNDRY_TEST_OPENAI_KEY",
	})
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*openai.Executor); !ok {
		t.Errorf("namedExecutor(vendor=openai) = %T, want *openai.Executor", exec)
	}
}

func TestNamedExecutor_UnknownVendorFails(t *testing.T) {
	_, err := namedExecutor(project.ExecutorConfig{Vendor: "some-future-vendor"})
	if err == nil {
		t.Fatal("namedExecutor with an unrecognized vendor returned nil error")
	}
	if !strings.Contains(err.Error(), "some-future-vendor") {
		t.Errorf("error = %q, want it to name the unrecognized vendor", err.Error())
	}
}
