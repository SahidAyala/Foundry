package project_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foundry/project"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile(%q) failed: %v", name, err)
	}
}

func TestFilesystemPipelineSource_MissingDirectoryReturnsNoPipelines(t *testing.T) {
	source := project.FilesystemPipelineSource{Dir: filepath.Join(t.TempDir(), "does-not-exist")}

	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 0 {
		t.Errorf("Load() = %+v, want no pipelines for a missing directory", pipelines)
	}
}

func TestFilesystemPipelineSource_EmptyDirectoryReturnsNoPipelines(t *testing.T) {
	source := project.FilesystemPipelineSource{Dir: t.TempDir()}

	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 0 {
		t.Errorf("Load() = %+v, want no pipelines for an empty directory", pipelines)
	}
}

func TestFilesystemPipelineSource_DecodesValidDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "feature.json", `{
		"name": "feature",
		"steps": [
			{"id": "generate", "kind": "generate"},
			{"id": "verify", "kind": "verify"}
		],
		"repair": {"max_attempts": 1}
	}`)

	source := project.FilesystemPipelineSource{Dir: dir}
	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("Load() = %+v, want exactly 1 pipeline", pipelines)
	}
	if pipelines[0].Name != "feature" {
		t.Errorf("Name = %q, want %q", pipelines[0].Name, "feature")
	}
	if len(pipelines[0].Steps) != 2 {
		t.Errorf("Steps = %+v, want 2 entries", pipelines[0].Steps)
	}
}

func TestFilesystemPipelineSource_MalformedDocumentFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.json", `{not valid json`)

	source := project.FilesystemPipelineSource{Dir: dir}
	_, err := source.Load(context.Background())
	if err == nil {
		t.Fatal("Load with a malformed document returned nil error")
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Errorf("error = %q, want it to name the offending file %q", err.Error(), "broken.json")
	}
}

func TestFilesystemPipelineSource_MultipleDocumentsLoadInSortedOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "zeta.json", `{"name": "zeta", "steps": [{"id": "generate", "kind": "generate"}]}`)
	writeFile(t, dir, "alpha.json", `{"name": "alpha", "steps": [{"id": "generate", "kind": "generate"}]}`)

	source := project.FilesystemPipelineSource{Dir: dir}
	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("Load() = %+v, want 2 pipelines", pipelines)
	}
	if pipelines[0].Name != "alpha" || pipelines[1].Name != "zeta" {
		t.Errorf("Load() order = [%s, %s], want [alpha, zeta]", pipelines[0].Name, pipelines[1].Name)
	}
}

func TestFilesystemPipelineSource_IgnoresNonJSONFilesAndSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "not a pipeline")
	writeFile(t, dir, "feature.json", `{"name": "feature", "steps": [{"id": "generate", "kind": "generate"}]}`)
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	writeFile(t, filepath.Join(dir, "nested"), "ignored.json", `{"name": "ignored", "steps": [{"id": "generate", "kind": "generate"}]}`)

	source := project.FilesystemPipelineSource{Dir: dir}
	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(pipelines) != 1 || pipelines[0].Name != "feature" {
		t.Errorf("Load() = %+v, want exactly the 1 top-level *.json pipeline", pipelines)
	}
}
