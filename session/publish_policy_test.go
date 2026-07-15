package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"foundry/session"
)

// writeConfig writes root's .foundry/config.json with the given raw JSON
// content, creating the .foundry directory first.
func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".foundry"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".foundry", "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
}

// writePipeline writes a Pipeline document to root's project-local
// pipelines directory.
func writePipeline(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".foundry", "pipelines")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write pipeline document: %v", err)
	}
}

// TestNewSession_RequireApprovalBeforeRemotePublish_RefusesUnguardedPipeline
// proves ADR-0010's publish policy actually reaches NewSession end to end:
// a project that sets require_approval_before_remote_publish and authors a
// Pipeline declaring a "remote-pr" apply Step with no preceding approve
// Step fails to start the session at all, with a clear error — never a
// session that silently drops the offending Pipeline or defers the failure
// to whenever that Pipeline is first run.
func TestNewSession_RequireApprovalBeforeRemotePublish_RefusesUnguardedPipeline(t *testing.T) {
	root := initGitRepo(t)
	writeConfig(t, root, `{"require_approval_before_remote_publish": true}`)
	writePipeline(t, root, "publish.json", `{
		"name": "publish",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"},
			{"id": "ship", "kind": "apply", "target": "remote-pr"}
		]
	}`)

	_, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err == nil {
		t.Fatal("NewSession with an unguarded remote-pr Pipeline returned nil error")
	}
}

// TestNewSession_RequireApprovalBeforeRemotePublish_AllowsGuardedPipeline
// is the companion positive case: the same policy, but the Pipeline
// declares an approve Step before its remote-pr apply Step, so NewSession
// starts normally.
func TestNewSession_RequireApprovalBeforeRemotePublish_AllowsGuardedPipeline(t *testing.T) {
	root := initGitRepo(t)
	writeConfig(t, root, `{"require_approval_before_remote_publish": true}`)
	writePipeline(t, root, "publish.json", `{
		"name": "publish",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"},
			{"id": "approve", "kind": "approve"},
			{"id": "ship", "kind": "apply", "target": "remote-pr"}
		]
	}`)

	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if _, err := s.Engine("publish"); err != nil {
		t.Errorf(`Engine("publish") failed: %v`, err)
	}
}

// TestNewSession_NoPublishPolicy_AllowsUnguardedRemotePRPipeline verifies
// the default (require_approval_before_remote_publish absent, or false):
// a Pipeline declaring remote-pr with no approve Step loads exactly as it
// did before this policy existed.
func TestNewSession_NoPublishPolicy_AllowsUnguardedRemotePRPipeline(t *testing.T) {
	root := initGitRepo(t)
	writePipeline(t, root, "publish.json", `{
		"name": "publish",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"},
			{"id": "ship", "kind": "apply", "target": "remote-pr"}
		]
	}`)

	s, err := session.NewSession(context.Background(), root, &bytes.Buffer{}, &bytes.Buffer{}, newScriptedExecutorFactory(scriptedPatch))
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if _, err := s.Engine("publish"); err != nil {
		t.Errorf(`Engine("publish") failed: %v`, err)
	}
}
