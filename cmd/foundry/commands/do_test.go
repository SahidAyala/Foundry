package commands

import (
	"testing"

	"foundry/engine"
	"foundry/project"
)

// TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured proves
// wireEngine's buildApplierRegistry (ADR-0010, Piece 6 of
// docs/04-guides/multi-executor-router-implementation-plan.md) registers
// engine.ApplyTargetRemotePR only when project.Config names a
// RemotePublishTokenEnv, mirroring session.Session's own
// buildApplierRegistry.
func TestBuildApplierRegistry_RegistersRemotePRTargetWhenConfigured(t *testing.T) {
	registry, err := buildApplierRegistry(project.Config{RemotePublishTokenEnv: "GITHUB_PR_TOKEN"})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	if _, err := registry.Get(engine.ApplyTargetRemotePR); err != nil {
		t.Errorf("Get(%q) failed, want the remote-pr Applier registered: %v", engine.ApplyTargetRemotePR, err)
	}
}

// TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig verifies a
// project that never sets remote_publish_token_env registers no
// remote-pr Applier at all — exactly as an apply Step with that Target
// would have behaved before ADR-0010 existed.
func TestBuildApplierRegistry_NoRemotePRTargetWithoutConfig(t *testing.T) {
	registry, err := buildApplierRegistry(project.Config{})
	if err != nil {
		t.Fatalf("buildApplierRegistry failed: %v", err)
	}
	if _, err := registry.Get(engine.ApplyTargetRemotePR); err == nil {
		t.Error("Get(remote-pr) succeeded, want an error: no Applier should be registered without config")
	}
	// Knowledge-lite capture's own target still registers unconditionally.
	if _, err := registry.Get(engine.ApplyTargetKnowledgeNote); err != nil {
		t.Errorf("Get(knowledge-note) failed: %v", err)
	}
}
