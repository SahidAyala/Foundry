# Chief Architecture Review Board — Full Consistency Review

| | |
|---|---|
| **Scope** | Cross-document architectural consistency across the accepted set |
| **Authority order** | 1. RFC-0001 · 2. `ARCHITECTURE.md` · 3. `IMPLEMENTATION-ROADMAP.md` · 4. ADR Gate Review · 5. ADR-0001 |
| **Question** | Can implementation begin without architectural debt? |
| **Date** | 2026-06-29 |
| **Determination** | **CONDITIONAL GO.** M0 + early-M1 may begin now; the P1 findings are architectural debt that must be cleared before their respective milestones, and two (F1, F4) must be cleared immediately. |

> Method: every finding cites exact passages. No rewrites, no new architecture, no tradeoff resolution — only consistency verification and the minimum change that removes the inconsistency. Where a category is clean, it is stated explicitly (§"Categories verified clean").

---

## F1 — Two accepted documents assign the same ADR numbers to different decisions

- **Severity: P1**
- **Category: Contradiction / Duplicated source of truth**

**Evidence.**
- Roadmap (PR-007): *"append-only event store (embedded — SQLite or append-only file, **ratify in ADR-0004**)"* → ADR-0004 = **ledger**.
- Roadmap (Tech-debt table): *"WASM is a hardening step **(ADR-0002)**"* → ADR-0002 = **plugin isolation**.
- Gate Review headings: *"**ADR-0002** — State Persistence, Content-Addressing & On-Disk Layout"*; *"**ADR-0004** — Declarative Manifest Schema & Evolution Policy"*; *"**ADR-0008** — Extension Contract: Plugin Isolation"*.
- Gate Review mapping table: *"ADR-0002 Persistence … = Appendix A **ADR-0004 (ledger)**"*.

**Why they conflict.** Across the accepted set, **"ADR-0002" means *plugin isolation*** (Roadmap, ARCH Appendix A) **and *persistence*** (Gate Review); **"ADR-0004" means *ledger*** (Roadmap, ARCH Appendix A) **and *manifest schema*** (Gate Review). Any future document that cites "ADR-0004" is now ambiguous between two unrelated decisions. This is not cosmetic: the Gate Review's own dependency graph and the Roadmap's PR dependencies will mis-link the moment anyone follows a number.

