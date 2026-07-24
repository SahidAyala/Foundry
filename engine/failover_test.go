package engine_test

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/model"
)

// TestEngine_Run_FailsOverToNextPreferredModelOnRetryableFailure covers
// ADR-0013's sixth increment (Proposed): the maintainer's own worked
// example — Claude Sonnet unavailable, switching to Gemini 3.1 Pro. The
// first Preferred entry's Executor fails with a retryable FailureClass;
// the second entry's Executor is tried instead and its patch is used.
func TestEngine_Run_FailsOverToNextPreferredModelOnRetryableFailure(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnetFailure := &model.FailureError{Class: model.FailureRateLimit, Err: errors.New("429 too many requests")}
	sonnet := &captureExecutor{errs: []error{sonnetFailure}, patches: []string{""}}
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

	router := engine.NewRouter(executors, def).WithModels(models)

	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)
	reporter := &fakeReporter{}
	eng.SetReporter(reporter)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.Patch != "gemini-patch" {
		t.Errorf("Patch = %q, want the failed-over Gemini executor's patch", act.Patch)
	}
	if len(sonnet.calls) != 1 {
		t.Errorf("sonnet executor calls = %d, want 1", len(sonnet.calls))
	}
	if len(gemini.calls) != 1 {
		t.Errorf("gemini executor calls = %d, want 1", len(gemini.calls))
	}
	if len(def.calls) != 0 {
		t.Errorf("default executor calls = %d, want 0 (Preferred names both models explicitly)", len(def.calls))
	}

	found := false
	for _, e := range reporter.events {
		if e == "failover:implement:claude-sonnet-5->gemini-3.1-pro:rate_limit" {
			found = true
		}
	}
	if !found {
		t.Errorf("events = %v, want a failover event naming the switch", reporter.events)
	}
}

// TestEngine_Run_DoesNotFailOverOnNonRetryableFailure covers the explicit
// requirement: authentication, invalid model, and unsupported capability
// failures must never trigger failover, even with more Preferred entries
// available.
func TestEngine_Run_DoesNotFailOverOnNonRetryableFailure(t *testing.T) {
	for _, class := range []model.FailureClass{
		model.FailureAuthentication,
		model.FailureInvalidModel,
		model.FailureUnsupportedCapability,
	} {
		t.Run(string(class), func(t *testing.T) {
			pipeline := engine.Pipeline{
				Name: "failover",
				Steps: []engine.Step{
					{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
					{ID: "verify", Kind: domain.StepKindVerify},
				},
			}

			failure := &model.FailureError{Class: class, Err: errors.New("failure")}
			sonnet := &captureExecutor{errs: []error{failure}, patches: []string{""}}
			gemini := &captureExecutor{patches: []string{"gemini-patch"}}
			def := &captureExecutor{}

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

			router := engine.NewRouter(executors, def).WithModels(models)
			eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
			eng.SetRouter(router)

			_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
			if err == nil {
				t.Fatal("Run succeeded despite a non-retryable failure, want an error")
			}
			if len(gemini.calls) != 0 {
				t.Errorf("gemini executor calls = %d, want 0 (must never fail over for %s)", len(gemini.calls), class)
			}
		})
	}
}

// TestEngine_Run_FailoverExhaustsAllPreferredEntries covers the case
// where every Preferred entry fails retryably — the Step still fails
// overall, since there is nothing further to try.
func TestEngine_Run_FailoverExhaustsAllPreferredEntries(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"claude-sonnet-5", "gemini-3.1-pro"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	sonnetFailure := &model.FailureError{Class: model.FailureRateLimit, Err: errors.New("429")}
	geminiFailure := &model.FailureError{Class: model.FailureTemporaryUnavailable, Err: errors.New("503")}
	sonnet := &captureExecutor{errs: []error{sonnetFailure}, patches: []string{""}}
	gemini := &captureExecutor{errs: []error{geminiFailure}, patches: []string{""}}
	def := &captureExecutor{}

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

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run succeeded despite every Preferred entry failing, want an error")
	}
	if len(sonnet.calls) != 1 || len(gemini.calls) != 1 {
		t.Errorf("calls: sonnet=%d gemini=%d, want exactly 1 each", len(sonnet.calls), len(gemini.calls))
	}
}

// TestEngine_Run_NoFailoverWithoutPreferredConfigured covers the explicit
// requirement: "supported only when preferred[] is configured" — a Step
// naming only Model (no Preferred) never fails over, even for an
// otherwise-retryable failure class, since there is nothing else to try.
func TestEngine_Run_NoFailoverWithoutPreferredConfigured(t *testing.T) {
	pipeline := engine.Pipeline{
		Name: "no-failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Model: "claude-sonnet-5"},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	failure := &model.FailureError{Class: model.FailureRateLimit, Err: errors.New("429")}
	sonnet := &captureExecutor{errs: []error{failure}, patches: []string{""}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("claude-vendor", sonnet); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "claude-sonnet-5", Executor: "claude-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run succeeded despite a retryable failure with no Preferred list configured, want an error")
	}
	if len(def.calls) != 0 {
		t.Errorf("default executor calls = %d, want 0", len(def.calls))
	}
}
