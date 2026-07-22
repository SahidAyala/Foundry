package engine_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// slogLines decodes each line of buf as a JSON log record and returns its
// "msg", "level", and any extra key this test asserts on.
func slogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if raw == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("decode log line %q: %v", raw, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestSlogReporter_EmitsStructuredEventsAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := engine.NewSlogReporter(logger)

	r.Gathering()
	r.Executing(2)
	r.Verifying(2)
	r.Verified(2, &domain.Judgment{Verdict: "fail", Checked: []string{"go-test: fail"}})
	r.Repairing()

	lines := slogLines(t, &buf)
	if len(lines) != 5 {
		t.Fatalf("got %d log lines, want 5:\n%s", len(lines), buf.String())
	}

	wantMsgs := []string{"act.gather.start", "act.execute.start", "act.verify.start", "act.verify.done", "act.repair.start"}
	for i, want := range wantMsgs {
		if got := lines[i]["msg"]; got != want {
			t.Errorf("line %d msg = %v, want %q", i, got, want)
		}
		if got := lines[i]["level"]; got != "INFO" {
			t.Errorf("line %d level = %v, want INFO", i, got)
		}
	}

	verified := lines[3]
	if got := verified["iteration"]; got != float64(2) {
		t.Errorf("verified iteration = %v, want 2", got)
	}
	if got := verified["verdict"]; got != "fail" {
		t.Errorf("verified verdict = %v, want fail", got)
	}
	if got := verified["findings"]; got != float64(1) {
		t.Errorf("verified findings = %v, want 1", got)
	}
}

func TestSlogReporter_RepairSkippedAndBudgetExceededAreWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := engine.NewSlogReporter(logger)

	r.RepairSkipped("budget too low")
	r.BudgetExceeded("cap reached")

	lines := slogLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2:\n%s", len(lines), buf.String())
	}
	if lines[0]["level"] != "WARN" || lines[0]["msg"] != "act.repair.skipped" || lines[0]["reason"] != "budget too low" {
		t.Errorf("RepairSkipped line = %v", lines[0])
	}
	if lines[1]["level"] != "WARN" || lines[1]["msg"] != "act.budget.exceeded" || lines[1]["reason"] != "cap reached" {
		t.Errorf("BudgetExceeded line = %v", lines[1])
	}
}

func TestSlogReporter_ExecutedIncludesActualCostWhenPresent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := engine.NewSlogReporter(logger)

	cost := 0.0042
	r.Executed(1, &cost)

	lines := slogLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1:\n%s", len(lines), buf.String())
	}
	if lines[0]["msg"] != "act.execute.done" || lines[0]["level"] != "INFO" {
		t.Errorf("Executed line = %v", lines[0])
	}
	if got := lines[0]["actual_cost_usd"]; got != 0.0042 {
		t.Errorf("actual_cost_usd = %v, want 0.0042", got)
	}
}

func TestSlogReporter_ExecutedOmitsActualCostWhenNil(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	r := engine.NewSlogReporter(logger)

	r.Executed(1, nil)

	lines := slogLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1:\n%s", len(lines), buf.String())
	}
	if _, present := lines[0]["actual_cost_usd"]; present {
		t.Errorf("actual_cost_usd present = %v, want absent when Executor reported no actual cost", lines[0]["actual_cost_usd"])
	}
}
