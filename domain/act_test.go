package domain

import (
	"encoding/json"
	"regexp"
	"strings"
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
		CheckedFindings: []string{"go-build: pass", "go-test: pass"},
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
	if len(unmarshaled.CheckedFindings) != len(original.CheckedFindings) {
		t.Errorf("CheckedFindings length: got %d, want %d", len(unmarshaled.CheckedFindings), len(original.CheckedFindings))
	}
	for i, f := range unmarshaled.CheckedFindings {
		if f != original.CheckedFindings[i] {
			t.Errorf("CheckedFindings[%d]: got %q, want %q", i, f, original.CheckedFindings[i])
		}
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

func TestJudgmentFields(t *testing.T) {
	// Verify that Judgment has all 4 expected fields with correct types.
	// This is a compile-time check; if a field is missing or wrong, the code won't compile.
	_ = &Judgment{
		Verdict:   "",
		Authority: "",
		At:        nil,
		Checked:   []string{},
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
		CheckedFindings: []string{},
		Patch:           "",
		JudgmentVerdict: "",
		ApprovedBy:      "",
		ApprovedAt:      nil,
		Iterations:      0,
		CostEstimateUSD: 0,
		Steps:           []StepRecord{},
	}
}

func TestStepRecordFields(t *testing.T) {
	// Verify that StepRecord has all 8 expected fields with correct types.
	// This is a compile-time check; if a field is missing or wrong, the code won't compile.
	_ = &StepRecord{
		StepID:          "",
		Kind:            "",
		Considered:      []string{},
		Produced:        []string{},
		Checked:         []string{},
		JudgmentVerdict: "",
		Authority:       "",
		StartedAt:       time.Time{},
		FinishedAt:      time.Time{},
	}
}

func TestActJSONRoundTrip_Steps(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Second)
	original := &Act{
		ID:        "a1b2c3d4e5f6g7h8",
		Intent:    "add logging to main.go",
		CreatedAt: started,
		Steps: []StepRecord{
			{
				StepID:     "1",
				Kind:       StepKindGenerate,
				Considered: []string{"main.go"},
				Produced:   []string{"diff --git a/main.go b/main.go"},
				StartedAt:  started,
				FinishedAt: finished,
			},
			{
				StepID:          "2",
				Kind:            StepKindVerify,
				Checked:         []string{"go-build: pass"},
				JudgmentVerdict: "pass",
				StartedAt:       finished,
				FinishedAt:      finished,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	unmarshaled := &Act{}
	if err := json.Unmarshal(data, unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(unmarshaled.Steps) != 2 {
		t.Fatalf("Steps length = %d, want 2", len(unmarshaled.Steps))
	}
	if unmarshaled.Steps[0].Kind != StepKindGenerate || unmarshaled.Steps[0].StepID != "1" {
		t.Errorf("Steps[0] = %+v, want Kind=%q StepID=1", unmarshaled.Steps[0], StepKindGenerate)
	}
	if unmarshaled.Steps[1].Kind != StepKindVerify || unmarshaled.Steps[1].JudgmentVerdict != "pass" {
		t.Errorf("Steps[1] = %+v, want Kind=%q JudgmentVerdict=pass", unmarshaled.Steps[1], StepKindVerify)
	}
}

func TestActJSONRoundTrip_StepsOmittedWhenEmpty(t *testing.T) {
	original := NewAct("test")

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if strings.Contains(string(data), `"steps"`) {
		t.Errorf("JSON contains a \"steps\" key for an Act with no Steps; want it omitted:\n%s", data)
	}
}
