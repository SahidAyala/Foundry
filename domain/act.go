package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Act is the immutable, recorded unit of work.
// It carries an Intent, accumulates Evidence, yields an Outcome, and passes a Judgment.
// Once recorded to durable storage, an Act is never modified.
type Act struct {
	ID              string     `json:"id"`
	Intent          string     `json:"intent"`
	CreatedAt       time.Time  `json:"created_at"`
	ConsideredFiles []string   `json:"considered_files"`
	BuildOutput     string     `json:"build_output"`
	TestOutput      string     `json:"test_output"`
	BuildPassed     bool       `json:"build_passed"`
	TestPassed      bool       `json:"test_passed"`
	Patch           string     `json:"patch"`
	JudgmentVerdict string     `json:"judgment_verdict"`
	ApprovedBy      string     `json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
}

// Intent describes what was requested.
type Intent struct {
	Text string
}

// Evidence is what was considered (context) and what was checked (verification results).
type Evidence struct {
	Considered []string
	Checked    []string
}

// Outcome is what the executor produced.
type Outcome struct {
	Patch string
}

// Judgment is the verdict and approval decision.
type Judgment struct {
	Verdict   string
	Authority string
	At        *time.Time
	Checked   []string // validator output that produced the verdict
}

// NewAct creates a new Act with a unique ID and current timestamp.
func NewAct(intent string) *Act {
	return &Act{
		ID:        generateID(),
		Intent:    intent,
		CreatedAt: time.Now(),
	}
}

// generateID creates a unique 16-character hexadecimal ID.
func generateID() string {
	b := make([]byte, 8) // 8 bytes = 16 hex digits
	_, err := rand.Read(b)
	if err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
