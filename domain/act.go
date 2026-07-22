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
	ID     string `json:"id"`
	Intent string `json:"intent"`
	// Pipeline is the name of the Pipeline that produced this Act — set
	// once, at Engine.RunBudgeted. Resume (engine/engine.go) needs it to
	// look up the same declared Steps an interrupted attempt was running.
	Pipeline        string     `json:"pipeline,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ConsideredFiles []string   `json:"considered_files"`
	CheckedFindings []string   `json:"checked_findings"`
	Patch           string     `json:"patch"`
	JudgmentVerdict string     `json:"judgment_verdict"`
	ApprovedBy      string     `json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
	Iterations      int        `json:"iterations"`
	CostEstimateUSD float64    `json:"cost_estimate_usd"`
	// ActualCostUSD is the sum of every generate Step's own ActualCostUSD
	// that was reported (ADR-0011, docs/03-adrs/ADR-0011-cost-as-a-first-class-constraint.md)
	// — nil until at least one Executor reports one, distinguishing "never
	// reported" from "reported as zero." Reported Evidence only: it never
	// gates Budget, which is enforced solely from CostEstimateUSD's
	// pre-execution estimates. May be a partial total if only some of the
	// Act's generate Steps' Executors could report a real cost — see
	// ActualCostCoverage.
	ActualCostUSD *float64     `json:"actual_cost_usd,omitempty"`
	Steps         []StepRecord `json:"steps,omitempty"`
}

// ActualCostCoverage reports how many of Act's generate Steps recorded an
// actual, post-execution cost (StepRecord.ActualCostUSD non-nil) against
// how many generate Steps ran in total, so a caller can render
// ActualCostUSD's total honestly — "actual cost for N of M generate
// Steps" — rather than silently implying a partial sum is complete
// (ADR-0011 Decision 3).
func (a *Act) ActualCostCoverage() (reported, total int) {
	for _, step := range a.Steps {
		if step.Kind != StepKindGenerate {
			continue
		}
		total++
		if step.ActualCostUSD != nil {
			reported++
		}
	}
	return reported, total
}

// Step kinds a StepRecord may carry. RFC-0002 §4.2 closes this vocabulary at
// five: Generate and Verify are executed by PipelineStrategy today; Approve,
// Apply, and Record decode and validate (engine/document.go) but are not yet
// executed by PipelineStrategy — that lands with RFC-0002 §9 Phase 4, which
// moves approval, applying, and recording out of the CLI and into declared
// Steps.
const (
	StepKindGenerate = "generate"
	StepKindVerify   = "verify"
	StepKindApprove  = "approve"
	StepKindApply    = "apply"
	StepKindRecord   = "record"
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
	// ActualCostUSD is a generate Step's own real, post-execution cost, if
	// its Executor could report one (ADR-0011) — nil for every other Step
	// kind, and nil for a generate Step whose Executor has no billing
	// signal to report (e.g. executor/claude.ClaudeExecutor).
	ActualCostUSD *float64 `json:"actual_cost_usd,omitempty"`
}

// Intent describes what was requested.
type Intent struct {
	Text string
}

// Outcome is what the executor produced.
type Outcome struct {
	Patch string
	// ActualCostUSD is the real, post-execution cost of the Execute call
	// that produced this Outcome, if the Executor can report one — nil
	// when it cannot (ADR-0011, docs/03-adrs/ADR-0011-cost-as-a-first-class-constraint.md).
	// Never used to gate Budget (see Budget's own doc comment below):
	// CostEstimator's pre-execution estimate remains the sole enforcement
	// signal; this is reported Evidence only.
	ActualCostUSD *float64
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
