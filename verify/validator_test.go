package verify

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

// TestValidator_Run_TimesOutOnHangingCommand guards the fix for a real gap:
// production wires no deadline on the context `foundry do` runs under, so a
// hanging validator (an AI-generated infinite loop, most plausibly inside
// its own test suite) previously hung the entire Act forever. A timeout
// must resolve quickly and fail cleanly, not hang or error out.
func TestValidator_Run_TimesOutOnHangingCommand(t *testing.T) {
	v := &Validator{Name: "hang", Cmd: "sleep 5", Timeout: 50 * time.Millisecond}

	start := time.Now()
	result, err := v.Run(context.Background(), t.TempDir())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run returned error on timeout: %v (a timeout must be a failed Result, not an error)", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Run took %s, want it bounded by the 50ms Timeout", elapsed)
	}
	if result.Passed {
		t.Error("Passed = true, want false for a timed-out validator")
	}
	if !strings.Contains(result.Output, "timed out") {
		t.Errorf("Output = %q, want it to mention the timeout", result.Output)
	}
}

// TestValidator_Run_DefaultTimeoutDoesNotBlockFastCommands guards against a
// regression where the default timeout itself becomes the bottleneck for
// the common case: a fast command must still return immediately.
func TestValidator_Run_DefaultTimeoutDoesNotBlockFastCommands(t *testing.T) {
	v := &Validator{Name: "fast", Cmd: "exit 0"} // Timeout unset

	start := time.Now()
	result, err := v.Run(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Run took %s for a fast command, want near-instant", elapsed)
	}
	if !result.Passed {
		t.Error("Passed = false, want true")
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
