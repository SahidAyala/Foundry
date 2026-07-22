package cli

import (
	"bytes"
	"testing"

	"foundry/engine"
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
