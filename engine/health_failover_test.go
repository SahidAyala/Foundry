package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
)

// TestEngine_Run_SkipsUnavailableModelPreemptively covers ADR-0013's tenth
// increment: connecting model.HealthManager to failover. A Preferred
// entry reported StatusUnavailable is never attempted at all when a
// healthier later entry exists — proven by the unhealthy executor
// receiving zero calls, not one failed call followed by a switch.
func TestEngine_Run_SkipsUnavailableModelPreemptively(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "health-failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnet := &captureExecutor{patches: []string{"sonnet-patch"}}
	gemini := &captureExecutor{patches: []string{"gemini-patch"}}
	def := &captureExecutor{patches: []string{"default-patch"}}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("claude-vendor", sonnet); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("gemini-vendor", gemini); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	health := model.NewHealthManager()
	if err := health.Report("claude-sonnet-5", model.Health{Status: model.StatusUnavailable, Reason: "rate limited"}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	models.SetHealthManager(health)

	router := engine.NewRouter(executors, def).WithModels(models)

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "gemini-patch" {
		t.Errorf("Patch = %q, want the healthy Gemini executor's patch", act.Patch)
	}
	if len(sonnet.calls) != 0 {
		t.Errorf("sonnet executor calls = %d, want 0 (Unavailable, must never be attempted while a healthy candidate exists)", len(sonnet.calls))
	}
	if len(gemini.calls) != 1 {
		t.Errorf("gemini executor calls = %d, want 1", len(gemini.calls))
	}
}

// TestEngine_Run_AllUnavailableStillAttemptsFirstCandidate covers the
// degrade-gracefully requirement: HealthManager reports are one
// Executor's own observation, which can go stale, so they are never a
// hard exclusion. If every Preferred entry is currently reported
// Unavailable, Foundry still attempts the Step's first declared
// candidate rather than refusing the Step outright.
func TestEngine_Run_AllUnavailableStillAttemptsFirstCandidate(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "health-failover-all-down",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnet := &captureExecutor{patches: []string{"sonnet-patch"}}
	gemini := &captureExecutor{patches: []string{"gemini-patch"}}
	def := &captureExecutor{patches: []string{"default-patch"}}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("claude-vendor", sonnet); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("gemini-vendor", gemini); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	health := model.NewHealthManager()
	if err := health.Report("claude-sonnet-5", model.Health{Status: model.StatusUnavailable}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	if err := health.Report("gemini-3.1-pro", model.Health{Status: model.StatusCooldown}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	models.SetHealthManager(health)

	router := engine.NewRouter(executors, def).WithModels(models)

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "sonnet-patch" {
		t.Errorf("Patch = %q, want the first declared candidate's patch even though every candidate is reported Unavailable", act.Patch)
	}
	if len(sonnet.calls) != 1 {
		t.Errorf("sonnet executor calls = %d, want 1 (still attempted despite being reported Unavailable, since every candidate is)", len(sonnet.calls))
	}
	if len(gemini.calls) != 0 {
		t.Errorf("gemini executor calls = %d, want 0", len(gemini.calls))
	}
}

// TestEngine_Run_ExpiredCooldownIsTriedFirst covers Health.Unavailable's
// own automatic-lift behavior: a model reported StatusCooldown with a
// RetryAt already in the past is treated as available again without a
// fresh Report call, so it is tried first, not deprioritized.
func TestEngine_Run_ExpiredCooldownIsTriedFirst(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "health-failover-expired-cooldown",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnet := &captureExecutor{patches: []string{"sonnet-patch"}}
	gemini := &captureExecutor{patches: []string{"gemini-patch"}}
	def := &captureExecutor{patches: []string{"default-patch"}}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("claude-vendor", sonnet); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("gemini-vendor", gemini); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	health := model.NewHealthManager()
	if err := health.Report("claude-sonnet-5", model.Health{Status: model.StatusCooldown, RetryAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	models.SetHealthManager(health)

	router := engine.NewRouter(executors, def).WithModels(models)

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "sonnet-patch" {
		t.Errorf("Patch = %q, want the first candidate's patch (its cooldown already expired)", act.Patch)
	}
	if len(gemini.calls) != 0 {
		t.Errorf("gemini executor calls = %d, want 0 (never needed — the first candidate's cooldown already lifted)", len(gemini.calls))
	}
}

// TestEngine_Run_CapabilityCheckPrefersHealthyOverUnhealthyCapableCandidate
// covers the same health-awareness applied alongside capability filtering
// (ADR-0013's seventh increment): two candidates both satisfy
// RequireCapabilities, but the first is reported Unavailable — the
// healthy second candidate is tried first, without ever attempting the
// unhealthy one.
func TestEngine_Run_CapabilityCheckPrefersHealthyOverUnhealthyCapableCandidate(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "health-capability",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}, RequireCapabilities: []string{"tool_use"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnet := &captureExecutor{patches: []string{"sonnet-patch"}}
	gemini := &captureExecutor{patches: []string{"gemini-patch"}}
	def := &captureExecutor{patches: []string{"default-patch"}}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("claude-vendor", sonnet); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("gemini-vendor", gemini); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor", Capabilities: model.Capabilities{ToolUse: true}}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gemini-3.1-pro", Executor: "gemini-vendor", Capabilities: model.Capabilities{ToolUse: true}}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	health := model.NewHealthManager()
	if err := health.Report("claude-sonnet-5", model.Health{Status: model.StatusUnavailable}); err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	models.SetHealthManager(health)

	router := engine.NewRouter(executors, def).WithModels(models)

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "gemini-patch" {
		t.Errorf("Patch = %q, want the healthy, capable Gemini executor's patch", act.Patch)
	}
	if len(sonnet.calls) != 0 {
		t.Errorf("sonnet executor calls = %d, want 0 (capable but Unavailable, must never be attempted while a healthy capable candidate exists)", len(sonnet.calls))
	}
}
