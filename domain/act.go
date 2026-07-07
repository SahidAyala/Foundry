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
	CheckedFindings []string   `json:"checked_findings"`
	Patch           string     `json:"patch"`
	JudgmentVerdict string     `json:"judgment_verdict"`
	ApprovedBy      string     `json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
	Iterations      int        `json:"iterations"`
	CostEstimateUSD float64    `json:"cost_estimate_usd"`
}

// Intent describes what was requested.
type Intent struct {
	Text string
}

// Outcome is what the executor produced.
type Outcome struct {
	Patch string
}

// Budget is an enforceable ceiling on an Act — enforced as a constraint,
// never merely reported as a metric.
type Budget struct {
	MaxIterations int
	MaxCostUSD    float64
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
