package session

import (
	"context"
	"fmt"
	"strings"

	"foundry/cli"
	"foundry/workspace"
)

// RunPipelineCommand is the one CommandHandler backing every slash
// command that runs a Pipeline by name — "/feature", "/bug", "/review",
// "/release", and any project-defined command shaped the same way. It
// contains no per-command logic at all, only which Pipeline it
// resolves: this is the literal form of "la sesión interactiva actúa
// únicamente como una capa superior que traduce slash commands hacia la
// ejecución de Pipelines" — the translation is data (PipelineName), not
// a bespoke handler per command.
type RunPipelineCommand struct {
	// PipelineName is the Pipeline this command resolves from the
	// Session's registry — by convention, the slash command's own name
	// ("feature" backs /feature), though nothing requires that.
	PipelineName string
}

var _ CommandHandler = RunPipelineCommand{}

// Describe returns this command's one-line /help description, naming the
// Pipeline it runs.
func (cmd RunPipelineCommand) Describe() string {
	return fmt.Sprintf("Run the %q Pipeline over a description of the work.", cmd.PipelineName)
}

// Run resolves cmd.PipelineName from s, then delegates the entire
// approve/apply/record lifecycle to the unchanged cli.CLI.Do — the same
// call cmd/foundry/commands/do.go already makes once per process, made
// here once per slash command instead. args becomes the Act's Intent
// text verbatim.
func (cmd RunPipelineCommand) Run(ctx context.Context, s *Session, args string) error {
	if strings.TrimSpace(args) == "" {
		return fmt.Errorf("session: /%s requires a description of the work, e.g. /%s \"implement X\"", cmd.PipelineName, cmd.PipelineName)
	}

	eng, err := s.Engine(cmd.PipelineName)
	if err != nil {
		return err
	}
	eng.SetReporter(cli.NewReporter(s.Out))
	eng.SetAuthority(cli.InteractiveAuthority{In: s.In, Out: s.Out})
	eng.SetApplier(workspace.GitApplier{})
	eng.SetCheckpointer(s.Recorder())
	eng.SetCheckpointSaver(s.Checkpoints())

	c := cli.NewCLI(eng, s.Recorder(), s.In, s.Out)
	return c.Do(ctx, args, s.Root)
}
