package session

import (
	"context"
	"fmt"

	"foundry/project"
)

// InitCommand backs /init: it scaffolds the project's pipelines
// directory (project.ProjectLoader.Scaffold) so a project has starter
// Pipeline documents ready to edit — it runs no Pipeline itself. Unlike
// RunPipelineCommand, this is genuinely different work, not another
// instance of the same handler with a different PipelineName.
type InitCommand struct{}

var _ CommandHandler = InitCommand{}

// Run scaffolds s.Root's pipelines directory, then reloads s's registry
// so the newly scaffolded Pipelines (and anything else a user has
// already added to .foundry/pipelines) are immediately resolvable by
// /feature, /bug, /release, etc. — in this same session, with no
// restart. Re-running /init is safe: Scaffold never overwrites a file
// that already exists.
func (InitCommand) Run(ctx context.Context, s *Session, args string) error {
	if err := (project.ProjectLoader{}).Scaffold(s.Root); err != nil {
		return err
	}
	if err := s.ReloadPipelines(ctx); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "Initialized %s — edit its Pipeline documents to customize /feature, /bug, and /release.\n", project.PipelinesDir)
	return nil
}
