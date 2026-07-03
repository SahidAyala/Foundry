package engine

import (
	"errors"
	"testing"

	"foundry/domain"
)

func TestDefaultBudget(t *testing.T) {
	b := DefaultBudget()
	if b.MaxIterations != 2 {
		t.Errorf("MaxIterations = %d, want 2", b.MaxIterations)
	}
	if b.MaxCostUSD != 1.00 {
		t.Errorf("MaxCostUSD = %v, want 1.00", b.MaxCostUSD)
	}
}

func TestTracker_ChargeWithinBudget(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{MaxIterations: 2, MaxCostUSD: 1.00}}

	for i := 1; i <= 2; i++ {
		if err := spent.charge(0.50); err != nil {
			t.Fatalf("charge %d failed: %v", i, err)
		}
	}
	if spent.iterations != 2 {
		t.Errorf("iterations = %d, want 2", spent.iterations)
	}
	if spent.costUSD != 1.00 {
		t.Errorf("costUSD = %v, want 1.00", spent.costUSD)
	}
}

func TestTracker_IterationCeiling(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{MaxIterations: 1, MaxCostUSD: 100}}

	if err := spent.charge(0.50); err != nil {
		t.Fatalf("first charge failed: %v", err)
	}
	err := spent.charge(0.50)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second charge error = %v, want ErrBudgetExceeded", err)
	}
	if spent.iterations != 1 || spent.costUSD != 0.50 {
		t.Errorf("refused charge consumed budget: iterations=%d costUSD=%v", spent.iterations, spent.costUSD)
	}
}

func TestTracker_CostCeiling(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{MaxIterations: 100, MaxCostUSD: 0.75}}

	if err := spent.charge(0.50); err != nil {
		t.Fatalf("first charge failed: %v", err)
	}
	err := spent.charge(0.50) // would reach $1.00 > $0.75
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second charge error = %v, want ErrBudgetExceeded", err)
	}
	if spent.iterations != 1 || spent.costUSD != 0.50 {
		t.Errorf("refused charge consumed budget: iterations=%d costUSD=%v", spent.iterations, spent.costUSD)
	}
}

func TestTracker_CostAtCeilingAllowed(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{MaxIterations: 2, MaxCostUSD: 1.00}}

	if err := spent.charge(0.50); err != nil {
		t.Fatalf("first charge failed: %v", err)
	}
	if err := spent.charge(0.50); err != nil {
		t.Fatalf("charge reaching the ceiling exactly must pass, got: %v", err)
	}
}

func TestTracker_ZeroBudgetRefusesFirstCharge(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{}}

	if err := spent.charge(0.50); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("charge error = %v, want ErrBudgetExceeded", err)
	}
}
