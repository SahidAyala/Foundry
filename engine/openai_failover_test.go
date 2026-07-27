package engine_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/executor/openai"
	"github.com/SahidAyala/Foundry/model"
)

// TestEngine_Run_FailsOverFromRealOpenAIRateLimitToAnotherModel proves the
// automatic failover mechanism (ADR-0013, Proposed, sixth increment) is
// no longer dormant against every real vendor: executor/openai's own
// error classification (added the same session, mapping its documented
// 401/403/429/404/5xx error taxonomy to model.FailureClass) lets a
// genuine 429 response from a real HTTP server trigger a real failover
// to a second model — using the real *openai.Executor, not a hand-rolled
// model.FailureError the way engine/failover_test.go's own tests do.
func TestEngine_Run_FailsOverFromRealOpenAIRateLimitToAnotherModel(t *testing.T) {
	rateLimited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "Rate limit reached"}}`))
	}))
	defer rateLimited.Close()

	pipeline := engine.Pipeline{
		Name: "real-openai-failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"gpt-primary", "gpt-fallback"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	primary := openai.NewExecutorWithEndpoint("gpt-primary", "test-key", rateLimited.URL)
	fallback := &captureExecutor{patches: []string{"fallback-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("primary-vendor", primary); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("fallback-vendor", fallback); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gpt-primary", Executor: "primary-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gpt-fallback", Executor: "fallback-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Patch != "fallback-patch" {
		t.Errorf("Patch = %q, want the fallback model's patch", act.Patch)
	}
	if len(fallback.calls) != 1 {
		t.Errorf("fallback executor calls = %d, want 1", len(fallback.calls))
	}
	if len(def.calls) != 0 {
		t.Errorf("default executor calls = %d, want 0", len(def.calls))
	}
}

// TestEngine_Run_DoesNotFailOverFromRealOpenAIAuthFailure proves the
// inverse: a real 401 from executor/openai is classified
// FailureAuthentication, which must never trigger failover, even with a
// second Preferred entry available.
func TestEngine_Run_DoesNotFailOverFromRealOpenAIAuthFailure(t *testing.T) {
	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Incorrect API key provided"}}`))
	}))
	defer unauthorized.Close()

	pipeline := engine.Pipeline{
		Name: "real-openai-no-failover",
		Steps: []engine.Step{
			{ID: "implement", Kind: domain.StepKindGenerate, Preferred: []string{"gpt-primary", "gpt-fallback"}},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	primary := openai.NewExecutorWithEndpoint("gpt-primary", "bad-key", unauthorized.URL)
	fallback := &captureExecutor{patches: []string{"fallback-patch"}}
	def := &captureExecutor{}

	executors := engine.NewExecutorRegistry()
	if err := executors.Register("primary-vendor", primary); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := executors.Register("fallback-vendor", fallback); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	models := model.NewRegistry()
	if err := models.Register(model.Info{ID: "gpt-primary", Executor: "primary-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := models.Register(model.Info{ID: "gpt-fallback", Executor: "fallback-vendor"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	router := engine.NewRouter(executors, def).WithModels(models)
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, def, &fakeVerifier{verdict: "pass"}, "", pipeline)
	eng.SetRouter(router)

	_, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err == nil {
		t.Fatal("Run succeeded despite a real authentication failure, want an error")
	}
	if len(fallback.calls) != 0 {
		t.Errorf("fallback executor calls = %d, want 0 (must never fail over for authentication)", len(fallback.calls))
	}
}
