package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"foundry/engine"
)

// PipelinesDir is the conventional location, relative to a project root,
// where FilesystemPipelineSource reads and ProjectLoader.Scaffold
// writes project-local Pipeline documents.
const PipelinesDir = ".foundry/pipelines"

// ProjectLoader resolves a project's full set of Pipelines — every
// built-in Pipeline this build of Foundry ships, plus every Pipeline the
// project has authored for itself — and scaffolds a fresh project's
// pipelines directory for /init. It owns no Engine, Strategy, or
// PipelineRegistry logic of its own; it only composes what
// engine.BuiltinPipelineSource, FilesystemPipelineSource, and
// engine.PipelineRegistry already do.
type ProjectLoader struct{}

// LoadRegistry returns a PipelineRegistry populated first with every
// built-in Pipeline (engine.BuiltinPipelineSource), then every Pipeline
// authored under root's pipelines directory (FilesystemPipelineSource)
// — the same RegisterMany composition engine.NewDefaultRegistry already
// uses for built-ins alone, generalized by one more source. A
// project-local Pipeline whose name collides with a built-in surfaces
// PipelineRegistry.RegisterMany's existing duplicate-name error; a
// collision is never silently resolved in either direction.
//
// cfg's RequireApprovalBeforeRemotePublish is applied via
// PipelineRegistry.SetPublishPolicy before either source's Pipelines are
// registered (ADR-0010,
// docs/03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md Decision
// 3) — this is the one place a project-authored Pipeline declaring a
// "remote-pr" apply Step is registered at all, so it is the one place that
// policy can take effect.
func (ProjectLoader) LoadRegistry(ctx context.Context, root string, cfg Config) (*engine.PipelineRegistry, error) {
	registry := engine.NewPipelineRegistry()
	registry.SetPublishPolicy(cfg.RequireApprovalBeforeRemotePublish)

	builtins, err := engine.BuiltinPipelineSource{}.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: load built-in pipelines: %w", err)
	}
	if err := registry.RegisterMany(builtins...); err != nil {
		return nil, fmt.Errorf("project: register built-in pipelines: %w", err)
	}

	local, err := FilesystemPipelineSource{Dir: pipelinesDir(root)}.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: load project pipelines: %w", err)
	}
	if err := registry.RegisterMany(local...); err != nil {
		return nil, fmt.Errorf("project: register project pipelines: %w", err)
	}

	return registry, nil
}

// pipelinesDir returns root's conventional Pipeline-documents directory.
func pipelinesDir(root string) string {
	return filepath.Join(root, PipelinesDir)
}

// Scaffold writes root's pipelines directory with one starter Pipeline
// document per non-built-in slash command this build resolves by
// default ("feature", "bugfix", "release" — "review" already exists as
// a built-in and needs no starter). Each starter reproduces
// engine.DefaultPipeline's own generate → verify, one-bounded-repair
// shape under the slash command's name, ready for a project to edit —
// it is a starting point, not a policy Scaffold expects to be kept as-is.
//
// Scaffold is safe to re-run: it never overwrites a file that already
// exists, mirroring `git init`'s own re-run safety. A project's edits to
// an already-scaffolded document are never clobbered by running /init
// again.
func (ProjectLoader) Scaffold(root string) error {
	dir := pipelinesDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("project: scaffold %q: %w", dir, err)
	}

	for _, starter := range starterDocuments {
		path := filepath.Join(dir, starter.filename)
		if _, err := os.Stat(path); err == nil {
			continue // Already exists — never overwrite a project's own edits.
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("project: scaffold %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(starter.document), 0o644); err != nil {
			return fmt.Errorf("project: scaffold %q: %w", path, err)
		}
	}
	return nil
}

// starterDocument is one file Scaffold writes into a freshly initialized
// project's pipelines directory.
type starterDocument struct {
	filename string
	document string
}

// starterDocuments are the Pipeline documents Scaffold writes for a
// project that has never run /init before.
var starterDocuments = []starterDocument{
	{filename: "feature.json", document: starterPipelineDocument("feature")},
	{filename: "bugfix.json", document: starterPipelineDocument("bugfix")},
	{filename: "release.json", document: starterPipelineDocument("release")},
}

// starterPipelineDocument renders a minimal, valid PipelineDocument
// (engine/document.go) under name: generate → verify, one bounded
// repair — the same shape engine.DefaultPipeline already has, decoded
// the same way BuiltinPipelineSource's own embedded documents are.
func starterPipelineDocument(name string) string {
	return fmt.Sprintf(`{
  "name": %q,
  "steps": [
    { "id": "generate", "kind": "generate" },
    { "id": "verify", "kind": "verify" }
  ],
  "repair": {
    "max_attempts": 1
  }
}
`, name)
}
