package engine

// Step is one instruction in a Pipeline: what kind of work to perform.
// Step's Kind reuses the domain.StepKind* vocabulary that also labels the
// StepRecord an executed Step leaves in an Act's trace (domain/act.go), so
// a Pipeline's declared shape and an Act's recorded shape share one
// vocabulary. ID doubles as Step's human-readable name: RFC-0002 §4.3 calls
// a repair target "a named earlier Step", and ID is that name — no
// separate label field is needed.
//
// Capability, Executor, FeedsForward, and Target are RFC-0004 §2's Router
// and Knowledge-lite-capture groundwork
// (docs/04-guides/multi-executor-router-implementation-plan.md Pieces 1 and
// 4): all four are additive and zero-valued by default, so a Step literal
// or PipelineDocument written before they existed keeps its exact current
// behavior. Capability is carried but not yet interpreted by any Router
// policy — Piece 1's Router (router.go) is explicit-pin-only;
// capability-based negotiation is RFC-0002 §7 layer 2, out of scope until a
// real multi-Executor Pipeline motivates it. Executor is the explicit pin a
// Router resolves against an ExecutorRegistry; empty means "the Engine's
// default Executor," exactly what every Step meant before Executor existed.
// FeedsForward, when true, has runSteps append the immediately-preceding
// Step's recorded output to this Step's Context — never an arbitrarily
// named earlier Step, per RFC-0004 §3. Target is meaningful only for a
// domain.StepKindApply Step: which Applier resolves it — ApplyTargetLocal
// (the empty string, the default, today's only behavior — the Engine's
// single configured Applier) or a name registered in an ApplierRegistry
// (ApplyTargetKnowledgeNote, ApplyTargetProjectDoc — RFC-0004 §2.6).
type Step struct {
	ID           string
	Kind         string
	Capability   map[string]string
	Executor     string
	FeedsForward bool
	Target       string
}

// Apply Step target names RFC-0004 §2.6's Knowledge-lite capture and
// ADR-0010's VCS/PR integration define, resolved by
// runContext.resolveApplier (strategy.go) against an ApplierRegistry.
// ApplyTargetLocal is never actually registered in one — it, and the empty
// string a Step predating Target always has, both mean "use the Engine's
// single configured Applier directly," exactly what every apply Step meant
// before Target existed. ApplyTargetRemotePR is additionally subject to
// PipelineRegistry.SetPublishPolicy's load-time approval check
// (registry.go, ADR-0010 Decision 3) — Foundry's first apply target that
// leaves the developer's own machine.
const (
	ApplyTargetLocal         = "local"
	ApplyTargetKnowledgeNote = "knowledge-note"
	ApplyTargetProjectDoc    = "project-doc"
	ApplyTargetRemotePR      = "remote-pr"
)

// RepairPolicy bounds how many times a Pipeline may be re-run after its
// final verify Step's Judgment is "fail", and names where each re-run jumps
// back to. Each re-run is fed the failing Judgment's findings as additional
// Context. This is today's single bounded repair round
// (docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md PR-011), expressed as data a
// Strategy interprets instead of a bespoke Go function.
//
// RFC-0002 §4.2 describes a Pipeline's full shape as "an ordered list of
// Steps plus named backward repair edges" and §4.3 calls the destination "a
// named earlier Step." Target is that name.
type RepairPolicy struct {
	MaxAttempts int

	// Target is the ID of the Step a repair round jumps back to, re-running
	// every Step from Target onward (not the whole Pipeline) with the
	// failing Judgment's findings added to Context. Empty means "restart
	// from Pipeline.Steps[0]" — the only behavior that existed before this
	// field did, preserved as the zero value so every Pipeline document
	// that predates Target keeps its exact current behavior.
	Target string
}

// Pipeline is an ordered sequence of Steps a Strategy executes to produce
// an Act's Outcome and Judgment. It is the runtime shape a Strategy walks,
// not the authored shape a Pipeline is written in: a Pipeline is authored
// as a declarative PipelineDocument (document.go) and translated into this
// type by DecodePipelineDocument, so its schema and evolution stay
// separate from what PipelineStrategy actually executes
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §9 Phase 3). A
// Pipeline is discovered by a PipelineProvider (provider.go —
// DefaultPipeline is built in via BuiltinProvider, builtin_provider.go,
// which now decodes its document rather than constructing this type by
// hand) and identified by Name within a PipelineRegistry (registry.go).
//
// Audited against RFC-0002 (§4.2, §4.5, §5, §6, §7) for missing structural
// concepts: Name, Steps, and RepairPolicy already cover everything the
// runtime executes today. Deliberately not added, and why:
//   - Pipeline/Step versioning (§6) — ADR-0004 ("Reusable-Act template
//     format & evolution policy", accepted) deliberately adds no version
//     field yet, ratifying additive-only field evolution instead; a real
//     version scheme is introduced only once a second reader of this
//     format actually exists.
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
