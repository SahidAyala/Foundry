package engine_test

import (
	"context"
	"reflect"
	"testing"

	"foundry/engine"
)

func TestBuiltinPipelineSource_LoadsDefaultPipeline(t *testing.T) {
	var source engine.PipelineSource = engine.BuiltinPipelineSource{}

	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := engine.DefaultPipeline()
	found := false
	for _, p := range pipelines {
		if p.Name == want.Name {
			found = true
			if !reflect.DeepEqual(p, want) {
				t.Errorf("loaded %q Pipeline = %+v, want %+v", want.Name, p, want)
			}
		}
	}
	if !found {
		t.Errorf("Load() = %+v, want it to include a Pipeline named %q", pipelines, want.Name)
	}
}

// TestBuiltinPipelineSource_LoadIsIndependentPerCall verifies Load never hands
// back a Pipeline sharing mutable state with a Pipeline from a prior call —
// a caller mutating one loaded Pipeline's Steps must never affect a
// subsequent Load.
func TestBuiltinPipelineSource_LoadIsIndependentPerCall(t *testing.T) {
	source := engine.BuiltinPipelineSource{}

	first, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("first Load failed: %v", err)
	}
	first[0].Steps[0].Kind = "tampered"

	second, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if second[0].Steps[0].Kind == "tampered" {
		t.Error("mutating a Pipeline from one Load call affected a later Load call")
	}
}

// TestBuiltinPipelineSource_DoesNotMutateRegistryState verifies a PipelineSource
// has no way to reach into a PipelineRegistry: registering its output,
// then mutating the registry, must never be observable from a fresh Load —
// there is no shared state for a registry mutation to reach.
func TestBuiltinPipelineSource_DoesNotMutateRegistryState(t *testing.T) {
	source := engine.BuiltinPipelineSource{}
	registry := engine.NewPipelineRegistry()

	pipelines, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if err := registry.RegisterMany(pipelines...); err != nil {
		t.Fatalf("RegisterMany failed: %v", err)
	}

	// Mutate the registry's own copy as far as the registry's public API
	// allows: fetch it out and change the caller's copy.
	got, err := registry.Get("default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	got.Steps[0].Kind = "mutated-after-register"

	again, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if again[0].Steps[0].Kind == "mutated-after-register" {
		t.Error("mutating a Pipeline fetched from the registry affected the source's next Load")
	}
}
