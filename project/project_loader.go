package project

import (
	"context"
	"fmt"
	"path/filepath"

	"foundry/engine"
)

// PipelinesDir is the conventional location, relative to a project root,
// where FilesystemPipelineProvider reads and ProjectLoader.Scaffold
// writes project-local Pipeline documents.
const PipelinesDir = ".foundry/pipelines"

// ProjectLoader resolves a project's full set of Pipelines — every
// built-in Pipeline this build of Foundry ships, plus every Pipeline the
// project has authored for itself — and scaffolds a fresh project's
// pipelines directory for /init. It owns no Engine, Strategy, or
// PipelineRegistry logic of its own; it only composes what
// engine.BuiltinProvider, FilesystemPipelineProvider, and
// engine.PipelineRegistry already do.
type ProjectLoader struct{}

// LoadRegistry returns a PipelineRegistry populated first with every
// built-in Pipeline (engine.BuiltinProvider), then every Pipeline
// authored under root's pipelines directory (FilesystemPipelineProvider)
// — the same RegisterMany composition engine.NewDefaultRegistry already
// uses for built-ins alone, generalized by one more provider. A
// project-local Pipeline whose name collides with a built-in surfaces
// PipelineRegistry.RegisterMany's existing duplicate-name error; a
// collision is never silently resolved in either direction.
func (ProjectLoader) LoadRegistry(ctx context.Context, root string) (*engine.PipelineRegistry, error) {
	registry := engine.NewPipelineRegistry()

	builtins, err := engine.BuiltinProvider{}.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: load built-in pipelines: %w", err)
	}
	if err := registry.RegisterMany(builtins...); err != nil {
		return nil, fmt.Errorf("project: register built-in pipelines: %w", err)
	}

	local, err := FilesystemPipelineProvider{Dir: pipelinesDir(root)}.Load(ctx)
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
