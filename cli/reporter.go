package cli

import (
	"io"
	"log/slog"
	"os"

	"foundry/engine"
)

// foundryLogEnv is the opt-in switch for structured observability logging
// (roadmap.md M5's named, previously-unstarted "observability" gap),
// mirroring how every other optional behavior in this codebase — a second
// Executor (.foundry/executors.json), remote publish
// (.foundry/config.json) — defaults to off and is enabled by adding
// something, never by removing something. Unset or empty: behavior is
// byte-for-byte what NewProgressReporter alone already produced, so no
// existing user or test sees any change.
const foundryLogEnv = "FOUNDRY_LOG"

// NewReporter returns the engine.Reporter a composition root should attach
// for one Engine run: always a human-facing ProgressReporter writing to
// out, and — only when FOUNDRY_LOG is set in the environment — also a
// structured engine.SlogReporter emitting JSON lines to stderr, fanned out
// via engine.MultiReporter. This is the one place that decision is made, so
// cmd/foundry/commands/do.go and session/run_pipeline_command.go both stay
// in sync without duplicating the environment check.
func NewReporter(out io.Writer) engine.Reporter {
	progress := NewProgressReporter(out)
	if os.Getenv(foundryLogEnv) == "" {
		return progress
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	return engine.MultiReporter{Reporters: []engine.Reporter{progress, engine.NewSlogReporter(logger)}}
}
