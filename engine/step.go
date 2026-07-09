package engine

// Step is one instruction in a Pipeline: what kind of work to perform.
// Step's Kind reuses the domain.StepKind* vocabulary that also labels the
// StepRecord an executed Step leaves in an Act's trace (domain/act.go), so
// a Pipeline's declared shape and an Act's recorded shape share one
// vocabulary.
type Step struct {
	ID   string
	Kind string
}

// RepairPolicy bounds how many times a Pipeline may be re-run, from its
// first Step, after its final verify Step's Judgment is "fail". Each re-run
// is fed the failing Judgment's findings as additional Context. This is
// today's single bounded repair round (docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md
// PR-011), expressed as data a Strategy interprets instead of a bespoke Go
// function.
type RepairPolicy struct {
	MaxAttempts int
}

// Pipeline is an ordered sequence of Steps a Strategy executes to produce
// an Act's Outcome and Judgment. It is Go data, not a user-authored
// document: declarative (e.g. YAML) Pipeline authoring is deferred to a
// later phase (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9
// Phase 3+). A Pipeline is discovered by a PipelineProvider (provider.go —
// DefaultPipeline is built in via BuiltinProvider, builtin_provider.go) and
// identified by Name within a PipelineRegistry (registry.go).
type Pipeline struct {
	Name   string
	Steps  []Step
	Repair RepairPolicy
}
