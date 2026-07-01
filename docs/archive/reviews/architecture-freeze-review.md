# Architecture Freeze Review — Foundry

| | |
|---|---|
| **Question** | Is the architecture mature enough to **permanently freeze** before implementation? |
| **Authority order** | 1. RFC-0001 · 2. `ARCHITECTURE.md` · 3. `IMPLEMENTATION-ROADMAP.md` · 4. ADR Gate Review · 5. ADR-0001 |
| **Stance** | Maximally critical. A false freeze is far more expensive than delay. Prior reviews are not binding here. |
| **Date** | 2026-06-29 |
| **Outcome** | **REQUIRES NEW ADRS** (and the *premise* of a permanent pre-implementation freeze is itself rejected — see Headline). |

---

## Headline finding — the freeze premise contradicts the documents being frozen

- **Severity: P0** · **Category: Governance**

**Evidence.**
- `ARCHITECTURE.md` header: *"**Status:** Draft v0 (foundational blueprint)"* and *"**Nature:** Living document. Sections marked _(decision pending)_ require ratification via ADR before implementation."*
- RFC-0001 header: *"**Status** | Draft — Proposed (seeking ratification)"*; RFC §6.7 commits the project to *"decide late when cheap."*
- `reviews/RFC-0001-review-round-1.md`: *"**MAJOR REVISION REQUIRED**."*

**Why this matters.** You are asked to *permanently freeze* a document set whose root (RFC-0001) is unratified and under major revision, and whose architecture self-identifies as *"Draft v0 … Living document"* with *"decision pending"* sections explicitly deferred to future ADRs. Freezing now does one of two harmful things: it either (a) freezes decisions the documents themselves say should be made later — violating the project's own "decide late when cheap" principle and guaranteeing premature lock-in — or (b) produces a *false* freeze that is violated by the first ADR that fills a deferred section. An engineer arriving in three years inherits either rigidity or a freeze that was never real. **A permanent total freeze is the wrong instrument.** The defensible action is a *scoped* freeze of the durable kernel invariants once their owning ADRs exist, with the edges explicitly left thawed — which is exactly what RFC §6.7 and ARCH's "living document" status already prescribe.

This finding alone precludes APPROVED. The remaining findings establish what must exist before even a *scoped* freeze is safe.

---

## Phase 1 — Cross-document consistency

