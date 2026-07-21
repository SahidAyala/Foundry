# ADR-0001 — Language & Toolchain

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-29 |
| **Deciders** | Architecture Review Board |
| **Ratifies** | The recommendation in `ARCHITECTURE.md §5.4` (previously non-binding) |
| **Gates** | `IMPLEMENTATION-ROADMAP.md` PR-001 (and therefore all subsequent work) |
| **Process note** | Accepted under interim board authority. A governance/RFC-process model does not yet exist (flagged P0-2 in `docs/reviews/RFC-0001-review-round-1.md`); when it does, it must retroactively bless this acceptance. This is recorded, not papered over. |

---

## Context

`ARCHITECTURE.md §5.4` recommends Go but marks the choice *decision pending*, and it states that recommendation as a short rationale embedded inside a larger structural argument. That is insufficient to build on for three reasons, and those reasons — not the merits of Go — are why this ADR exists:

1. **It is the least reversible decision in the project.** Every other early decision (storage engine, manifest schema, even the determinism contract) lives behind an interface or a version field and can be migrated. The implementation language cannot; reversing it is a rewrite. A decision with that blast radius must be recorded explicitly, with its alternatives and its costs, so that future contributors inherit the *reasoning*, not just the outcome.
2. **PR-001 cannot be reviewed against a "pending" decision.** The roadmap makes a ratified, citable language decision the literal precondition for the first commit. A recommendation buried in a planning document is not a thing a PR can be checked against.
3. **`§5.4` recorded the choice but not the obligations and the regret-paths.** It did not analyze realistic alternatives at decision grade, did not state what the choice makes *harder*, and did not record the constraints the choice imposes on later ADRs. Those omissions are exactly what an ADR is for.

This ADR deliberately does **not** restate the RFC's philosophy or the architecture's structural rationale. It assumes both as accepted context and records only the decision, the analysis `§5.4` lacked, and the forward-binding consequences.

The decision is shaped by four properties of Foundry that are already fixed by accepted documents and are therefore treated here as **hard requirements, not preferences**:

- **R1 — Single static, cross-platform binary.** The same artifact is a local CLI *and* a headless CI executable (`ARCHITECTURE.md §5.2`, roadmap CI-mode). This is a distribution requirement, not an aesthetic one.
- **R2 — Language-agnostic extension boundary.** The community must never be locked to the kernel's language (`ARCHITECTURE.md §15.2`, RFC goal 7 / V6). Whatever language the kernel uses, plugins must be writable in others.
- **R3 — Low contributor on-ramp.** An open, ownable platform (RFC goal 7) needs a large, low-friction contributor pool more than it needs maximal expressiveness.
- **R4 — Fan-out orchestration & fast cold start.** The runtime parallelizes stages and is invoked repeatedly in CI; startup latency and concurrency ergonomics are on the critical path of the user experience.

---

## Decision

**Foundry's kernel, CLI, and first-party adapters are written in Go.** The decision includes a toolchain policy, because the language alone does not protect the requirements above:

1. **Language: Go**, for the kernel, CLI/TUI surfaces, and all in-tree adapters.
2. **Static-binary policy: `CGO_ENABLED=0` is the default build mode.** Pure-Go dependencies are required for anything on the default build path. This is the operative clause that protects **R1** — it is the reason ADR-0002 must select a pure-Go storage driver, and it is what keeps cross-compilation (`GOOS`/`GOARCH`) trivial. cgo is permitted *only* behind an explicit build tag for an *optional* feature, and never on the default build (see Open Questions for the boundary of this rule).
3. **Toolchain version policy:** pin a minimum Go version in `go.mod` and CI; support the **latest two minor Go releases**; upgrades are a deliberate, reviewed change, never implicit.
4. **The extension boundary is not Go.** Per **R2**, the plugin/provider contract is defined in a language-neutral wire format (gRPC/protobuf), decided in ADR-0008. Go is an implementation detail of the *kernel*, not of the *ecosystem*. This clause is binding: any future work that makes a plugin author need to write Go violates this ADR.
5. **Module/import path** is deferred to the resolution of the project name (RFC Open Question 7) — see Open Questions. A provisional path unblocks PR-001 and is cheap to rewrite while there are no external importers.

---

## Alternatives Considered

Each alternative is evaluated against R1–R4 and against the *honest* cost of choosing it. "Rejected" never means "bad"; it means "worse for this system."

