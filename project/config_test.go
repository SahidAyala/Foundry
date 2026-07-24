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

func TestLoadConfig_DecodesJiraFields(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{
		"ticket_provider": "jira",
		"jira_base_url": "https://example.atlassian.net",
		"jira_email": "someone@example.com",
		"jira_api_token_env": "JIRA_API_TOKEN"
	}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.JiraBaseURL != "https://example.atlassian.net" {
		t.Errorf("JiraBaseURL = %q, want %q", config.JiraBaseURL, "https://example.atlassian.net")
	}
	if config.JiraEmail != "someone@example.com" {
		t.Errorf("JiraEmail = %q, want %q", config.JiraEmail, "someone@example.com")
	}
	if config.JiraAPITokenEnv != "JIRA_API_TOKEN" {
		t.Errorf("JiraAPITokenEnv = %q, want %q", config.JiraAPITokenEnv, "JIRA_API_TOKEN")
	}
}

func TestLoadConfig_DecodesAsanaAPITokenEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{
		"ticket_provider": "asana",
		"asana_api_token_env": "ASANA_API_TOKEN"
	}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.TicketProvider != "asana" {
		t.Errorf("TicketProvider = %q, want %q", config.TicketProvider, "asana")
	}
	if config.AsanaAPITokenEnv != "ASANA_API_TOKEN" {
		t.Errorf("AsanaAPITokenEnv = %q, want %q", config.AsanaAPITokenEnv, "ASANA_API_TOKEN")
	}
}

func TestLoadConfig_DecodesAIReviewFields(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, filepath.Join(root, ".foundry"), "config.json", `{
		"ai_review_model": "gpt-5.1",
		"ai_review_base_url": "https://api.openai.com/v1/chat/completions",
		"ai_review_api_key_env": "OPENAI_API_KEY"
	}`)

	config, err := project.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.AIReviewModel != "gpt-5.1" {
		t.Errorf("AIReviewModel = %q, want %q", config.AIReviewModel, "gpt-5.1")
	}
	if config.AIReviewBaseURL != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("AIReviewBaseURL = %q, want %q", config.AIReviewBaseURL, "https://api.openai.com/v1/chat/completions")
	}
	if config.AIReviewAPIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("AIReviewAPIKeyEnv = %q, want %q", config.AIReviewAPIKeyEnv, "OPENAI_API_KEY")
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
