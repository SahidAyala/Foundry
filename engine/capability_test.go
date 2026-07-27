package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
)

// TestEngine_Run_CapabilityAwareResolution_SkipsIncompatibleFirstCandidate
// covers ADR-0013's seventh increment (Proposed) — the maintainer's own
// example: a Step requires structured_output, tool_use, and thinking.
// The first Preferred entry lacks "thinking"; the second supports all
// three. The incompatible first entry must never even be attempted.
func TestEngine_Run_CapabilityAwareResolution_SkipsIncompatibleFirstCandidate(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "capability-aware",
		Steps: []engine.Step{
			{
				ID: "implement", Kind: domain.StepKindGenerate,
				Preferred:           []string{"claude-haiku-4-5", "claude-opus-4-8"},
				RequireCapabilities: []string{"structured_output", "tool_use", "thinking"},
			},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	haiku := &captureExecutor{patches: []string{"haiku-patch"}}
	opus := &captureExecutor{patches: []string{"opus-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("haiku-vendor", haiku); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("opus-vendor", opus); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{
		ID: "claude-haiku-4-5", Executor: "haiku-vendor",
		Capabilities: model.Capabilities{StructuredOutput: true, ToolUse: true, Thinking: false},
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{
		ID: "claude-opus-4-8", Executor: "opus-vendor",
		Capabilities: model.Capabilities{StructuredOutput: true, ToolUse: true, Thinking: true},
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "opus-patch" {
		t.Errorf("Patch = %q, want the capable model's patch", act.Patch)
	}
	if len(haiku.calls) != 0 {
		t.Errorf("haiku (incompatible) executor calls = %d, want 0 — it must never be attempted", len(haiku.calls))
	}
	if len(opus.calls) != 1 {
		t.Errorf("opus (compatible) executor calls = %d, want 1", len(opus.calls))
	}
}

// TestEngine_Run_CapabilityAwareResolution_FailsBeforeExecutionWhenNoneQualify
// covers the explicit requirement: "if none exist, return a validation
// error before execution." Neither candidate supports every required
// capability — the Step must fail without ever calling Execute on either.
func TestEngine_Run_CapabilityAwareResolution_FailsBeforeExecutionWhenNoneQualify(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "capability-aware",
		Steps: []engine.Step{
			{
				ID: "implement", Kind: domain.StepKindGenerate,
				Preferred:           []string{"claude-haiku-4-5", "gemini-3.5-flash-lite"},
				RequireCapabilities: []string{"structured_output", "tool_use", "thinking"},
			},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	haiku := &captureExecutor{patches: []string{"haiku-patch"}}
	flashLite := &captureExecutor{patches: []string{"flash-lite-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("haiku-vendor", haiku); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("gemini-vendor", flashLite); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{
		ID: "claude-haiku-4-5", Executor: "haiku-vendor",
		Capabilities: model.Capabilities{StructuredOutput: true, ToolUse: true, Thinking: false},
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{
		ID: "gemini-3.5-flash-lite", Executor: "gemini-vendor",
		Capabilities: model.Capabilities{StructuredOutput: true, ToolUse: true, Thinking: false},
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run succeeded despite no candidate supporting every required capability, want an error")
	}
	if len(haiku.calls) != 0 {
		t.Errorf("haiku executor calls = %d, want 0 (validation must fail before any Execute)", len(haiku.calls))
	}
	if len(flashLite.calls) != 0 {
		t.Errorf("gemini flash-lite executor calls = %d, want 0 (validation must fail before any Execute)", len(flashLite.calls))
	}
	if len(def.calls) != 0 {
		t.Errorf("default executor calls = %d, want 0", len(def.calls))
	}
}

// TestEngine_Run_CapabilityAwareResolution_FailoverSkipsIncapableCandidates
// combines capability filtering with automatic failover: three
// candidates in order — capable-but-fails-retryably, incapable, capable-
// and-succeeds. Failover must move straight from the first to the third,
// since the incapable middle entry was excluded before execution began.
func TestEngine_Run_CapabilityAwareResolution_FailoverSkipsIncapableCandidates(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "capability-aware-failover",
		Steps: []engine.Step{
			{
				ID: "implement", Kind: domain.StepKindGenerate,
				Preferred:           []string{"model-a", "model-b", "model-c"},
				RequireCapabilities: []string{"tool_use"},
			},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	failure := &model.FailureError{Class: model.FailureRateLimit, Err: errors.New("429")}
	a := &captureExecutor{errs: []error{failure}, patches: []string{""}}
	b := &captureExecutor{patches: []string{"b-patch"}}
	c := &captureExecutor{patches: []string{"c-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	for name, exec := range map[string]*captureExecutor{"a-vendor": a, "b-vendor": b, "c-vendor": c} {
		if err := executors.Register(name, exec); err != nil {
			t.Fatalf("Register(%q) failed: %v", name, err)
		}
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "model-a", Executor: "a-vendor", Capabilities: model.Capabilities{ToolUse: true}}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "model-b", Executor: "b-vendor", Capabilities: model.Capabilities{ToolUse: false}}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "model-c", Executor: "c-vendor", Capabilities: model.Capabilities{ToolUse: true}}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "c-patch" {
		t.Errorf("Patch = %q, want model-c's patch", act.Patch)
	}
	if len(a.calls) != 1 {
		t.Errorf("model-a calls = %d, want 1", len(a.calls))
	}
	if len(b.calls) != 0 {
		t.Errorf("model-b (incapable) calls = %d, want 0 — it must never be attempted, even as a failover target", len(b.calls))
	}
	if len(c.calls) != 1 {
		t.Errorf("model-c calls = %d, want 1", len(c.calls))
	}
}

// TestEngine_Run_RequireCapabilitiesHasNoEffectWithoutModelOrPreferred
// covers a deliberate scope boundary: Capabilities live on model.Info,
// keyed by Model ID — a Step naming only Executor has nothing to check
// them against, so RequireCapabilities is inert there.
func TestEngine_Run_RequireCapabilitiesHasNoEffectWithoutModelOrPreferred(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "executor-only",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Executor: "pinned", RequireCapabilities: []string{"tool_use"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	pinned := &captureExecutor{patches: []string{"pinned-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("pinned", pinned); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(engine.NewRouter(executors, def))

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "pinned-patch" {
		t.Errorf("Patch = %q, want the pinned executor's patch", act.Patch)
	}
}