### Rust — the strongest alternative
- **For:** Best-in-class type system (sum types, exhaustiveness, ownership) for making illegal states unrepresentable — genuinely attractive for the domain-rich, aggregate-heavy model the architecture describes. No GC. First-class WASM-guest story (relevant to ADR-0008's hardening track). Single static binary (satisfies R1).
- **Against:** Slowest contributor on-ramp of any option (violates R3 hardest); compile times tax iteration on a large codebase; the async ecosystem is powerful but heavy for orchestration glue; the out-of-process plugin host story is less worn-in than Go's. 
- **Verdict:** Rejected on **R3**. The expressiveness win is real and is the single most painful thing we give up (see Consequences → Harder). But a platform whose success depends on community contribution cannot afford the narrowest contributor funnel. We choose a weaker type system and a wider door.

### TypeScript / Node
- **For:** Largest contributor pool and the best DX for the eventual TUI; rich ecosystem; the AI/LLM SDK ecosystem is most mature here.
- **Against:** **Fails R1 outright** — no genuine single static binary; ships a runtime dependency (or a heavyweight bundler like `pkg`/Bun with caveats). Cold start and memory profile are poor for a frequently-invoked CI executable (hurts R4). Dynamic typing pushes invariant-enforcement entirely into tests.
- **Verdict:** Rejected on **R1 + R4**. The distribution model is incompatible with "the same binary runs locally and in CI" without compromises that erode the core UX promise.

### .NET (C#) with NativeAOT
- **For:** Strong type system, good concurrency, and NativeAOT now produces a single self-contained native binary (satisfies R1); excellent tooling.
- **Against:** NativeAOT still carries trimming/reflection caveats that complicate a plugin-loading host; smaller open-source-infrastructure contributor culture than Go in this specific domain (weaker R3); the out-of-process, cross-language plugin ecosystem is thinner than Go's.
- **Verdict:** Rejected, narrowly, on **R3** and ecosystem fit. It is technically viable; it is not where the relevant contributor community lives.

### Others (briefly, to show they were not ignored)
- **Zig** — too young to anchor a decade-stable foundation; toolchain not yet 1.0. Rejected on stability risk.
- **Python** — fails R1 and R4 badly; rejected without deep analysis.

---

## Consequences

### What this decision makes EASIER (and why)

- **ADR-0002 (persistence):** a mature pure-Go SQLite implementation exists, so an embedded storage engine *could* preserve R1 if one were ever needed. **In the event, [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) chose no database at all** — [principles.md](../00-overview/principles.md)'s Filesystem-first persistence principle settled this before it became a live tradeoff, so the tension this clause anticipated never actually arose.
- **ADR-0008 (extension contract):** Go's out-of-process plugin ecosystem (gRPC, `go-plugin`-class tooling) is the most trodden path of any candidate. The *primary* isolation mechanism (gRPC subprocess) becomes low-risk.
- **Distribution & CI (roadmap §7):** trivial cross-compilation to every target from one machine; `goreleaser`-class signed multi-platform releases; the "same binary in CI" promise (R1) is met by construction, not effort.
- **Parallel stage execution (postponed in roadmap §8):** goroutines/channels make the eventual move from sequential to concurrent orchestration an internal change, not a paradigm shift (serves R4).
- **Observability (M5):** the OpenTelemetry Go SDK is first-class; the trace tree the architecture wants falls out cheaply.
- **Contributor onboarding (RFC goal 7):** the largest low-ramp pool of the viable options; a reviewer can be productive in days (serves R3).

### What this decision makes HARDER (stated honestly, not minimized)

- **Expressing domain invariants — the primary cost.** The architecture describes a rich DDD model (aggregates, value objects, a state machine). Go lacks sum types and exhaustive matching and has `nil`. **Making illegal states unrepresentable is harder than in Rust.** Consequence for ADR-0004 and the domain layer: invariants must be enforced by constructors, validation, and tests rather than by the type system. We are trading compile-time guarantees for runtime+test guarantees, and we must invest in the latter deliberately.
- **The WASM hardening track (ADR-0008).** Go-as-WASM-guest in the component model is less mature than Rust's. This is part of *why* ADR-0008 picks gRPC-subprocess as the primary path and treats WASM as a later hardening track — this ADR makes that ordering harder to invert.
- **Numeric / on-device ML (ADR-0007).** Go's ML ecosystem is thin. On-device embeddings for air-gapped mode will likely need a cgo binding (e.g., onnxruntime) or a subprocess — which directly tensions the `CGO_ENABLED=0` default (clause 2). ADR-0007 must resolve this explicitly; this ADR has pre-loaded the constraint it will fight against.
- **In-process sandboxing of untrusted code is impossible.** Go offers no in-process sandbox, so untrusted community code *must* run out-of-process. This is not regret — it is the direction ADR-0008 chose anyway — but it forecloses an in-process plugin option permanently.
- **No manual memory control on hot paths.** GC is irrelevant at today's workload (`ARCHITECTURE.md §5.4`), but a future massive-repo indexing or embedding hot path could expose GC pressure, and the escape hatch (drop to cgo or a sidecar) reopens the R1 tension above.

### Reversibility

This is the **least reversible** decision in the project; reversing it is a rewrite of the kernel. The decision is rated low-reversibility deliberately, and the mitigation is structural, not optimistic — see Migration Strategy.

---

## Migration Strategy

There is **no existing code to migrate** (greenfield, pre-PR-001), so "migration" here means *exit-cost containment* — the strategy that limits the blast radius **if** Go is ever regretted:

1. **Keep the domain layer pure and exhaustively tested.** A pure domain with a thick specification suite is the most portable asset; it is the part a rewrite could re-derive fastest. (This also pays the "harder invariants" cost above.)
2. **Hold the extension and provider boundaries language-neutral (R2 / ADR-0008).** Because plugins, providers, and (eventually) ML-heavy extractors talk to the kernel over a wire protocol, a specialized component can already be written in another language *without* touching the kernel. This is the pressure-release valve: the parts most likely to *want* a different language are the parts already designed to allow one.
3. **Confine cgo, if ever admitted, behind build tags (clause 2).** This keeps any non-Go dependency quarantined and the default build pure, so a problematic native dependency never silently becomes load-bearing.

The honest summary: we cannot make this decision cheaply reversible, so we instead make the *components most likely to need a different language* reachable without reversing it.

---

## Future ADR Dependencies

- **ADR-0002 (Persistence):** **inherits a hard constraint** — the storage driver must be pure-Go to satisfy clause 2 / R1. This ADR removes "use a cgo SQLite driver" from ADR-0002's option set. **Correction (2026-07-20, per [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md), Accepted):** ADR-0002 selected no database at all — flat, versionable JSON files (`record.FileStore`), per [principles.md](../00-overview/principles.md)'s Filesystem-first persistence principle, which sits at a higher precedence tier than this ADR. The clause above describing "a mature pure-Go SQLite implementation... embedded" was this ADR's own anticipation, written before that principle settled the question the other way; it never became the shipped design and should not be read as though it had.
- **ADR-0005 (Executor contract & ports):** ports are expressed as Go interfaces *in-process*; this ADR fixes that substrate. ADR-0005 inherits Go's interface semantics (structural, no default methods) as its modeling vocabulary.
- **ADR-0007 (Knowledge & semantic store):** **must explicitly resolve** the on-device-embedding vs `CGO_ENABLED=0` tension this ADR creates.
- **ADR-0008 (Extension contract):** **discharges R2** — it must define the language-neutral boundary that clause 4 requires; it is also where the WASM-track difficulty noted above is formally accepted.
- **Possible new ADR (Dependency & supply-chain policy):** vendoring, module-proxy, and dependency-vetting policy is implied by choosing Go's module ecosystem but is not decided here; flag for later if the dependency surface grows.

---

## Open Questions

1. **CGO boundary.** Clause 2 permits cgo behind a build tag for optional features. *Is the default-distribution build pure-Go-only forever, or do we ship an optional "full" build with cgo features (e.g., faster SQLite, on-device embeddings)?* ADR-0007 will force this; resolve it there or in a dedicated build-policy ADR. The risk of leaving it open: feature creep quietly normalizes cgo and erodes R1.
2. **Toolchain support window.** "Latest two minor releases" is proposed; ratify it, and decide the upgrade cadence (who bumps, how often, what breaks a release).
3. **Module/import path.** Blocked on the project name (RFC Open Question 7 — "Foundry" is provisional and collides with adjacent tools). A provisional path unblocks PR-001; the canonical path should be set before the first *external* importer exists (i.e., before the M6 public SDK), after which it is expensive to change.
4. **Generics & style conventions.** Whether/how to use generics, and the error-handling idiom, are style-guide matters, not architecture. Out of scope for this ADR unless a convention turns out to have cross-cutting architectural weight.

---

## Review Checklist

For the board (and future re-reviewers) to validate this ADR remains sound:

- [ ] **No contradiction with accepted documents.** Confirmed at authoring: RFC-0001 is mechanism-free and cannot conflict; ARCH §5.4 is ratified, not contradicted; the roadmap assumes this outcome. Re-confirm on any future amendment.
- [ ] **R1 protected by the toolchain, not just hoped for.** Is `CGO_ENABLED=0` enforced on the default build in CI? Does any merged dependency break the static-binary guarantee?
- [ ] **R2 obligation is live.** Does any change require a plugin author to write Go? If yes, it violates clause 4 — block it.
- [ ] **Alternatives remain realistic.** If a rejected alternative's blocking weakness has materially changed (e.g., a Go WASM-component story matures, or a Node single-binary story becomes real), open an amendment — do not let the rejection ossify on stale facts.
- [ ] **The "Harder" list is honored downstream.** Does ADR-0004 invest in validation/tests to compensate for weak compile-time invariants? Does ADR-0007 actually resolve the cgo/embedding tension rather than inheriting it silently?
- [ ] **Reversibility mitigations are real.** Is the domain layer pure and well-specified? Is the extension boundary genuinely language-neutral?
- [ ] **Process caveat tracked.** Is the interim-authority acceptance reconciled once the governance RFC lands?

---

_This ADR records the single most expensive-to-reverse decision in Foundry. It chooses a wider contributor door and a simpler distribution model over maximal type-system expressiveness, and it pays for that choice in the domain layer and the ML path — knowingly. Amend it only with the same rigor, and only when a rejected alternative's blocking weakness has actually changed._
