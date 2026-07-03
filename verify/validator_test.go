package verify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"foundry/domain"
)

func TestValidator_Run_Passes(t *testing.T) {
	v := &Validator{Name: "true-check", Cmd: "exit 0"}

	result, err := v.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Passed {
		t.Error("Passed = false, want true")
	}
	if result.Name != "true-check" {
		t.Errorf("Name = %q, want %q", result.Name, "true-check")
	}
}

func TestValidator_Run_Fails(t *testing.T) {
	v := &Validator{Name: "false-check", Cmd: "echo boom && exit 1"}

	result, err := v.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Passed {
		t.Error("Passed = true, want false")
	}
	if !strings.Contains(result.Output, "boom") {
		t.Errorf("Output = %q, want it to contain %q", result.Output, "boom")
	}
}

func TestValidator_Run_InvalidWorkspace(t *testing.T) {
	v := &Validator{Name: "bad-workspace", Cmd: "exit 0"}

	if _, err := v.Run(context.Background(), "/nonexistent/workspace/path"); err == nil {
		t.Fatal("Run with nonexistent workspace returned nil error")
	}
}

func TestNewGate_RejectsUnsupportedRule(t *testing.T) {
	v := &Validator{Name: "check", Cmd: "exit 0"}

	if _, err := NewGate("any-pass", v); err == nil {
		t.Fatal("NewGate with unsupported rule returned nil error")
	}
}

func TestNewGate_RequiresAtLeastOneValidator(t *testing.T) {
	if _, err := NewGate("all-pass"); err == nil {
		t.Fatal("NewGate with no validators returned nil error")
	}
}

func TestGate_Verify_AllPass(t *testing.T) {
	gate, err := NewGate("all-pass",
		&Validator{Name: "one", Cmd: "exit 0"},
		&Validator{Name: "two", Cmd: "exit 0"},
	)
	if err != nil {
		t.Fatalf("NewGate failed: %v", err)
	}

	judgment, err := gate.Verify(context.Background(), &domain.Outcome{}, t.TempDir())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	if len(judgment.Checked) != 2 {
		t.Errorf("len(Checked) = %d, want 2", len(judgment.Checked))
	}
}

func TestGate_Verify_MixedResults(t *testing.T) {
	gate, err := NewGate("all-pass",
		&Validator{Name: "passing", Cmd: "exit 0"},
		&Validator{Name: "failing", Cmd: "exit 1"},
	)
	if err != nil {
		t.Fatalf("NewGate failed: %v", err)
	}

	judgment, err := gate.Verify(context.Background(), &domain.Outcome{}, t.TempDir())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "fail")
	}
	if len(judgment.Checked) != 2 {
		t.Errorf("len(Checked) = %d, want 2", len(judgment.Checked))
	}
}

func TestGate_Verify_PropagatesValidatorError(t *testing.T) {
	gate, err := NewGate("all-pass", &Validator{Name: "bad-workspace", Cmd: "exit 0"})
	if err != nil {
		t.Fatalf("NewGate failed: %v", err)
	}

	if _, err := gate.Verify(context.Background(), &domain.Outcome{}, "/nonexistent/workspace/path"); err == nil {
		t.Fatal("Verify with nonexistent workspace returned nil error")
	}
}

func TestJudgment_GoldenShape(t *testing.T) {
	gate, err := NewGate("all-pass", &Validator{Name: "only", Cmd: "exit 0"})
	if err != nil {
		t.Fatalf("NewGate failed: %v", err)
	}

	judgment, err := gate.Verify(context.Background(), &domain.Outcome{}, t.TempDir())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	data, err := json.MarshalIndent(judgment, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	want := `{
  "Verdict": "pass",
  "Authority": "",
  "At": null,
  "Checked": [
    "only: pass"
  ]
}`

	if string(data) != want {
		t.Errorf("Judgment golden mismatch:\ngot:\n%s\nwant:\n%s", data, want)
	}
}
