package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
)

func TestNewReporter_DefaultIsProgressReporterOnly(t *testing.T) {
	t.Setenv(foundryLogEnv, "")

	var out bytes.Buffer
	r := NewReporter(&out)

	if _, ok := r.(*ProgressReporter); !ok {
		t.Errorf("NewReporter() = %T, want *ProgressReporter when %s is unset", r, foundryLogEnv)
	}
}

func TestNewReporter_FoundryLogEnvAddsStructuredReporter(t *testing.T) {
	t.Setenv(foundryLogEnv, "1")

	var out bytes.Buffer
	r := NewReporter(&out)

	multi, ok := r.(engine.MultiReporter)
	if !ok {
		t.Fatalf("NewReporter() = %T, want engine.MultiReporter when %s is set", r, foundryLogEnv)
	}
	if len(multi.Reporters) != 2 {
		t.Fatalf("MultiReporter has %d Reporters, want 2", len(multi.Reporters))
	}
	if _, ok := multi.Reporters[0].(*ProgressReporter); !ok {
		t.Errorf("first Reporter = %T, want *ProgressReporter", multi.Reporters[0])
	}
	if _, ok := multi.Reporters[1].(*engine.SlogReporter); !ok {
		t.Errorf("second Reporter = %T, want *engine.SlogReporter", multi.Reporters[1])
	}
}

// tracedExecutor is an Executor that can stream, so a test can see whether
// TraceExecutor actually installed a sink on it.
type tracedExecutor struct {
	sink func(string)
}

func (e *tracedExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return &domain.Outcome{}, nil
}

func (e *tracedExecutor) SetProgress(sink func(string)) { e.sink = sink }

// plainExecutor cannot stream — the majority of Executors (every HTTP-API
// one), which must be left untouched rather than rejected.
type plainExecutor struct{}

func (plainExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return &domain.Outcome{}, nil
}

func TestTraceExecutor_OffByDefault(t *testing.T) {
	t.Setenv(foundryVerboseEnv, "")

	var out bytes.Buffer
	e := &tracedExecutor{}
	TraceExecutor(e, NewReporter(&out))

	if e.sink != nil {
		t.Errorf("a sink was installed with %s unset: streaming changes how the Executor is invoked, so it must be opt-in", foundryVerboseEnv)
	}
}

func TestTraceExecutor_WiresExecutorOutputIntoTheNarration(t *testing.T) {
	t.Setenv(foundryVerboseEnv, "1")
	t.Setenv(foundryLogEnv, "")

	var out bytes.Buffer
	e := &tracedExecutor{}
	TraceExecutor(e, NewReporter(&out))

	if e.sink == nil {
		t.Fatalf("no sink installed with %s set", foundryVerboseEnv)
	}
	e.sink("tool Read main.go")
	if got := out.String(); !strings.Contains(got, "tool Read main.go") {
		t.Errorf("output = %q, want the Executor's own line rendered", got)
	}
}

// TestTraceExecutor_ReachesThroughAMultiReporter guards the composition
// FOUNDRY_LOG builds: a MultiReporter satisfies only engine.Reporter, so
// without unwrapping it the caller who asked for the most observability
// would get the least.
func TestTraceExecutor_ReachesThroughAMultiReporter(t *testing.T) {
	t.Setenv(foundryVerboseEnv, "1")
	t.Setenv(foundryLogEnv, "1")

	var out bytes.Buffer
	e := &tracedExecutor{}
	TraceExecutor(e, NewReporter(&out))

	if e.sink == nil {
		t.Fatal("no sink installed when the Reporter is a MultiReporter")
	}
	e.sink("tool Read main.go")
	if got := out.String(); !strings.Contains(got, "tool Read main.go") {
		t.Errorf("output = %q, want the Executor's own line rendered", got)
	}
}

func TestTraceExecutor_LeavesANonStreamingExecutorAlone(t *testing.T) {
	t.Setenv(foundryVerboseEnv, "1")

	var out bytes.Buffer
	// Must not panic or fail: an Executor that cannot stream is simply
	// never traced.
	TraceExecutor(plainExecutor{}, NewReporter(&out))
}

// TestTraceRelay_RoutesToTheCurrentReporterAndThenNowhere covers the
// interactive session's shape: Executors built once at startup, a Reporter
// per Act. Output after an Act ends must go nowhere rather than print over
// the next prompt.
func TestTraceRelay_RoutesToTheCurrentReporterAndThenNowhere(t *testing.T) {
	t.Setenv(foundryVerboseEnv, "1")
	t.Setenv(foundryLogEnv, "")

	relay := NewTraceRelay()
	e := &tracedExecutor{}
	TraceExecutorTo(e, relay.Line)
	if e.sink == nil {
		t.Fatal("no sink installed on the relay")
	}

	// Before any Act: discarded, not panicking.
	e.sink("orphan line")

	var first bytes.Buffer
	relay.To(NewReporter(&first))
	e.sink("during the act")
	if got := first.String(); !strings.Contains(got, "during the act") {
		t.Errorf("output = %q, want the line routed to the running Act's Reporter", got)
	}

	relay.To(nil)
	e.sink("after the act")
	if strings.Contains(first.String(), "after the act") {
		t.Errorf("output = %q, want nothing written once the Act is done", first.String())
	}
}
