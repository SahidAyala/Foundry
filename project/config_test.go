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
	if config.RequireApprovalBeforeRemotePublish {
		t.Error("RequireApprovalBeforeRemotePublish = true, want false for a missing file")
	}
	if config.RemotePublishTokenEnv != "" {
		t.Errorf("RemotePublishTokenEnv = %q, want empty for a missing file", config.RemotePublishTokenEnv)
	}
}

func TestLoadConfig_DecodesRemotePublishFields(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{
		"require_approval_before_remote_publish": true,
		"remote_publish_token_env": "GITHUB_PR_TOKEN"
	}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !config.RequireApprovalBeforeRemotePublish {
		t.Error("RequireApprovalBeforeRemotePublish = false, want true")
	}
	if config.RemotePublishTokenEnv != "GITHUB_PR_TOKEN" {
		t.Errorf("RemotePublishTokenEnv = %q, want %q", config.RemotePublishTokenEnv, "GITHUB_PR_TOKEN")
	}
}

func TestLoadConfig_DecodesTicketProvider(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{"ticket_provider": "github"}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.TicketProvider != "github" {
		t.Errorf("TicketProvider = %q, want %q", config.TicketProvider, "github")
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
