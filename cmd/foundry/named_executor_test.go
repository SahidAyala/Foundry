package main

import (
	"strings"
	"testing"

	"foundry/executor/gemini"
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

// TestNamedExecutor_GeminiVendorConstructsGeminiExecutor confirms
// namedExecutor's vendor dispatch recognizes "gemini" (executor/gemini's
// addition alongside "openai") — no new architectural decision was needed
// for this, per ADR-0005/ADR-0006 already covering any number of named
// vendors.
func TestNamedExecutor_GeminiVendorConstructsGeminiExecutor(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_GEMINI_KEY", "test-key-value")

	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:    "gemini",
		Model:     "gemini-3.5-flash",
		APIKeyEnv: "FOUNDRY_TEST_GEMINI_KEY",
	})
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*gemini.Executor); !ok {
		t.Errorf("namedExecutor(vendor=gemini) = %T, want *gemini.Executor", exec)
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
