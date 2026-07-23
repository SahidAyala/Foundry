package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/project"
)

func TestLoadExecutorConfig_MissingFileReturnsEmptyMap(t *testing.T) {
	config, err := project.LoadExecutorConfig(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadExecutorConfig failed: %v", err)
	}
	if len(config) != 0 {
		t.Errorf("LoadExecutorConfig() = %+v, want an empty map for a missing file", config)
	}
}

func TestLoadExecutorConfig_DecodesValidFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{
		"openai-gpt5": {"vendor": "openai", "model": "gpt-5", "api_key_env": "OPENAI_API_KEY"}
	}`)

	config, err := project.LoadExecutorConfig(root)
	if err != nil {
		t.Fatalf("LoadExecutorConfig failed: %v", err)
	}
	if len(config) != 1 {
		t.Fatalf("LoadExecutorConfig() = %+v, want exactly 1 entry", config)
	}
	got, ok := config["openai-gpt5"]
	if !ok {
		t.Fatalf("LoadExecutorConfig() = %+v, want an %q entry", config, "openai-gpt5")
	}
	want := project.ExecutorConfig{Vendor: "openai", Model: "gpt-5", APIKeyEnv: "OPENAI_API_KEY"}
	if got != want {
		t.Errorf("config[%q] = %+v, want %+v", "openai-gpt5", got, want)
	}
}

// TestLoadExecutorConfig_DecodesBaseURL covers the "openai-compatible"
// vendor's own field (Ollama, Groq, DeepSeek, ... — anything speaking the
// same Chat Completions shape at a different endpoint).
func TestLoadExecutorConfig_DecodesBaseURL(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{
		"local-llama": {"vendor": "openai-compatible", "model": "llama3", "base_url": "http://localhost:11434/v1/chat/completions"}
	}`)

	config, err := project.LoadExecutorConfig(root)
	if err != nil {
		t.Fatalf("LoadExecutorConfig failed: %v", err)
	}
	got, ok := config["local-llama"]
	if !ok {
		t.Fatalf("LoadExecutorConfig() = %+v, want a %q entry", config, "local-llama")
	}
	want := project.ExecutorConfig{Vendor: "openai-compatible", Model: "llama3", BaseURL: "http://localhost:11434/v1/chat/completions"}
	if got != want {
		t.Errorf("config[%q] = %+v, want %+v", "local-llama", got, want)
	}
}

func TestLoadExecutorConfig_MalformedFileFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{not valid json`)

	_, err := project.LoadExecutorConfig(root)
	if err == nil {
		t.Fatal("LoadExecutorConfig with a malformed file returned nil error")
	}
	if !strings.Contains(err.Error(), "executors.json") {
		t.Errorf("error = %q, want it to name the offending file %q", err.Error(), "executors.json")
	}
}

// stubExecutor is a minimal engine.Executor for BuildExecutorRegistry
// tests; it is never actually run.
type stubExecutor struct{ name string }

func (stubExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return nil, nil
}

func TestBuildExecutorRegistry_MissingFileReturnsEmptyRegistryEvenWithNilConstruct(t *testing.T) {
	registry, err := project.BuildExecutorRegistry(filepath.Join(t.TempDir(), "does-not-exist"), nil)
	if err != nil {
		t.Fatalf("BuildExecutorRegistry failed: %v", err)
	}
	if _, err := registry.Get("anything"); err == nil {
		t.Error("Get on an empty registry returned nil error, want it to report no such executor")
	}
}

func TestBuildExecutorRegistry_ConstructsAndRegistersEachEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{
		"openai-gpt5": {"vendor": "openai", "model": "gpt-5", "api_key_env": "OPENAI_API_KEY"}
	}`)

	var gotConfig project.ExecutorConfig
	var gotWorkspace string
	construct := func(cfg project.ExecutorConfig, workspace string) (engine.Executor, error) {
		gotConfig, gotWorkspace = cfg, workspace
		return stubExecutor{name: "constructed"}, nil
	}

	registry, err := project.BuildExecutorRegistry(root, construct)
	if err != nil {
		t.Fatalf("BuildExecutorRegistry failed: %v", err)
	}
	if gotConfig.Vendor != "openai" || gotConfig.Model != "gpt-5" {
		t.Errorf("construct received %+v, want vendor=openai model=gpt-5", gotConfig)
	}
	if gotWorkspace != root {
		t.Errorf("construct received workspace %q, want %q", gotWorkspace, root)
	}
	exec, err := registry.Get("openai-gpt5")
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", "openai-gpt5", err)
	}
	if exec.(stubExecutor).name != "constructed" {
		t.Errorf("Get(%q) returned an executor not built by construct", "openai-gpt5")
	}
}

func TestBuildExecutorRegistry_ConstructErrorIsWrappedWithName(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{
		"broken": {"vendor": "unsupported-vendor"}
	}`)

	construct := func(cfg project.ExecutorConfig, workspace string) (engine.Executor, error) {
		return nil, errors.New("unsupported vendor")
	}

	_, err := project.BuildExecutorRegistry(root, construct)
	if err == nil {
		t.Fatal("BuildExecutorRegistry with a failing construct returned nil error")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error = %q, want it to name the offending executor %q", err.Error(), "broken")
	}
}

func TestBuildExecutorRegistry_NonEmptyConfigWithNilConstructFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "executors.json", `{
		"openai-gpt5": {"vendor": "openai", "model": "gpt-5", "api_key_env": "OPENAI_API_KEY"}
	}`)

	_, err := project.BuildExecutorRegistry(root, nil)
	if err == nil {
		t.Fatal("BuildExecutorRegistry with a non-empty config and a nil construct returned nil error")
	}
}
