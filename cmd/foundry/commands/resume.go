package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"foundry/cli"
	"foundry/engine"
	"foundry/project"
	"foundry/record"
)

func resumeUsage() string {
	return `Usage: foundry resume [<act-id>] --repo <path>

With an act ID, continues an Act interrupted mid-Pipeline — by a crash or
kill before it reached a terminal Judgment — from its last completed Step,
without starting a new Act.

Without an act ID, lists every interrupted Act for the repository that a
checkpoint survives for.

Resume does not cross a repair boundary or a Pipeline-definition change
since the interrupted attempt started
(docs/06-open-questions/OQ-008-in-progress-act-persistence.md).

Flags:
  --repo <path>   path to the repository to operate on (required)
  -h, --help      show this help message
`
}

// Resume implements the `foundry resume [<act-id>]` command: with an act
// ID, continue an interrupted Act; without one, list every Act a
// checkpoint survives for.
//
// newNamedExecutor is the same vendor-dispatch seam Do accepts (see its own
// doc comment) — passed through to wireEngine so a resumed Act's Router is
// wired identically to a fresh one's.
func Resume(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor) int {
	actID, repoPath, err := parseResumeArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, resumeUsage())
			return 0
		}
		fmt.Fprintln(stdout, err)
		fmt.Fprint(stdout, resumeUsage())
		return 2
	}

	checkpoints, ok, err := checkpointStore(repoPath, stdout)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if !ok {
		fmt.Fprintln(stdout, "No interrupted acts.")
		return 0
	}

	if actID == "" {
		return listInterrupted(ctx, checkpoints, stdout)
	}

	checkpointed, err := checkpoints.Load(ctx, actID)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	eng, store, _, err := wireEngine(ctx, repoPath, stdin, stdout, newExecutor, newNamedExecutor, checkpointed.Pipeline)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	c := cli.NewCLI(eng, store, stdin, stdout)

	if err := c.Resume(ctx, actID, checkpoints, repoPath); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// listInterrupted writes every Act a checkpoint survives for, newest first:
// ID, creation time, intent, and how many Steps it completed before it was
// interrupted.
func listInterrupted(ctx context.Context, checkpoints *record.CheckpointStore, stdout io.Writer) int {
	acts, err := checkpoints.List(ctx)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if len(acts) == 0 {
		fmt.Fprintln(stdout, "No interrupted acts.")
		return 0
	}

	for i := len(acts) - 1; i >= 0; i-- {
		act := acts[i]
		fmt.Fprintf(stdout, "%s  %s  %d step(s) completed  %s\n",
			act.ID, act.CreatedAt.Format(time.RFC3339), len(act.Steps), act.Intent)
	}
	return 0
}

// parseResumeArgs parses `foundry resume` arguments: an optional positional
// Act ID and a required --repo flag, in either order.
func parseResumeArgs(args []string) (actID string, repoPath string, err error) {
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help" || arg == "-help":
			return "", "", cli.ErrHelp
		case arg == "--repo" || arg == "-repo":
			i++
			if i >= len(args) {
				return "", "", errors.New("cli: --repo requires a value")
			}
			repoPath = args[i]
		default:
			positional = append(positional, arg)
		}
	}

	if repoPath == "" {
		return "", "", errors.New("cli: --repo is required")
	}
	if len(positional) > 1 {
		return "", "", errors.New("cli: at most one act-id argument is allowed")
	}
	if len(positional) == 1 {
		actID = positional[0]
	}
	return actID, repoPath, nil
}

// checkpointStore opens the repository's CheckpointStore for read-only
// listing or lookup. ok is false when the repository has no Record
// directory yet — resuming or listing must not create one.
func checkpointStore(repoPath string, stdout io.Writer) (store *record.CheckpointStore, ok bool, err error) {
	actsDir := filepath.Join(repoPath, ".foundry", "acts")
	if _, statErr := os.Stat(actsDir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cli: read checkpoints: %w", statErr)
	}

	checkpoints, err := record.NewCheckpointStore(actsDir)
	if err != nil {
		return nil, false, err
	}
	return checkpoints, true, nil
}
