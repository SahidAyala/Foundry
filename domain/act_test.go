package domain

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

func TestNewAct(t *testing.T) {
	intent := "add logging to main.go"
	act := NewAct(intent)

	if act == nil {
		t.Fatal("NewAct returned nil")
	}
	if act.ID == "" {
		t.Error("Act.ID is empty")
	}
	if act.Intent != intent {
		t.Errorf("Act.Intent = %q, want %q", act.Intent, intent)
	}
	if act.CreatedAt.IsZero() {
		t.Error("Act.CreatedAt is zero")
	}
}

func TestNewAct_IDLength(t *testing.T) {
	act := NewAct("test")
	if len(act.ID) != 16 {
		t.Errorf("Act.ID length = %d, want 16", len(act.ID))
	}
}

func TestNewAct_IDFormat(t *testing.T) {
	act := NewAct("test")
	hexPattern := regexp.MustCompile(`^[a-f0-9]{16}$`)
	if !hexPattern.MatchString(act.ID) {
		t.Errorf("Act.ID = %q, does not match hexadecimal pattern", act.ID)
	}
}

func TestNewAct_TimestampPrecision(t *testing.T) {
	before := time.Now()
	act := NewAct("test")
	after := time.Now()

	if act.CreatedAt.Before(before) || act.CreatedAt.After(after.Add(time.Second)) {
		t.Errorf("Act.CreatedAt = %v, not within [%v, %v]", act.CreatedAt, before, after)
	}
}

func TestNewAct_IDUniqueness(t *testing.T) {
	acts := make([]*Act, 100)
	idSet := make(map[string]bool)

	for i := 0; i < 100; i++ {
		acts[i] = NewAct("test")
		idSet[acts[i].ID] = true
	}

	if len(idSet) != 100 {
		t.Errorf("Generated 100 Acts but only %d unique IDs", len(idSet))
	}
}

func TestActJSONRoundTrip(t *testing.T) {
	now := time.Now()
	original := &Act{
		ID:              "a1b2c3d4e5f6g7h8",
		Intent:          "add logging to main.go",
		CreatedAt:       now,
		ConsideredFiles: []string{"main.go", "go.mod"},
		BuildOutput:     "go build ./...\nok",
		TestOutput:      "ok\tmodule/...\t0.001s",
		BuildPassed:     true,
		TestPassed:      true,
		Patch:           "diff --git a/main.go b/main.go\nindex abc..def",
		JudgmentVerdict: "pass",
		ApprovedBy:      "alice",
		ApprovedAt:      &now,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	// Unmarshal from JSON
	unmarshaled := &Act{}
	err = json.Unmarshal(data, unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify all fields round-trip correctly
	if unmarshaled.ID != original.ID {
		t.Errorf("ID: got %q, want %q", unmarshaled.ID, original.ID)
	}
	if unmarshaled.Intent != original.Intent {
		t.Errorf("Intent: got %q, want %q", unmarshaled.Intent, original.Intent)
	}
	if !unmarshaled.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", unmarshaled.CreatedAt, original.CreatedAt)
	}
	if len(unmarshaled.ConsideredFiles) != len(original.ConsideredFiles) {
		t.Errorf("ConsideredFiles length: got %d, want %d", len(unmarshaled.ConsideredFiles), len(original.ConsideredFiles))
	}
	for i, f := range unmarshaled.ConsideredFiles {
		if f != original.ConsideredFiles[i] {
			t.Errorf("ConsideredFiles[%d]: got %q, want %q", i, f, original.ConsideredFiles[i])
		}
	}
	if unmarshaled.BuildOutput != original.BuildOutput {
		t.Errorf("BuildOutput: got %q, want %q", unmarshaled.BuildOutput, original.BuildOutput)
	}
	if unmarshaled.TestOutput != original.TestOutput {
		t.Errorf("TestOutput: got %q, want %q", unmarshaled.TestOutput, original.TestOutput)
	}
	if unmarshaled.BuildPassed != original.BuildPassed {
		t.Errorf("BuildPassed: got %v, want %v", unmarshaled.BuildPassed, original.BuildPassed)
	}
	if unmarshaled.TestPassed != original.TestPassed {
		t.Errorf("TestPassed: got %v, want %v", unmarshaled.TestPassed, original.TestPassed)
	}
	if unmarshaled.Patch != original.Patch {
		t.Errorf("Patch: got %q, want %q", unmarshaled.Patch, original.Patch)
	}
	if unmarshaled.JudgmentVerdict != original.JudgmentVerdict {
		t.Errorf("JudgmentVerdict: got %q, want %q", unmarshaled.JudgmentVerdict, original.JudgmentVerdict)
	}
	if unmarshaled.ApprovedBy != original.ApprovedBy {
		t.Errorf("ApprovedBy: got %q, want %q", unmarshaled.ApprovedBy, original.ApprovedBy)
	}
	if unmarshaled.ApprovedAt == nil && original.ApprovedAt != nil {
		t.Error("ApprovedAt: got nil, want non-nil")
	} else if unmarshaled.ApprovedAt != nil && original.ApprovedAt != nil {
		if !unmarshaled.ApprovedAt.Equal(*original.ApprovedAt) {
			t.Errorf("ApprovedAt: got %v, want %v", *unmarshaled.ApprovedAt, *original.ApprovedAt)
		}
	}
}

func TestActFields(t *testing.T) {
	// Verify that Act has all 12 expected fields with correct types.
	// This is a compile-time check; if a field is missing or wrong, the code won't compile.
	_ = &Act{
		ID:              "",
		Intent:          "",
		CreatedAt:       time.Time{},
		ConsideredFiles: []string{},
		BuildOutput:     "",
		TestOutput:      "",
		BuildPassed:     false,
		TestPassed:      false,
		Patch:           "",
		JudgmentVerdict: "",
		ApprovedBy:      "",
		ApprovedAt:      nil,
	}
}
