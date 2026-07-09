package engine

// Step is one instruction in a Pipeline: what kind of work to perform.
// Step's Kind reuses the domain.StepKind* vocabulary that also labels the
// StepRecord an executed Step leaves in an Act's trace (domain/act.go), so
// a Pipeline's declared shape and an Act's recorded shape share one
// vocabulary. ID doubles as Step's human-readable name: RFC-0002 §4.3 calls
// a repair target "a named earlier Step", and ID is that name — no
// separate label field is needed.
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
//
// RFC-0002 §4.2 describes a Pipeline's full shape as "an ordered list of
// Steps plus named backward repair edges." RepairPolicy does not yet name
// a repair target Step because no Pipeline this build ships needs one:
// with only Generate and Verify Steps, "restart from the top" and "jump to
// the named first Step" are the same behavior. A named target belongs here
// once a Pipeline has more than one plausible repair destination — which
// arrives with Approve/Apply Steps (RFC-0002 §9 Phase 4), not before.
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
//
// Audited against RFC-0002 (§4.2, §4.5, §5, §6, §7) for missing structural
// concepts: Name, Steps, and RepairPolicy already cover everything the
// runtime executes today. Deliberately not added, and why:
//   - Pipeline/Step versioning (§6) — the versioning scheme is an unwritten
//     prerequisite ADR's decision (§9 Phase 0), not this type's; guessing
//     at a shape now risks the premature-hardening §10 warns against for
//     the Record's on-disk surface.
//   - A named repair-jump target (§4.2, §4.3) — see RepairPolicy.
//   - Capabilities, model hints, or routing metadata (§4.4, §7) — belong
//     to the Router, which does not exist until Phase 6.
//   - A Retry policy distinct from Repair (§5) — tied to Executor failover
//     (§7, Phase 7); no Executor failure mode in this codebase motivates
//     it independently of failover.
//   - Per-Step execution constraints, e.g. a per-Step Budget slice (§3
//     limitation #10, §10 risk) — Budget stays Act-level until a Pipeline
//     with heterogeneous per-Step cost actually exists.
type Pipeline struct {
	Name   string
	Steps  []Step
	Repair RepairPolicy
}
