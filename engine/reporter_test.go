package engine_test

import (
	"reflect"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

func TestMultiReporter_FansOutEveryEventToEachReporter(t *testing.T) {
	a, b := &fakeReporter{}, &fakeReporter{}
	m := engine.MultiReporter{Reporters: []engine.Reporter{a, b}}

	cost := 0.0042
	m.Gathering()
	m.Executing(1)
	m.Executed(1, &cost)
	m.Verifying(1)
	m.Verified(1, &domain.Judgment{Verdict: "pass"})
	m.Repairing()
	m.RepairSkipped("budget too low")
	m.BudgetExceeded("cap reached")

	want := []string{
		"gathering",
		"executing:1",
		"executed:1:0.0042",
		"verifying:1",
		"verified:1:pass",
		"repairing",
		"repair-skipped:budget too low",
		"budget-exceeded:cap reached",
	}
	for _, got := range [][]string{a.events, b.events} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("events = %v, want %v", got, want)
		}
	}
}

func TestMultiReporter_ZeroReportersIsNoop(t *testing.T) {
	var m engine.MultiReporter
	// Must not panic with no Reporters attached.
	m.Gathering()
	m.Executing(1)
	cost := 0.01
	m.Executed(1, &cost)
	m.Verifying(1)
	m.Verified(1, &domain.Judgment{Verdict: "pass"})
	m.Repairing()
	m.RepairSkipped("reason")
	m.BudgetExceeded("reason")
}
