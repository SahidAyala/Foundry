package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
