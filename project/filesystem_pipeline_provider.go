// Package project discovers and authors the Pipeline documents a single
// project keeps for itself — everything about how Foundry relates to the
// project it is running inside, as distinct from engine (which knows
// nothing about the filesystem) and session (which knows nothing about
// Pipeline documents).
package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"foundry/engine"
)

// FilesystemPipelineProvider discovers Pipeline documents a project has
// authored for itself: every *.json file directly inside Dir (no
// recursion, no subdirectories), each decoded by the same
// engine.DecodePipelineDocument BuiltinProvider already uses for its own
// embedded documents. It satisfies engine.PipelineProvider — the seam
// RFC-0002/RFC-0003 designed for exactly this: a non-built-in source of
// Pipelines requiring no change to Engine, Strategy, or PipelineRegistry.
type FilesystemPipelineProvider struct {
	// Dir is the directory to read Pipeline documents from, conventionally
	// "<project root>/.foundry/pipelines".
	Dir string
}

var _ engine.PipelineProvider = FilesystemPipelineProvider{}

// Load decodes every *.json document directly inside Dir, in
// filename-sorted order so Load's result is deterministic across calls
// and across machines. A Dir that does not exist — a project that has
// never run /init — is not an error: it decodes to no Pipelines, exactly
// as an existing-but-empty directory would.
func (p FilesystemPipelineProvider) Load(ctx context.Context) ([]engine.Pipeline, error) {
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("project: read pipeline directory %q: %w", p.Dir, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	pipelines := make([]engine.Pipeline, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(p.Dir, name))
		if err != nil {
			return nil, fmt.Errorf("project: read pipeline document %q: %w", name, err)
		}
		pipeline, err := engine.DecodePipelineDocument(data)
		if err != nil {
			return nil, fmt.Errorf("project: %s: %w", name, err)
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}
