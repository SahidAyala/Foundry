package project_test

import (
	"context"
	"testing"

	"foundry/domain"
	"foundry/project"
)

// TestDogfoodPipelines_DecodeAndUseTrustSteps loads this repository's own
// .foundry/pipelines — the "well-generated pipeline examples" the repo
// ships for anyone reading it, not test fixtures — through the same
// FilesystemPipelineProvider a real project's Session uses, so a future
// edit that breaks their JSON or reverts them to the trivial
// generate-then-verify starter shape fails here, not silently.
func TestDogfoodPipelines_DecodeAndUseTrustSteps(t *testing.T) {
	provider := project.FilesystemPipelineProvider{Dir: "../.foundry/pipelines"}

	pipelines, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	byName := make(map[string]int) // name -> number of trust-boundary Steps
	for _, p := range pipelines {
		for _, step := range p.Steps {
			switch step.Kind {
			case domain.StepKindApprove, domain.StepKindApply, domain.StepKindRecord:
				byName[p.Name]++
			}
		}
	}

	for _, name := range []string{"feature", "bugfix", "release"} {
		if byName[name] == 0 {
			t.Errorf("pipeline %q declares no approve/apply/record Step — want it to demonstrate the full RFC-0002 §9 Phase 4 lifecycle, not the trivial generate-then-verify starter shape", name)
		}
	}
}
