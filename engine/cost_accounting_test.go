package engine_test

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// pricedExecutor is an Executor that also implements engine.CostEstimator,
// reporting a fixed per-call cost — the shape of executor/openai.Executor
// (ADR-0005 Decision 3), used here to prove per-Step Budget accounting
// (RFC-0004 §2.7) sums each Step's own estimate rather than charging one
// flat rate for the whole attempt.
type pricedExecutor struct {
	patch string
	cost  float64
	calls int
}

func (e *pricedExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	e.calls++
	return &domain.Outcome{Patch: e.patch}, nil
}

func (e *pricedExecutor) EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error) {
	return e.cost, nil
}

var _ engine.Executor = (*pricedExecutor)(nil)
var _ engine.CostEstimator = (*pricedExecutor)(nil)

// TestEngine_PerStepBudget_SumsHeterogeneousExecutorCosts pins RFC-0004
// §2.7's fix: a Pipeline with two Generate Steps pinned to two different
// Executors, each reporting its own cost via CostEstimator, is charged the
// sum of both — not one flat executeCostEstimateUSD charge for the whole
// attempt, which is what a seven-role Pipeline mixing vendors would have
// silently undercounted before this change.
func TestEngine_PerStepBudget_SumsHeterogeneousExecutorCosts(t *testing.T) {
	cheap := &pricedExecutor{patch: "patch-cheap", cost: 0.10}
	pricey := &pricedExecutor{patch: "patch-pricey", cost: 1.40}

	registry := engine.NewExecutorRegistry()
	if err := registry.Register("cheap", cheap); err != nil {
		t.Fatalf("Register(cheap) failed: %v", err)
	}
	if err := registry.Register("pricey", pricey); err != nil {
		t.Fatalf("Register(pricey) failed: %v", err)
	}

	pipeline := engine.Pipeline{
		Name: "priced",
		Steps: []engine.Step{
			{ID: "plan", Kind: domain.StepKindGenerate, Executor: "cheap"},
			{ID: "implement", Kind: domain.StepKindGenerate, Executor: "pricey"},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	eng := engine.NewEngine(gatherer, cheap, verifier, "", pipeline)
	eng.SetRouter(engine.NewRouter(registry, cheap))

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "do work"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if act.CostEstimateUSD != 1.50 {
		t.Errorf("CostEstimateUSD = %v, want 1.50 (0.10 + 1.40, summed per Step)", act.CostEstimateUSD)
	}
	if act.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2 (one charge per Generate Step)", act.Iterations)
	}
	if cheap.calls != 1 || pricey.calls != 1 {
		t.Errorf("calls: cheap=%d pricey=%d, want 1 each", cheap.calls, pricey.calls)
	}
}

// TestEngine_PerStepBudget_RefusesMidAttemptWhenSecondStepExceedsCost pins
// the new failure mode per-Step charging introduces: a Budget that easily
// affords the first Generate Step can still be exhausted by the second one
// within the very same (first) attempt, not only between attempts. This was
// impossible before RFC-0004 §2.7 — the old flat per-attempt charge only
// ever checked the ceiling once, before any Step ran.
func TestEngine_PerStepBudget_RefusesMidAttemptWhenSecondStepExceedsCost(t *testing.T) {
	cheap := &pricedExecutor{patch: "patch-cheap", cost: 0.10}
	pricey := &pricedExecutor{patch: "patch-pricey", cost: 1.40}

	registry := engine.NewExecutorRegistry()
	if err := registry.Register("cheap", cheap); err != nil {
		t.Fatalf("Register(cheap) failed: %v", err)
	}
	if err := registry.Register("pricey", pricey); err != nil {
		t.Fatalf("Register(pricey) failed: %v", err)
	}

	pipeline := engine.Pipeline{
		Name: "priced",
		Steps: []engine.Step{
			{ID: "plan", Kind: domain.StepKindGenerate, Executor: "cheap"},
			{ID: "implement", Kind: domain.StepKindGenerate, Executor: "pricey"},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	eng := engine.NewEngine(gatherer, cheap, verifier, "", pipeline)
	eng.SetRouter(engine.NewRouter(registry, cheap))

	_, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "do work"},
		&domain.Budget{MaxIterations: 10, MaxCostUSD: 1.00})
	if !errors.Is(err, engine.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded", err)
	}
	if cheap.calls != 1 {
		t.Errorf("cheap.calls = %d, want 1 (it ran before the refusal)", cheap.calls)
	}
	if pricey.calls != 0 {
		t.Errorf("pricey.calls = %d, want 0 (refused before Execute)", pricey.calls)
	}
}
