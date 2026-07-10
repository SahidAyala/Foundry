package project_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"foundry/project"
)

// writeDocument writes a Pipeline document to dir/name, creating dir
// (including any missing parents) first.
func writeDocument(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func TestProjectLoader_LoadRegistry_OnlyBuiltinsWhenNoProjectDir(t *testing.T) {
	registry, err := project.ProjectLoader{}.LoadRegistry(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	if _, err := registry.Get("default"); err != nil {
		t.Errorf("Get(\"default\") failed: %v", err)
	}
	if _, err := registry.Get("review"); err != nil {
		t.Errorf("Get(\"review\") failed: %v", err)
	}
}

func TestProjectLoader_LoadRegistry_IncludesProjectLocalPipelines(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, project.PipelinesDir)
	if err := writeDocument(dir, "feature.json", `{
		"name": "feature",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"}
		],
		"repair": {"max_attempts": 1}
	}`); err != nil {
		t.Fatalf("writeDocument failed: %v", err)
	}

	registry, err := project.ProjectLoader{}.LoadRegistry(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	got, err := registry.Get("feature")
	if err != nil {
		t.Fatalf("Get(\"feature\") failed: %v", err)
	}
	if len(got.Steps) != 2 {
		t.Errorf("feature Steps = %+v, want 2 entries", got.Steps)
	}

	// The built-ins must still be present alongside the project-local one.
	if _, err := registry.Get("default"); err != nil {
		t.Errorf("Get(\"default\") failed after loading a project-local pipeline: %v", err)
	}
}

func TestProjectLoader_LoadRegistry_NameCollisionWithBuiltinFails(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, project.PipelinesDir)
	if err := writeDocument(dir, "default.json", `{
		"name": "default",
		"steps": [{"id": "generate", "kind": "generate"}]
	}`); err != nil {
		t.Fatalf("writeDocument failed: %v", err)
	}

	_, err := project.ProjectLoader{}.LoadRegistry(context.Background(), root)
	if err == nil {
		t.Fatal("LoadRegistry with a project pipeline named \"default\" returned nil error")
	}
}
