package cli

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/SahidAyala/Foundry/engine"
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

// foundryVerboseEnv opts into streaming an Executor's own live output into
// the progress narration — for a CLI-backed Executor, the tools it is
// running and the turns it is taking while an Act waits on it. Off by
// default, and additive when on, the same shape as FOUNDRY_LOG above: it
// changes how the Executor is invoked (see executor/claude's stream.go), so
// a caller opts in deliberately rather than inheriting a different
// invocation for every run.
const foundryVerboseEnv = "FOUNDRY_VERBOSE"

// ExecutorTracer is the optional seam an Executor may implement to stream
// its own progress somewhere while it runs. executor/claude implements it;
// an Executor that doesn't is simply never traced.
type ExecutorTracer interface {
	SetProgress(sink func(string))
}

// TraceExecutor wires e's live output into r's narration, if FOUNDRY_VERBOSE
// is set, e can stream at all, and r can render a line. Called by a
// composition root once per Executor it builds — every condition failing is
// an ordinary no-op, never an error: this is additional visibility, and an
// Act must not fail because it could not be narrated.
func TraceExecutor(e engine.Executor, r engine.Reporter) {
	TraceExecutorTo(e, executorSink(r))
}

// TraceExecutorTo is TraceExecutor against an explicit sink, for a caller
// whose Executors outlive any single Reporter — see TraceRelay. A nil sink
// traces nothing.
func TraceExecutorTo(e engine.Executor, sink func(string)) {
	if sink == nil || os.Getenv(foundryVerboseEnv) == "" {
		return
	}
	if tracer, ok := e.(ExecutorTracer); ok {
		tracer.SetProgress(sink)
	}
}

// TraceRelay forwards an Executor's live output to whichever Reporter is
// narrating right now. It exists because the interactive session builds its
// Executors once, at startup, but narrates one Act at a time through a
// Reporter created per slash command: without an indirection the
// long-lived Executor would be permanently bound to the first Act's
// Reporter, printing later Acts' output into a Reporter that is done. A
// relay with no destination set discards, so an Executor that outlives an
// Act narrates nowhere rather than into a closed Reporter.
type TraceRelay struct {
	mu   sync.Mutex
	sink func(string)
}

// NewTraceRelay returns a relay with no destination yet.
func NewTraceRelay() *TraceRelay {
	return &TraceRelay{}
}

// To points the relay at r, or at nothing when r is nil (what a caller
// defers once an Act is done). Safe to call while a call is in flight.
func (r *TraceRelay) To(reporter engine.Reporter) {
	var sink func(string)
	if reporter != nil {
		sink = executorSink(reporter)
	}
	r.mu.Lock()
	r.sink = sink
	r.mu.Unlock()
}

// Line is the sink to hand TraceExecutorTo. It forwards to the current
// destination, if there is one.
func (r *TraceRelay) Line(s string) {
	r.mu.Lock()
	sink := r.sink
	r.mu.Unlock()
	if sink != nil {
		sink(s)
	}
}

// executorLineWriter is implemented by a Reporter that can render one line
// of an Executor's own output. It is defined here rather than in engine
// because the Engine never emits these lines — they come from inside an
// Executor, below the Engine entirely, and reach the terminal only because
// a composition root wired the two together.
type executorLineWriter interface {
	ExecutorLine(string)
}

// executorSink returns the function an Executor should write its live
// output to, or nil if r renders none. A MultiReporter is unwrapped rather
// than asserted on: it satisfies only engine.Reporter, so the composition
// NewReporter builds under FOUNDRY_LOG would otherwise be unable to
// narrate anything — precisely the case where a caller asked for more
// visibility.
func executorSink(r engine.Reporter) func(string) {
	switch v := r.(type) {
	case executorLineWriter:
		return v.ExecutorLine
	case engine.MultiReporter:
		for _, child := range v.Reporters {
			if sink := executorSink(child); sink != nil {
				return sink
			}
		}
	}
	return nil
}

// NewReporter returns the engine.Reporter a composition root should attach
// for one Engine run: always a human-facing ProgressReporter writing to
// out, and — only when FOUNDRY_LOG is set in the environment — also a
// structured engine.SlogReporter emitting JSON lines to stderr, fanned out
// via engine.MultiReporter. This is the one place that decision is made, so
// cmd/foundry/commands/do.go and session/run_pipeline_command.go both stay
// in sync without duplicating the environment check.
// The returned Reporter may hold a goroutine open for the duration of one
// Act (ProgressReporter's elapsed-time heartbeat). A caller whose process
// outlives the Act — the interactive session, which runs one Act per slash
// command — must pass it to CloseReporter when the Act is done; a one-shot
// `foundry do` process may simply exit.
func NewReporter(out io.Writer) engine.Reporter {
	progress := NewProgressReporter(out)
	if os.Getenv(foundryLogEnv) == "" {
		return progress
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	return engine.MultiReporter{Reporters: []engine.Reporter{progress, engine.NewSlogReporter(logger)}}
}

// CloseReporter releases whatever r holds open, if r holds anything open at
// all — both concrete Reporters NewReporter can return implement io.Closer,
// but a Reporter is not required to, so a non-Closer is simply left alone.
func CloseReporter(r engine.Reporter) error {
	if c, ok := r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
