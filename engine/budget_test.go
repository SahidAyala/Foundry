package engine

import (
	"errors"
	"testing"

	"foundry/domain"
)

func TestDefaultBudget(t *testing.T) {
	b := DefaultBudget()
	if b.MaxIterations != 4 {
		t.Errorf("MaxIterations = %d, want 4", b.MaxIterations)
	}
	if b.MaxCostUSD != 2.00 {
		t.Errorf("MaxCostUSD = %v, want 2.00", b.MaxCostUSD)
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

// TestTracker_NegativeEstimateRefusedNotSilentlyLoosened covers a broken
// CostEstimator (or ADR-0011's actualCostUSD) reporting a negative
// estimate: without a guard, costUSD+estimateUSD > MaxCostUSD would
// evaluate false, decrementing costUSD and permanently loosening the
// ceiling for every later charge in the same Act — a silent defeat of
// Budget's enforcement, not merely a bad number.
func TestTracker_NegativeEstimateRefusedNotSilentlyLoosened(t *testing.T) {
	spent := &tracker{budget: &domain.Budget{MaxIterations: 10, MaxCostUSD: 1.00}}

	if err := spent.charge(0.50); err != nil {
		t.Fatalf("first charge failed: %v", err)
	}
	if err := spent.charge(-100); err == nil {
		t.Fatal("charge(-100) succeeded, want an error refusing a negative estimate")
	} else if errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("charge(-100) error wraps ErrBudgetExceeded, want a distinct error — a negative estimate is a broken component, not a legitimate budget refusal: %v", err)
	}
	if spent.iterations != 1 || spent.costUSD != 0.50 {
		t.Errorf("refused negative charge changed tracker state: iterations=%d costUSD=%v, want unchanged at 1/0.50", spent.iterations, spent.costUSD)
	}

	// The ceiling must still be enforced correctly afterward — this is the
	// concrete failure a missing guard would produce: a negative charge
	// silently making room for more spend than MaxCostUSD allows.
	if err := spent.charge(0.50); err != nil {
		t.Fatalf("charge reaching the ceiling exactly must still pass, got: %v", err)
	}
	if err := spent.charge(0.01); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("charge over the ceiling = %v, want ErrBudgetExceeded", err)
	}
}