### P1-1 — Terminology conflict: "Workflow" (ARCH) vs "Pipeline" (Roadmap)
- **Severity: P1** · **Category: Contradiction / Conflicting terminology**
- **Evidence.** ARCH §3.1: *"these terms must mean exactly this everywhere in code, docs, and CLI"*; the term is **Workflow** — *"WorkflowDefinition"*, *"WorkflowRun ──executes──▶ StageRuns"*. ARCH Appendix B: *"Changing a term is a documentation event, not a casual rename."* The Roadmap uses **Pipeline** pervasively (45 occurrences vs 5 of "workflow"): *"PR-005 — Pipeline domain model"*, *"`foundry run <pipeline>`"*, *"Pipeline runtime"*.
- **Why this matters.** The authoritative ubiquitous language (ARCH, authority #2) names the core aggregate **Workflow**; the Roadmap (#3) renames it **Pipeline** for the domain type, the CLI verb, and the schema. ARCH explicitly forbids casual renames. Frozen, this guarantees a codebase where half the surface says `Workflow` and half says `Pipeline` — the exact ubiquitous-language fracture DDD exists to prevent. Three years in, this is a pervasive, expensive rename.
- **Minimal amendment.** Pick one term in one place. Per authority order, "Workflow" wins unless ARCH is amended to adopt "Pipeline" as a deliberate, documented rename. Do not resolve which here; record that the two documents must not ship divergent core nouns.

### P1-2 — An execution abstraction ("Executor") exists in the Roadmap but not in the architecture's domain model
- **Severity: P1** · **Category: Architecture Drift / Documentation Ownership**
- **Evidence.** ARCH domain model (§3.2): *"WorkflowDefinition ──composed of──▶ Stages ──invoke──▶ Skills"* and *"SkillInvocation ──calls──▶ Provider (via Router)"* — the execution path is **Stage → Skill → Provider**; there is **no `Executor` port** in ARCH (the word appears once, lowercase, as *"the kernel's generic skill executor"*, not a port). The Roadmap makes **Executor** a first-class port: *"E5 — Executors … Executor port; shell → mock → real providers"*, *"the **Executor port** (PR-011/019) is where shell → mock → real providers all plug in"*; in M1 a *stage runs an executor directly* (`op: shell`) **before skills exist** (skills are M2).
- **Why this matters.** Two different execution models are recorded in two accepted documents: ARCH's `Stage→Skill→Provider` versus the Roadmap's `Stage→Executor` (with Provider as one executor among shell/mock). The Gate Review even mints "ADR-0005 — Executor … Contract", treating Executor as central — yet the authoritative domain model (ARCH §3, the ubiquitous language) does not contain the concept. This is an architectural decision made in the Roadmap that reshapes the domain model. Freezing with two execution models is a guaranteed redesign once the layers are reconciled in code.
- **Minimal amendment.** The "Executor" abstraction must be either admitted into ARCH §3's domain model (with its relationship to Skill and Provider defined) or removed from the Roadmap/Gate-Review in favor of ARCH's Skill/Provider/op vocabulary. One model, one home. Do not design the reconciliation here.

### P1-3 — ADR numbering collision across accepted documents
- **Severity: P1** · **Category: Contradiction / Documentation Ownership**
- **Evidence.** Roadmap PR-007: *"ratify in **ADR-0004**"* (= ledger); Roadmap tech-debt: *"WASM is a hardening step **(ADR-0002)**"* (= plugin isolation). Gate Review: *"**ADR-0002** — State Persistence"*, *"**ADR-0004** — Declarative Manifest Schema"*, *"**ADR-0008** — … Plugin Isolation"*.
- **Why this matters.** "ADR-0002" and "ADR-0004" each denote two unrelated decisions across the set; any frozen cross-reference is ambiguous. (Confirmed in `reviews/architecture-consistency-review.md` F1; re-affirmed — not waived.)
- **Minimal amendment.** Declare one numbering authoritative (the Gate Review's consolidation supersedes ARCH Appendix A) and mark the two stale Roadmap references.

### P1-4 — `ARCHITECTURE.md §2.2` fixes a determinism guarantee owned by (the unwritten) ADR-0003, and overstates it
- **Severity: P1** · **Category: Architecture Drift**
- **Evidence.** ARCH §2.2: *"**validators and gates are deterministic functions of artifacts. The same diff always yields the same pass/fail verdict.**"* Roadmap PR-011 marks the shell executor *"`deterministic: false`"*; validators (PR-012) wrap external tools. Gate Review lists the *"Replay & Determinism Contract"* as ADR-0003, *not yet written*.
- **Why this matters.** The platform's flagship guarantee is asserted, normatively, in the architecture, while the contract that owns it (ADR-0003) does not exist, and the assertion is stronger than shell-executed validators can honor. (Confirmed `architecture-consistency-review.md` F3; re-affirmed.)
- **Minimal amendment.** Determinism/replay contract is owned solely by ADR-0003; §2.2's claim is descriptive pending that ADR.

### P1-5 — Plugin isolation mechanism: ARCH says WASM-preferred; Roadmap and ADR-0001 say gRPC-first
- **Severity: P1** · **Category: Contradiction / Premature Decision**
- **Evidence.** ARCH §15.2: *"sandbox (**WASM component model preferred**; subprocess … as fallback)"*. Roadmap: *"subprocess/gRPC isolation first"*. ADR-0001: *"ADR-0008 picks **gRPC-subprocess as the primary path**"*.
- **Why this matters.** A not-yet-made plugin decision (ADR-0008) is pre-decided in the least-reversible document, against the higher-authority architecture. (Confirmed `architecture-consistency-review.md` F2 and `ADR-0001-change-list.md` Change 1; re-affirmed.)
- **Minimal amendment.** Defer mechanism to ADR-0008; ADR-0001/Roadmap state only the language-agnostic obligation, or ARCH §15.2 is marked open.

---

## Phase 2 — Decision ownership

| Decision | Appears in | Correct owner | Problem |
|---|---|---|---|
| Language = Go | ARCH §5.4 (recommend) + ADR-0001 (decide) | ADR-0001 | Clean — ARCH self-defers correctly |
| Manifest representation (YAML/JSON, no DSL) | ARCH §7.1/§10.1 (**"Decision:"**) + Gate ADR-0004 | ADR-0004 | **Duplicated** (P2-1) |
| Determinism/replay guarantee | ARCH §2.2 (normative) + Gate ADR-0003 | ADR-0003 | **Wrong doc** (P1-4) |
| Plugin isolation mechanism | ARCH §15.2 + Roadmap + ADR-0001 + Gate ADR-0008 | ADR-0008 | **Pre-decided in 3 docs** (P1-5) |
| Executor abstraction | Roadmap + Gate ADR-0005 | ARCH domain model | **Missing from owner** (P1-2) |
| Core aggregate name | ARCH (Workflow) vs Roadmap (Pipeline) | ARCH §3 | **Conflicting** (P1-1) |
| Registry/distribution | ARCH §15.4 (decides) + ARCH Appendix A (lists as open ADR) | ARCH §15.4 | Internal dup (P3) |

### P2-1 — Manifest representation decided twice
- **Severity: P2** · **Category: Documentation Ownership.** ARCH §7.1: *"**Decision:** Workflow definitions are declarative documents (YAML authored, JSON canonical)"*; §10.1: *"**Decision:** A Skill is a declarative manifest … Never a bespoke DSL."* Gate ADR-0004 re-ratifies the same representation. **Minimal amendment:** ADR-0004 references ARCH for representation and owns only the *evolution/versioning policy*.

---

## Phase 3 — Dependency graph

```
RFC-0001 (UNRATIFIED, major revision)
   └─▶ ARCHITECTURE.md (Draft v0, living)
         ├─▶ Roadmap ──▶ ADR-0001 (Accepted, interim authority)
         └─▶ Gate Review ──▶ {ADR-0002 … ADR-0008}  (ALL UNWRITTEN except 0001)

ADR-0001 ─constrains─▶ ADR-0002 (pure-Go driver)
ADR-0002 ─feeds─▶ ADR-0003 (reads ledger) ─feeds─▶ ADR-0005 (determinism flag)
ADR-0005 ◀──capability hooks──▶ ADR-0006   ← CYCLE (P2-2)
ADR-0002 ─reused by─▶ ADR-0007
ADR-0005/0006 ─stabilize ports for─▶ ADR-0008
```

### P3-1 — Foundational compatibility-surface ADRs are unwritten — freeze is impossible
- **Severity: P0** · **Category: Missing Decision.** Gate Review identifies ADR-0002 (persistence/hash/layout), ADR-0003 (replay), ADR-0004 (manifest schema) as owning permanent compatibility surfaces, all rated P0, **none written**. *Why this matters:* you cannot freeze a compatibility surface whose owning decision does not exist. *Minimal amendment:* write and ratify ADR-0002, ADR-0003, ADR-0004 before any freeze (they are also the M0→M1 critical-path blockers).

### P2-2 — Circular dependency ADR-0005 ↔ ADR-0006
- **Severity: P2** · **Category: Dependency.** Gate ADR-0005 includes *"required-capability hooks (link to ADR-0006)"* while ADR-0006 builds on the executor contract. (Confirmed `architecture-consistency-review.md` F7.) *Minimal amendment:* remove the capability back-reference from ADR-0005's scope.

### P3-2 — "decide late when cheap" is honored for the far ADRs
No premature-decision violation found in ADR sequencing itself: ADR-0007 (M4), ADR-0008 (M6) are correctly deferred. The premature decisions are the *content leaks* (P1-4, P1-5), not the ADR ordering.

---

## Phase 4 — Compatibility surfaces and their owners

| Surface | Owner | Status |
|---|---|---|
| Manifest schema | ADR-0004 | Unwritten + dup representation (P2-1) |
| Plugin/wire protocol | ADR-0008 | Unwritten + contradicted (P1-5) |
| On-disk layout (`.foundry/`) | ADR-0002 | Unwritten + **audit contradiction (P1-6)** |
| Hashing algorithm | ADR-0002 | Unwritten |
| Replay contract | ADR-0003 | Unwritten + **cross-version gap (P1-7)** |
| **CLI interface** | **NONE** | **Unowned (P1-8)** |
| **Daemon / local API** | **NONE** | **Unowned (P2-3)** |
| Capability descriptor | ADR-0006 | Unwritten |
| Module path | ADR-0001 (deferred to name, RFC OQ7) | Blocked on name |
| Port version negotiation | ADR-0008 / ARCH §15.3 | Unwritten |
| Ledger event schema | ADR-0002 | Unwritten |
| **Cassette request-hash canonicalization** | **NONE** | **Unowned (P2-4)** |
| **Authored-knowledge format + migration** | **NONE** | **Unowned (P1-9)** |

### P1-6 — The run ledger is classified as a "rebuildable cache" yet is the immutable compliance audit log
- **Severity: P1** · **Category: Contradiction / Compatibility Risk**
- **Evidence.** Gate ADR-0002: *"the derived index + **run store are gitignored (rebuildable cache)**."* ARCH §17: *"**Audit log = the run ledger** … recorded **immutably**. **Exportable for compliance**."* ARCH §7.4 makes the ledger the spine of audit/replay/resume.
- **Why this matters.** A rebuildable, gitignored cache is by definition disposable and recomputable; the audit ledger is by definition durable and *not* recomputable. The storage classification (a permanent layout compatibility surface) directly contradicts the ledger's role and RFC value V3 ("every consequential action is auditable"). Frozen, this means the compliance audit trail lives in a gitignored single-file cache that `git clean` or a laptop reimage destroys — discovered exactly when an auditor asks for it, three years in.
- **Minimal amendment.** ADR-0002 must classify the run ledger as durable (separate from the rebuildable derived cache) and state its retention, or ARCH §17 must downgrade the audit/compliance claim. Do not resolve which.

### P1-7 — Replay across Foundry versions is undefined
- **Severity: P1** · **Category: Missing Decision / Compatibility Risk**
- **Evidence.** ARCH §2.2: *"A run can be re-executed from its ledger with cached responses, **yielding identical artifacts**"* — unscoped by version. Roadmap PR-008 verifies replay within one build. Gate ADR-0003 defines the contract but is silent on cross-version replay.
- **Why this matters.** Replay is the flagship trust/audit property. Deterministic stage *implementations* change across releases; replaying a 3-year-old run with a new binary may not reproduce identical artifacts. Whether replay is guaranteed only within a version or across versions is undefined — and the 3-years-later auditor is the literal use case. An unscoped guarantee that cannot hold is worse than a scoped one.
- **Minimal amendment.** ADR-0003 must scope replay's version-compatibility guarantee; ARCH §2.2's unbounded "identical artifacts" claim is reconciled to that scope.

### P1-8 — The CLI interface has no owning decision
- **Severity: P1** · **Category: Missing Decision / Compatibility Risk**
- **Evidence.** ARCH §16 presents the CLI as *"illustrative"*. The Roadmap defines commands/flags/`--json` output per-PR. No document owns CLI stability or a versioning/deprecation policy.
- **Why this matters.** The CLI (commands, flags, exit codes, `--json` shape) is a primary compatibility surface: users script it and CI invokes it (`ARCHITECTURE.md §14.2` "same binary in CI"). Without an owner and a stability policy, every command change is a silent breaking change for downstream automation. This is a classic post-release redesign trigger.
- **Minimal amendment.** Assign CLI/output-contract stability to an owning decision (a new ADR, or an explicit section of an existing one) before the CLI is treated as frozen. Do not design the policy here.

### P1-9 — Authored-knowledge format stability and migration is unowned — the RFC's flagship value has no compatibility owner
- **Severity: P1** · **Category: Missing Decision / Compatibility Risk**
- **Evidence.** RFC V4: *"The organization owns its knowledge and process, and they are **portable** … can leave **with** you."* RFC §6.3 makes knowledge the durable capital. Gate ADR-0007 owns the *store* (derived) but no document owns the *authored* knowledge's on-disk format stability or its migration across Foundry versions. ADR-0001 review already flagged "durable-asset migration across Foundry's own versions" as a missing abstraction.
- **Why this matters.** The RFC's central promise is durable, portable knowledge. If Foundry cannot read its own *authored* knowledge (ADRs, conventions, decisions in `.foundry/`) across its own version upgrades, V4 is violated and the differentiator collapses — the highest-stakes compatibility surface in the project has no owner.
- **Minimal amendment.** Assign authored-knowledge format-stability + cross-version migration to an owning ADR (extend ADR-0007 or ADR-0003-knowledge scope). Do not design the format here.

### P2-3 / P2-4
- **P2-3 (Daemon/local API unowned).** ARCH §5.2 posits a daemon/local API for editor integration (M5); no stability owner. *Minimal amendment:* assign before M5.
- **P2-4 (Cassette request-hash unowned).** Roadmap PR-019: outputs *"keyed by request hash"*; no document owns the request canonicalization (a cassette-invalidation compat surface). *Minimal amendment:* assign to ADR-0003 or ADR-0005.

---

## Phase 5 — Architecture stress test (does the doc already define behavior?)

| Scenario | Behavior defined? | Finding |
|---|---|---|
| Deterministic replay after several releases | **No** | **P1-7** (cross-version replay undefined) |
| Corrupted local state | Partial — derived is rebuildable (ARCH §5.3); **ledger recovery undefined** | **P2-5** |
| Thousands of concurrent executions | **No** | **P2-6** (concurrency model + SQLite write contention) |
| Millions of repositories / distributed exec | Deferred (post-1.0); *"remote is additive"* claim **unproven** | **P2-7** |
| Repository migration | Authored knowledge travels in git; **run ledger (gitignored) does not** | folds into P1-6 |
| Partial failures / rollback | **Yes** — saga + compensating actions (ARCH §7.6) | clean |
| Plugin crash / version skew | Conceptually (ARCH §15.2/§15.3); ADR-0008 unwritten | tracked by P3-1 |
| Offline execution | Conceptually (ARCH §17); cgo/embedding tension | P2 (F5, prior) |
| Replacing SQLite / Go / engine | Ledger seam reserved; Go least-reversible (ADR-0001) | acceptable |

### P2-5 — Run-ledger corruption/backup recovery undefined
- **Severity: P2** · **Category: Missing Decision.** The ledger is the audit source of truth (ARCH §7.4/§17) but stored as a local, gitignored single SQLite file (per P1-6). No backup/recovery/integrity-check behavior is defined. *Minimal amendment:* ADR-0002 states ledger durability/recovery, coupled to P1-6's resolution.

### P2-6 — Concurrency model undefined; embedded SQLite may conflict with the "thousands of pipelines" future
- **Severity: P2** · **Category: Missing Decision / Dependency.** The Roadmap defers parallelism (*"runtime is sequential in M0–M1"*) and claims it is *"an orchestrator-internal change behind a stable interface."* Gate ADR-0002 selects embedded SQLite. Concurrent runs writing one SQLite ledger hit write-serialization limits; no coordination model is defined, and the roadmap's "think about the future: thousands of pipelines" is asserted without a concurrency design. *Why this matters:* the storage decision (ADR-0002, a permanent surface) is being made without the concurrency model it must serve — a redesign risk if the "internal change" assumption proves false. *Minimal amendment:* ADR-0002 must acknowledge the concurrency/contention assumption it depends on; do not design it here.

### P2-7 — "Remote is additive, not a redesign" is unproven
- **Severity: P2** · **Category: Dependency.** ARCH §5.3: *"A server/SaaS is a later deployment of the same kernel, not a different architecture."* Only the ledger remote-seam is reserved (Gate ADR-0002); no other seam (knowledge, context, run coordination) is shown to be remoteable. The claim is plausible but undemonstrated. *Minimal amendment:* none required for M0; record as an unproven assumption, not a frozen guarantee.

---

## Findings summary

| ID | Sev | Category | One-line |
|---|---|---|---|
| Headline | **P0** | Governance | Freezing a Draft-v0/living architecture atop an unratified RFC is incoherent |
| P3-1 | **P0** | Missing Decision | Foundational compat-surface ADRs (0002/0003/0004) are unwritten |
| P1-1 | P1 | Contradiction | Workflow (ARCH) vs Pipeline (Roadmap) |
| P1-2 | P1 | Arch Drift | Executor abstraction absent from ARCH domain model |
| P1-3 | P1 | Contradiction | ADR numbering collision |
| P1-4 | P1 | Arch Drift | §2.2 determinism guarantee in wrong doc + overstated |
| P1-5 | P1 | Premature | WASM-preferred vs gRPC-first |
| P1-6 | P1 | Contradiction | Run ledger = "rebuildable cache" vs immutable audit log |
| P1-7 | P1 | Compat Risk | Cross-version replay undefined |
| P1-8 | P1 | Missing Decision | CLI interface unowned |
| P1-9 | P1 | Compat Risk | Authored-knowledge format/migration unowned (RFC V4) |
| P2-1 | P2 | Doc Ownership | Manifest representation decided twice |
| P2-2 | P2 | Dependency | ADR-0005 ↔ 0006 cycle |
| P2-3 | P2 | Compat Risk | Daemon/local API unowned |
| P2-4 | P2 | Compat Risk | Cassette request-hash unowned |
| P2-5 | P2 | Missing Decision | Ledger corruption/backup undefined |
| P2-6 | P2 | Dependency | Concurrency model vs SQLite contention undefined |
| P2-7 | P2 | Dependency | "Remote is additive" unproven |
| P3 | P3 | Drift | Registry decided §15.4 yet listed open in Appendix A |

**Two P0, nine P1.**

---

## Phase 6 — Freeze decision

### REQUIRES NEW ADRS

The architecture is **not ready to freeze**, and a *permanent total* freeze is the wrong goal regardless (Headline). Concretely:

**Cannot freeze until these are written and ratified** (they own permanent compatibility surfaces that have no ratified owner today):
- **ADR-0002** — persistence, content-addressing, on-disk layout — **amended** to resolve P1-6 (ledger is durable, not rebuildable cache) and acknowledge P2-6 (concurrency assumption).
- **ADR-0003** — replay & determinism contract — **scoped** to resolve P1-4 (own §2.2's guarantee), P1-7 (cross-version replay), and own P2-4 (cassette key).
- **ADR-0004** — manifest schema & evolution — referencing ARCH for representation (P2-1).
- **A new owner for the CLI/output contract (P1-8)** — new ADR or explicit section; the CLI is a frozen surface with no owner.
- **A new owner for authored-knowledge format stability + migration (P1-9)** — extend ADR-0007/knowledge scope; this guards the RFC's flagship value V4.

**Required amendments before any freeze** (no new ADR, but blocking): P1-1 (one core noun), P1-2 (one execution model), P1-3 (one ADR numbering), P1-5 (defer isolation mechanism to ADR-0008), and ratify a minimal RFC-0001 (P0-1 ordering + P0-2 governance) so the root is no longer in major revision (Headline).

**What is safe regardless:** **M0 implementation (PR-001→PR-006) may proceed now, under provisional (non-frozen) status.** It is insulated from every finding — it touches only the language decision, the (to-be-renamed) workflow model, and deterministic execution before replay/audit/CLI-stability/plugins exist.

**Recommended freeze posture (not a redesign — a scoping of the freeze itself):** freeze only the **durable kernel invariants** once ADR-0002/0003/0004 exist — the ports-and-adapters shape, the event-sourced durable ledger, deterministic-first verification, and content-addressing — and leave the edges (providers, validators, knowledge, extensions, CLI surface) explicitly thawed under semver, exactly as RFC §6.7 and ARCH's "living document" status already require. A total freeze contradicts the project's own philosophy; a core freeze expresses it.

**Why not the other outcomes:**
- *APPROVED* — impossible: two P0s; the root RFC is unratified; the surfaces to be frozen have no owning ADRs.
- *APPROVED WITH REQUIRED AMENDMENTS* — insufficient: the gaps are not all amendments; several compatibility surfaces (replay, CLI, authored-knowledge, manifest) require **new ratified decisions**, not edits.
- *DO NOT START IMPLEMENTATION* — too strong: M0 is demonstrably insulated and can begin in parallel; halting it would burn the cheapest, safest work for no risk reduction.

---

_The architecture is close on its durable core and genuinely strong in philosophy — but it is not frozen-ready, and it should not be frozen whole. Write the three foundational ADRs (with the P1-6/P1-7 amendments), assign the two unowned compatibility surfaces (CLI, authored-knowledge), reconcile the five P1 contradictions, ratify the RFC root — then freeze the kernel invariants only. Begin M0 today; freeze nothing yet._
