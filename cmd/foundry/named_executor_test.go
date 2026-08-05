package main

import (
	"strings"
	"testing"
	"time"

	"github.com/SahidAyala/Foundry/executor/claude"
	"github.com/SahidAyala/Foundry/executor/copilotcli"
	"github.com/SahidAyala/Foundry/executor/gemini"
	"github.com/SahidAyala/Foundry/executor/geminicli"
	"github.com/SahidAyala/Foundry/executor/openai"
	"github.com/SahidAyala/Foundry/project"
)

// TestNamedExecutor_ClaudeVendorConstructsClaudeExecutor confirms a
// project can register Claude Code as a named, model-pinned Executor
// (unlike the zero-config default Executor, which passes no model at
// all) — closing ADR-0013's own named "claude vendor can never resolve
// via Model yet" limitation, post-ratification, at the maintainer's
// direct request.
func TestNamedExecutor_ClaudeVendorConstructsClaudeExecutor(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor: "claude",
		Model:  "fable-5",
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*claude.ClaudeExecutor); !ok {
		t.Errorf("namedExecutor(vendor=claude) = %T, want *claude.ClaudeExecutor", exec)
	}
}

// TestNamedExecutor_ClaudeVendorAppliesTimeoutSeconds confirms
// TimeoutSeconds overrides executor/claude.ClaudeExecutor's own
// 5-minute default when a project's own repository or Intent needs a
// longer single Execute call than that ceiling allows.
func TestNamedExecutor_ClaudeVendorAppliesTimeoutSeconds(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:         "claude",
		TimeoutSeconds: 600,
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	claudeExec, ok := exec.(*claude.ClaudeExecutor)
	if !ok {
		t.Fatalf("namedExecutor(vendor=claude) = %T, want *claude.ClaudeExecutor", exec)
	}
	if want := 600 * time.Second; claudeExec.Timeout() != want {
		t.Errorf("Timeout() = %s, want %s", claudeExec.Timeout(), want)
	}
}

// TestNamedExecutor_ClaudeVendorKeepsDefaultTimeoutWhenUnset confirms an
// unset TimeoutSeconds (the common case) leaves ClaudeExecutor's own
// 5-minute default untouched — this field is purely additive.
func TestNamedExecutor_ClaudeVendorKeepsDefaultTimeoutWhenUnset(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{Vendor: "claude"}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	claudeExec, ok := exec.(*claude.ClaudeExecutor)
	if !ok {
		t.Fatalf("namedExecutor(vendor=claude) = %T, want *claude.ClaudeExecutor", exec)
	}
	if want := 5 * time.Minute; claudeExec.Timeout() != want {
		t.Errorf("Timeout() = %s, want the unmodified default %s", claudeExec.Timeout(), want)
	}
}

func TestNamedExecutor_OpenAIVendorConstructsOpenAIExecutor(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_OPENAI_KEY", "test-key-value")

	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:    "openai",
		Model:     "gpt-5.1",
		APIKeyEnv: "FOUNDRY_TEST_OPENAI_KEY",
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*openai.Executor); !ok {
		t.Errorf("namedExecutor(vendor=openai) = %T, want *openai.Executor", exec)
	}
}

// TestNamedExecutor_GeminiVendorConstructsGeminiCLIExecutor confirms
// namedExecutor's vendor dispatch resolves "gemini" to executor/geminicli
// (the Gemini CLI subprocess, no API key ever read by Foundry) rather than
// executor/gemini's own raw-API-key HTTP path — the maintainer's own
// framing was that a raw API key should be a last resort, not the default.
func TestNamedExecutor_GeminiVendorConstructsGeminiCLIExecutor(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor: "gemini",
		Model:  "gemini-3.5-flash",
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*geminicli.Executor); !ok {
		t.Errorf("namedExecutor(vendor=gemini) = %T, want *geminicli.Executor", exec)
	}
}

// TestNamedExecutor_GeminiAPIVendorConstructsGeminiExecutor confirms the
// explicitly-named "gemini-api" vendor still resolves to executor/gemini's
// HTTP API-key path, for environments where no browser is ever available
// to complete the Gemini CLI's one-time "Sign in with Google" login.
func TestNamedExecutor_GeminiAPIVendorConstructsGeminiExecutor(t *testing.T) {
	t.Setenv("FOUNDRY_TEST_GEMINI_KEY", "test-key-value")

	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:    "gemini-api",
		Model:     "gemini-3.5-flash",
		APIKeyEnv: "FOUNDRY_TEST_GEMINI_KEY",
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*gemini.Executor); !ok {
		t.Errorf("namedExecutor(vendor=gemini-api) = %T, want *gemini.Executor", exec)
	}
}

// TestNamedExecutor_CopilotVendorConstructsCopilotCLIExecutor confirms
// namedExecutor's vendor dispatch resolves "copilot" to executor/copilotcli
// — delegating generate Steps to the GitHub Copilot CLI, not just PR review
// (vcs.GitHubPRApplier's own RequestCopilotReview).
func TestNamedExecutor_CopilotVendorConstructsCopilotCLIExecutor(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{Vendor: "copilot"}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*copilotcli.Executor); !ok {
		t.Errorf("namedExecutor(vendor=copilot) = %T, want *copilotcli.Executor", exec)
	}
}

// TestNamedExecutor_OpenAICompatibleVendorConstructsOpenAIExecutor covers
// the general escape hatch for any Chat-Completions-compatible endpoint
// (Ollama, Groq, DeepSeek, ...) — one client reused against a caller-named
// base_url instead of a near-duplicate package per vendor.
func TestNamedExecutor_OpenAICompatibleVendorConstructsOpenAIExecutor(t *testing.T) {
	exec, err := namedExecutor(project.ExecutorConfig{
		Vendor:  "openai-compatible",
		Model:   "llama3",
		BaseURL: "http://localhost:11434/v1/chat/completions",
	}, "/repo")
	if err != nil {
		t.Fatalf("namedExecutor failed: %v", err)
	}
	if _, ok := exec.(*openai.Executor); !ok {
		t.Errorf("namedExecutor(vendor=openai-compatible) = %T, want *openai.Executor", exec)
	}
}

func TestNamedExecutor_OpenAICompatibleVendorRequiresBaseURL(t *testing.T) {
	_, err := namedExecutor(project.ExecutorConfig{Vendor: "openai-compatible", Model: "llama3"}, "/repo")
	if err == nil {
		t.Fatal("namedExecutor(vendor=openai-compatible) with no base_url returned nil error")
	}
	if !strings.Contains(err.Error(), "base_url") {
		t.Errorf("error = %q, want it to name the missing base_url field", err)
	}
}

func TestNamedExecutor_UnknownVendorFails(t *testing.T) {
	_, err := namedExecutor(project.ExecutorConfig{Vendor: "some-future-vendor"}, "/repo")
	if err == nil {
		t.Fatal("namedExecutor with an unrecognized vendor returned nil error")
	}
	if !strings.Contains(err.Error(), "some-future-vendor") {
		t.Errorf("error = %q, want it to name the unrecognized vendor", err.Error())
	}
}
