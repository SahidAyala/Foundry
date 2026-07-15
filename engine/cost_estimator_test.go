package engine

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
)

// plainExecutor implements only Executor, not CostEstimator — the shape of
// executor/claude.ClaudeExecutor and executor.ScriptedExecutor today.
type plainExecutor struct{}

func (plainExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return &domain.Outcome{}, nil
}

// costEstimatingExecutor implements both Executor and CostEstimator,
// returning a fixed estimate — the shape of executor/openai.Executor.
type costEstimatingExecutor struct {
	estimate float64
	err      error
}

func (costEstimatingExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	return &domain.Outcome{}, nil
}

func (e costEstimatingExecutor) EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error) {
	return e.estimate, e.err
}

var _ Executor = plainExecutor{}
var _ Executor = costEstimatingExecutor{}
var _ CostEstimator = costEstimatingExecutor{}

func TestEstimateExecuteCostUSD_FallsBackWithoutCostEstimator(t *testing.T) {
	got, err := estimateExecuteCostUSD(context.Background(), plainExecutor{}, &domain.Intent{}, nil)
	if err != nil {
		t.Fatalf("estimateExecuteCostUSD failed: %v", err)
	}
	if got != executeCostEstimateUSD {
		t.Errorf("estimate = %v, want flat fallback %v", got, executeCostEstimateUSD)
	}
}

func TestEstimateExecuteCostUSD_UsesCostEstimator(t *testing.T) {
	exec := costEstimatingExecutor{estimate: 1.23}
	got, err := estimateExecuteCostUSD(context.Background(), exec, &domain.Intent{}, nil)
	if err != nil {
		t.Fatalf("estimateExecuteCostUSD failed: %v", err)
	}
	if got != 1.23 {
		t.Errorf("estimate = %v, want the Executor's own 1.23", got)
	}
}

func TestEstimateExecuteCostUSD_PropagatesCostEstimatorError(t *testing.T) {
	wantErr := errors.New("rate-limited")
	exec := costEstimatingExecutor{err: wantErr}
	_, err := estimateExecuteCostUSD(context.Background(), exec, &domain.Intent{}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
