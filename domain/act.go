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
	ID              string       `json:"id"`
	Intent          string       `json:"intent"`
	CreatedAt       time.Time    `json:"created_at"`
	ConsideredFiles []string     `json:"considered_files"`
	CheckedFindings []string     `json:"checked_findings"`
	Patch           string       `json:"patch"`
	JudgmentVerdict string       `json:"judgment_verdict"`
	ApprovedBy      string       `json:"approved_by"`
	ApprovedAt      *time.Time   `json:"approved_at"`
	Iterations      int          `json:"iterations"`
	CostEstimateUSD float64      `json:"cost_estimate_usd"`
	Steps           []StepRecord `json:"steps,omitempty"`
}

// Step kinds a StepRecord may carry today. The full vocabulary proposed in
// docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §4.2 also includes
// "approve", "apply", and "record"; those happen above the Engine (in the
// CLI) and are added to this set only once that part of the trace is wired
// up (RFC-0002 §9 Phase 4), not before.
const (
	StepKindGenerate = "generate"
	StepKindVerify   = "verify"
)

// StepRecord is one recorded attempt at a unit of work within an Act's
// production: a single Executor call (a "generate" step) or a single
// verification pass (a "verify" step). An Act accumulates StepRecords in
// the order they ran, including repair rounds, so its trace can answer
// "what happened at step N" instead of exposing only the latest considered
// and checked Evidence.
//
// StepRecord is additive: it coexists with Act's flat fields (Patch,
// JudgmentVerdict, ConsideredFiles, CheckedFindings, ...), which remain the
// final-round view existing callers already rely on. See
// docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §4.5 and §9 Phase 1.
type StepRecord struct {
	StepID          string    `json:"step_id"`
	Kind            string    `json:"kind"`
	Considered      []string  `json:"considered,omitempty"`
	Produced        []string  `json:"produced,omitempty"`
	Checked         []string  `json:"checked,omitempty"`
	JudgmentVerdict string    `json:"judgment_verdict,omitempty"`
	Authority       string    `json:"authority,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
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
