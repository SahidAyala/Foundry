package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"foundry/project"
)

func TestLoadConfig_MissingFileReturnsZeroValue(t *testing.T) {
	config, err := project.LoadConfig(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.DocsPath != "" {
		t.Errorf("LoadConfig() = %+v, want a zero Config for a missing file", config)
	}
}

func TestLoadConfig_DecodesValidFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{"docs_path": "docs/decisions.md"}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.DocsPath != "docs/decisions.md" {
		t.Errorf("DocsPath = %q, want %q", config.DocsPath, "docs/decisions.md")
	}
}

func TestLoadConfig_MalformedFileFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{not valid json`)

	_, err := project.LoadConfig(root)
	if err == nil {
		t.Fatal("LoadConfig with a malformed file returned nil error")
	}
}