**Minimum change.** Declare **one** numbering authoritative (the Gate Review's is the consolidation and supersedes ARCH Appendix A) and record that the two inline Roadmap references (PR-007 "ADR-0004"; tech-debt "ADR-0002") are stale — cite the ledger/isolation ADRs by *name* or canonical number. No content changes; one authority declaration plus two pointer corrections.

---

## F2 — Plugin isolation mechanism: higher-authority doc says WASM-preferred; two lower docs say gRPC-first

- **Severity: P1**
- **Category: Contradiction / Premature decision / ADR deciding things owned elsewhere**

**Evidence.**
- `ARCHITECTURE.md §15.2`: *"Community / untrusted — run in a **sandbox (WASM component model preferred; subprocess with seccomp/namespacing as fallback)**"*.
- Roadmap (tech-debt): *"M6; **subprocess/gRPC isolation first**; … WASM is a hardening step"*.
- ADR-0001 (Harder): *"this is part of why **ADR-0008 picks gRPC-subprocess as the primary path** and treats WASM as a later hardening track"*; (clause 4): *"the plugin/provider contract is defined in a language-neutral wire format **(gRPC/protobuf)**, decided in ADR-0008."*

**Why they conflict.** `ARCHITECTURE.md` (authority #2) states WASM as the *preferred* mechanism. The Roadmap (#3) and ADR-0001 (#5) reverse this to gRPC-first **without amending §15.2**. Worse, ADR-0001 — the project's *least-reversible* document — reasons *from* the reversed position and names the wire format (gRPC/protobuf), foreclosing the WASM component model that §15.2 prefers and that ADR-0008 is supposed to own. Per the authority rule, §15.2 currently wins, so the lower docs are in contradiction with it.

**Minimum change.** The mechanism is owned by ADR-0008; until it is taken, one home must hold it. Either (a) mark `ARCHITECTURE.md §15.2`'s "preferred" as superseded-pending-ADR-0008 (it already defers to "ADR-0002/isolation" in Appendix A), **or** (b) ADR-0001 and the Roadmap stop asserting gRPC-first and state only the language-agnostic *obligation*. Recommended minimum (per `ADR-0001-change-list.md` Change 1): ADR-0001 defers; reconcile §15.2 at its source *before* ADR-0008 begins. Do not decide the mechanism here.

---

## F3 — `ARCHITECTURE.md §2.2` fixes a determinism guarantee that (a) ADR-0003 owns and (b) is too strong for shell-executed validators

- **Severity: P1**
- **Category: Architecture drift / Responsibilities assigned to the wrong document**

**Evidence.**
- `ARCHITECTURE.md §2.2`: *"**Reproducible verification — validators and gates are deterministic functions of artifacts. The same diff always yields the same pass/fail verdict.**"*
- Roadmap (PR-011): shell executor *"marked `deterministic: false` so replay … excludes it from identity guarantees"*. Validators (PR-012) wrap external tools (*"exitcode (wrap any command…)"*).
- Gate Review (ADR-0003): the *"Replay & Determinism Contract"* is listed as a **NEW** decision the board added, owning *"a stage is classified deterministic or non-deterministic"* and *"ports/Executor (determinism flag)"*.

**Why they conflict.** §2.2 asserts as an accepted guarantee that validators are *deterministic functions of artifacts*. But the Roadmap models validators as executions of external tools (tsc, eslint, test runners) through an executor explicitly flagged **non-deterministic**. Real external tools are not pure functions of the artifact — they depend on tool version, environment, and execution ordering. So §2.2 has (1) prematurely become the **normative** determinism contract that the Gate Review assigns to the not-yet-written ADR-0003, and (2) made a claim stronger than the implementation can honor. This is the core trust property of the platform sitting in two places with two different strengths.

**Minimum change.** Assign the determinism/replay contract solely to ADR-0003, and mark §2.2's "validators and gates are deterministic functions of artifacts" as **descriptive, pending ADR-0003 reconciliation** — ADR-0003 must classify whether validator execution is re-executed or cassette-replayed. Do not resolve the classification in this review.

---

## F4 — The entire stack treats RFC-0001 as a ratified constitution; RFC-0001 is unratified and under MAJOR REVISION

- **Severity: P1**
- **Category: Governance issue**

**Evidence.**
- RFC-0001 header: *"**Status** | Draft — Proposed (seeking ratification)"*.
- `reviews/RFC-0001-review-round-1.md`: *"Verdict … **MAJOR REVISION REQUIRED**"*, with five P0s including **P0-1 (no principle ordering)**, **P0-2 (no governance/ratification process)**, **P0-3 (cost as a first-class constraint, currently absent)**.
- `ARCHITECTURE.md` header: *"must conform to this RFC"*; Gate Review and ADR-0001 repeatedly justify decisions by RFC section ("serves §6.3", "RFC §6.5").
- ADR-0001 header: *"Accepted under interim board authority. A governance/RFC-process model does not yet exist."*

**Why they conflict.** Documents 2–5 derive authority from RFC-0001 and cite its principles to justify decisions, while RFC-0001's own board review demands major revision and states that **no process exists to ratify anything** (P0-2). Two open RFC P0s have downstream reach: P0-1 (principle ordering) governs the §13 rubric every ADR invokes; P0-3 (cost constraint) would introduce a new evaluation criterion that none of the current ADRs applied. The foundation can shift under the ADRs built on it.

**Minimum change.** Do not block M0 (it is insulated from the open RFC P0s — see §Determination). But **gate ADR *finalization* (not drafting) on RFC-0001 ratification**, and either ratify a minimal RFC-0001 resolving at least P0-1 and P0-2 first, or explicitly record that documents 2–5 are *provisional* pending RFC ratification. The Gate Review already recommended "governance-first"; this finding makes that sequencing a consistency requirement, not a preference.

---

## F5 — `ARCHITECTURE.md §5.3` assumes a (C) vector extension; ADR-0001 mandates `CGO_ENABLED=0` by default

- **Severity: P2**
- **Category: Contradiction / Hidden coupling**

**Evidence.**
- `ARCHITECTURE.md §5.3`: *"Derived index (local cache) — **SQLite + a vector extension for embeddings** and the derived knowledge graph."*
- ADR-0001 (clause 2): *"`CGO_ENABLED=0` is the default build mode. Pure-Go dependencies are required for anything on the default build path."*

**Why they conflict.** The leading SQLite vector extensions are C and require cgo. §5.3 places the vector extension in the *derived index* — infrastructure for the context engine's semantic tier (a core M4 capability, not an obvious "optional feature"). ADR-0001 permits cgo only behind a build tag for *optional* features, never on the default build. Whether semantic retrieval is "optional" is unresolved, so the two documents are in latent contradiction. (Already captured as Change 2 in `ADR-0001-change-list.md`; this adds the direct `ARCHITECTURE.md` evidence.)

**Minimum change.** ADR-0007 must explicitly classify whether the semantic/vector tier ships on the default (pure-Go) build or an optional cgo build, and §5.3 must be reconciled to that classification. Do not decide it here; record it as ADR-0007's obligation.

---

## F6 — The manifest representation is decided in `ARCHITECTURE.md` and re-ratified in ADR-0004

- **Severity: P2**
- **Category: Duplicated source of truth**

**Evidence.**
- `ARCHITECTURE.md §7.1`: *"**Decision:** Workflow definitions are **declarative documents (YAML authored, JSON canonical)**…"*; §10.1: *"**Decision:** A Skill is a **declarative manifest** … **Never a bespoke DSL.**"*
- Gate Review (ADR-0004): *"ratify the **canonical representation** (YAML authored / JSON canonical … leaned in ARCH §7.1/§10.1) **AND** the schema evolution policy."*

**Why they conflict.** The representation (YAML/JSON, manifest+code, no DSL) is stated as a **Decision** in `ARCHITECTURE.md` *and* slated for ratification in ADR-0004. Two records of one decision; if they ever diverge, the authority rule silently prefers `ARCHITECTURE.md` over the ADR that is meant to be the decision of record.

**Minimum change.** Give the representation one home: ADR-0004 should *reference* `ARCHITECTURE.md §7.1/§10.1` for the representation and own only the **evolution/versioning policy** (the genuinely undecided part), or `ARCHITECTURE.md` marks the representation as carried-by-ADR-0004. One pointer, not a re-decision.

---

## F7 — Circular dependency between ADR-0005 (executor contract) and ADR-0006 (capability descriptor)

- **Severity: P2**
- **Category: Hidden coupling / Circular dependency**

**Evidence.**
- Gate Review (ADR-0005): the normalized executor contract includes *"required-capability hooks (link to ADR-0006)"*; ordering places **0005 before 0006**.
- Gate Review (ADR-0006): the capability descriptor + routing *"builds on"* the executor contract; *"PR-021 (provider port + descriptor)"* depends on ADR-0005.

**Why they conflict.** ADR-0005 cannot fully define "required-capability hooks" without the capability vocabulary that ADR-0006 owns, yet ADR-0006 builds on ADR-0005's executor contract. The declared 0005→0006 order is therefore not strictly acyclic.

**Minimum change.** Break the back-reference: ADR-0005 defines the executor contract **without** capability hooks (a minimal placeholder), and ADR-0006 introduces the capability vocabulary and fills the hook — or the capability *descriptor schema* is ratified jointly with 0005. Remove the mutual reference; do not merge the ADRs.

---

## F8 — The cassette request-hash canonicalization is a compatibility surface with no owning decision

- **Severity: P2**
- **Category: Missing decision / Hidden compatibility surface**

**Evidence.**
- Roadmap (PR-019): mock executor returns *"scripted outputs **keyed by request hash**"*; (PR-028) provider *"record/replay (cassettes)"*.
- Gate Review (ADR-0003) covers *"recorded and replayed from a cassette"* but does **not** specify how the request is canonicalized or hashed; ADR-0002 pins content-addressing for *artifacts/bundles/events* only.

**Why it matters.** The request-hash is the cassette key. Its canonicalization rules are a permanent compatibility surface exactly like the artifact hash (ADR-0002 treats that as load-bearing) — if it changes, every recorded cassette and every "no-network CI" replay invalidates. No accepted document assigns ownership of this rule.

**Minimum change.** Assign request canonicalization + hashing to ADR-0003 (replay contract) or ADR-0005 (executor contract). One sentence of scope assignment; no design here.

---

## F9 — `ARCHITECTURE.md` decides the registry in §15.4 but also lists it as an open ADR in Appendix A

- **Severity: P3**
- **Category: Architecture drift (internal)**

**Evidence.** §15.4: *"plugins are git repos with a manifest … not a bespoke package manager on day one"* (stated as the approach); Appendix A: *"ADR-0006 — Registry/distribution format (git+manifest vs. OCI artifacts)"* (listed as needing ratification). The Gate Review de-scoped it (*"pre-decided by ARCH §15.4"*).

**Why it matters.** Minor internal inconsistency: §15.4 effectively decides the day-one approach while Appendix A treats it as open. Already resolved by the Gate Review's de-scoping; recorded for completeness.

**Minimum change.** None required beyond honoring the Gate Review's de-scope; optionally note Appendix A's ADR-0006(registry) as superseded by §15.4.

---

## F10 — Roadmap states the executor `deterministic` flag before ADR-0003 defines it

- **Severity: P3**
- **Category: Future dependency**

**Evidence.** Roadmap PR-011 introduces `deterministic: false`; Gate Review lists ADR-0003 as owning *"ports/Executor (determinism flag)"* and as blocking PR-008.

**Why it matters.** The Roadmap's acceptance criteria pre-state a contract ADR-0003 owns. The dependency is *documented* (ADR-0003 blocks PR-008), and the Roadmap text is consistent with ADR-0003's intended scope, so the risk is low — but the flag's semantics are not authoritative until ADR-0003 lands.

**Minimum change.** None beyond the existing Gate Review dependency; ensure PR-011 cites ADR-0003 as the owner of the flag's semantics.

---

## Findings summary

| ID | Severity | Category | One-line |
|---|---|---|---|
| F1 | **P1** | Contradiction / dup. source | ADR-0002 & ADR-0004 numbers mean different things across docs |
| F2 | **P1** | Contradiction / premature | WASM-preferred (ARCH) vs gRPC-first (Roadmap, ADR-0001) |
| F3 | **P1** | Architecture drift | §2.2 over-strong validator-determinism guarantee owned by ADR-0003 |
| F4 | **P1** | Governance | Stack built on unratified RFC-0001 (in MAJOR REVISION) |
| F5 | P2 | Contradiction | §5.3 vector extension (cgo) vs ADR-0001 CGO=0 default |
| F6 | P2 | Dup. source of truth | Manifest representation decided in ARCH and ADR-0004 |
| F7 | P2 | Circular dependency | ADR-0005 ↔ ADR-0006 capability-hook coupling |
| F8 | P2 | Missing decision | Cassette request-hash canonicalization unowned |
| F9 | P3 | Architecture drift | Registry decided §15.4 yet listed open in Appendix A |
| F10 | P3 | Future dependency | Determinism flag stated in Roadmap before ADR-0003 |

**No P0 findings.**

---

## Categories verified clean (explicit)

These checklist categories were examined and found **consistent** across the accepted set:

- **Language assumptions** — Go is consistent in all five documents; `ARCHITECTURE.md §5.4` correctly self-marks "decision pending → ratify via ADR-0001" and did not accidentally become normative.
- **Distribution assumptions** — single static binary / "same binary in CLI, daemon, CI" is coherent across `ARCHITECTURE.md §5.2–5.4`, Roadmap §7, and ADR-0001; SaaS is explicitly a *later deployment of the same kernel*, not a fork.
- **CI assumptions** — Roadmap §7 (no-network PR CI, cassette replay, nightly real-provider) is consistent with `ARCHITECTURE.md §14.2` ("same binary runs in both"); transitively depends on ADR-0003 (documented).
- **RFC philosophy (content)** — deterministic-first ordering, human-accountability/approval-by-default (Roadmap PR-038/PR-041 ↔ RFC §6.4), worktree isolation (ARCH §7.3), Derived/Authored knowledge split (ARCH §8 ↔ RFC §6.3, V2) are all consistently honored. *Philosophy-as-content is clean; philosophy-as-process is F4.*
- **Replay/determinism (mechanism)** — the cassette-for-non-deterministic / re-execute-for-deterministic model is consistent across ARCH §2.2, Gate ADR-0003, and Roadmap PR-008/PR-011. The only defect is the *validator classification* over-claim (F3) and the *cassette key* ownership gap (F8), not the model itself.
- **Versioning (ports)** — semver-on-ports with pre-1.0 breakage (ARCH §15.3) is internally consistent; the M6-ecosystem-on-unfrozen-ports tension is an *acknowledged tradeoff* in §15.3, not a contradiction.

---

## Determination — Can implementation begin without architectural debt?

**Conditional GO.**

**Safe to begin now:** **M0 (PR-001 → PR-006)** and the early-M1 ledger/artifact PRs that precede replay. These touch only the language decision (settled by ADR-0001, modulo its change-list), the pipeline model, and deterministic execution. They are insulated from every finding above: no finding concerns the pure execution engine before replay.

**Debt that must be cleared before its milestone (not project-blocking, but real):**
- **Immediately** (they corrupt coordination/authority *now*, regardless of milestone): **F1** (ADR numbering — every cross-reference is currently ambiguous) and **F4** (gate ADR *finalization* on RFC ratification; M0 may proceed under provisional status).
- **Before PR-008 / PR-012 (mid-M1):** **F3** and **F8** — the replay contract and validator classification must be authoritative (ADR-0003) before replay and validators are built.
- **Before ADR-0001 is finalized and before ADR-0008 / M6:** **F2** — decouple the isolation mechanism so the contradiction is not carried into the least-reversible document or the extension milestone.
- **Before M4 (ADR-0007):** **F5** — resolve the cgo/vector-tier classification.
- **Before M2/M3 ADRs are drafted:** **F6, F7** — single-home the manifest representation; break the 0005↔0006 cycle.

**Bottom line:** the project is **not yet debt-free at the document level** — four P1 inconsistencies exist — but none block the start of coding. The deterministic engine can be built today; the debt is concentrated in *citations* (F1), *governance status* (F4), *the determinism contract's ownership* (F3), and *a not-yet-made plugin decision leaking backward* (F2). Clear F1 and F4 this week, and the remaining P1/P2 items just-in-time at their milestones, and implementation proceeds without accumulating architectural debt past M0.

---

_End of consistency review. No new architecture proposed; no tradeoffs resolved. The findings are inconsistencies between accepted documents and the minimum pointer/ownership changes that remove them._
