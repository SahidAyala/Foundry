# Architecture Review Board — Pre-Implementation ADR Gate

| | |
|---|---|
| **Reviews** | RFC-0001, `ARCHITECTURE.md`, `IMPLEMENTATION-ROADMAP.md` (all accepted; checked for conflicts) |
| **Question before the board** | Which architectural decisions MUST be ratified as ADRs before implementation can begin? |
| **Date** | 2026-06-29 |
| **Outcome** | **8 ADRs** (4×P0, 2×P1, 2×P2). Accepted ADRs land in `docs/adrs/`. |

> **Board discipline applied.** The brief said *prefer fewer, stronger decisions*. We started from `ARCHITECTURE.md` Appendix A (7 candidate ADRs) and deliberately **consolidated, added one, and de-scoped three** rather than rubber-stamping it. We also record (in §"Explicitly NOT ADRs") the decisions we refused to turn into ADRs, because over-producing ADRs is itself an architectural smell.
>
> **Numbering.** We keep **ADR-0001 = Language** (consistent with both Appendix A and the roadmap's PR-001 reference). The rest are re-sequenced by dependency. A mapping to Appendix A is given at the end.
>
> **No conflicts found** among the three documents. The roadmap faithfully implements the architecture, which faithfully implements the RFC. These ADRs fill *gaps*, they do not resolve *contradictions*.

---

# Missing ADRs

## ADR-0001 — Language & Toolchain
**Priority: P0**

- **Decision.** Build the kernel, CLI, and adapters in **Go**, shipped as a single static cross-platform binary, on a pinned toolchain version. Keep the eventual *plugin* boundary language-agnostic (out-of-process) so the community is never locked to Go.
- **Motivation.** A single static binary is critical for a tool that is both a CLI and a CI executable; Go gives strong concurrency for fan-out orchestration, fast cold start, and a proven out-of-process plugin story. This blocks literally the first line of code — nothing can be scaffolded until it is settled. The architecture already recommends Go (§5.4); this ADR exists to *ratify*, not re-open.
- **Alternatives considered.** *Rust* — stronger type safety, but slower iteration, harder plugin/IPC story, steeper contributor on-ramp (works against the open-source goal). *TypeScript/Node* — excellent ecosystem and DX, but no single static binary and a runtime dependency, poor fit for a long-lived CLI/CI tool. *Python* — worst distribution and performance profile for this shape.
- **Consequences.** GC pauses (irrelevant at this workload); less type expressiveness than Rust (accepted); large contributor pool (a benefit). **A hard obligation falls out:** the plugin boundary (ADR-0008) must be language-agnostic, or "Go" silently becomes ecosystem lock-in.
- **Affected components.** Everything: `cmd/`, `internal/*`, `Makefile`, CI.
- **Blocked roadmap PRs.** **PR-001** — and therefore, transitively, the entire roadmap.
- **Estimated difficulty.** **S** (direction pre-decided in §5.4; this is a ratification, plus pinning toolchain/CI versions).

---

## ADR-0002 — State Persistence, Content-Addressing & On-Disk Layout
**Priority: P0**

- **Decision.** Settle the four coupled persistence questions as one: **(a) ledger storage engine** — recommend embedded **SQLite via a pure-Go driver** (preserves the static binary; gives queryable projections) over a raw append-only log; **(b) event schema versioning** — every event carries a version, evolution is **additive-only**, fields are never repurposed; **(c) content-addressing** — pin the hash function (recommend **SHA-256**) and the canonicalization rules for artifacts, context bundles, and events; **(d) `.foundry/` on-disk layout** — *authored* knowledge + definitions are committed to git (portable, diffable), the *derived* index + run store are gitignored (rebuildable cache). Reserve (interface only, do not implement) a remote-ledger seam, and a **mandatory secret-redaction hook** that runs before anything is persisted.
- **Motivation.** The ledger is the spine: replay, resume, audit, and observability are all *projections* over it (ARCH §7.4). These are the most expensive-to-reverse decisions in the system — the hash function and canonicalization are a permanent compatibility surface, and the git semantics of `.foundry/` are user-facing and painful to change after adoption. Bundling them prevents four small, drifting decisions.
- **Alternatives considered.** *Storage:* raw append-only file log (simple writes, weak querying, hand-rolled projections) vs SQLite (richer queries, transactions, one portable file — **chosen**) vs embedded KV/BoltDB (fast, manual indexing). *Hash:* SHA-256 (ubiquitous) vs BLAKE3 (faster, less standard). *Layout:* everything-in-repo (git pollution, derived churn leaks into review) vs everything-external (loses portability) — the **Derived/Authored split rejects both**.
- **Consequences.** A pure-Go SQLite driver is required to keep the static-binary promise (a real constraint, not a footnote). Event-versioning discipline is forever. The hash choice is frozen at v1. The clean git story makes team sync nearly free later. Secret redaction must be wired before the *first* persisted byte, not retrofitted.
- **Affected components.** `kernel/ledger`, `domain/artifact`, `config` (`.foundry/` discovery), the secrets-redaction seam.
- **Blocked roadmap PRs.** **PR-004** (`.foundry/` layout), **PR-007** (ledger), **PR-008/PR-009** (replay/resume read the store), **PR-010** (artifact hashing). Soft-touches **PR-022** (redaction seam must already exist).
- **Estimated difficulty.** **M** (one genuine spike: pure-Go SQLite vs append-log benchmark + the event-schema design).

---

## ADR-0003 — Replay & Determinism Contract
**Priority: P0** · *(NEW — not in Appendix A; the board's most important addition)*

- **Decision.** Specify Foundry's flagship guarantee precisely: **(1)** every stage/executor is classified `deterministic` or `non-deterministic`; **(2)** replay *re-executes* deterministic stages and asserts hash-identical artifacts, while non-deterministic stages (shell, LLM providers) are **recorded and replayed from a cassette**, never live-re-executed for the identity guarantee; **(3)** what replay asserts, and how divergence is reported as a first-class outcome; **(4)** the exact public wording of the guarantee — *process* determinism, not *output* determinism (RFC §6.5).
- **Motivation.** This is the platform's central trust property *and* its regression-test harness, yet Appendix A never made it an explicit decision. Getting the contract wrong silently undermines replay, CI verification, and the honesty value (RFC §V5). It must be pinned before replay is built and before the executor port is shaped (the determinism flag lives on the executor).
- **Alternatives considered.** "Best-effort replay" with no formal guarantee (kills the trust story — rejected). "Full output determinism" (impossible and dishonest for models — rejected by RFC §6.5). Re-execute everything live (non-reproducible, expensive — rejected).
- **Consequences.** Every executor must declare determinism. Cassette infrastructure becomes mandatory for non-deterministic stages — *built* in M3, but *designed for* now. The payoff is a clean, honest, defensible public guarantee.
- **Affected components.** `kernel/orchestrator` (replay), `kernel/ledger`, `ports/Executor` (determinism flag), provider cassettes (M3).
- **Blocked roadmap PRs.** **PR-008** (replay). **Constrains** PR-011 (executor port must carry the flag) and **PR-028** (provider cassettes).
- **Estimated difficulty.** **M** (low code, high need for precise semantics — the hard part is writing the contract exactly).

---

## ADR-0004 — Declarative Manifest Schema & Evolution Policy
**Priority: P0**

- **Decision.** Ratify both the **canonical representation** (YAML authored / JSON canonical, **strict unknown-field rejection** — leaned in ARCH §7.1/§10.1) and, crucially, the **schema evolution policy**: a `version:` field per manifest kind; additive changes within a major version; breaking changes bump the major and ship a migration note/tool; one schema registry covering pipeline + skill (+ later profile) kinds; **never a DSL**.
- **Motivation.** The manifest is the user-facing source of truth. Once people author pipelines, the schema is the most painful thing in the system to change. The *evolution policy* — not the field list — is the durable decision, and it must exist before the first schema ships publicly at PR-005.
- **Alternatives considered.** No version field (silent breakage — rejected). Permissive/ignore-unknown parsing (silent drift, undebuggable — rejected). A bespoke DSL (unmaintainable mini-language — rejected, ARCH §10.1). JSON-authored (worse DX — rejected).
- **Consequences.** Schema discipline forever; strict parsing means precise errors but less forgiving input; a real obligation to ship migration tooling at the first major bump.
- **Affected components.** `internal/schema`, `domain` (pipeline/skill types), authoring docs, `examples/`.
- **Blocked roadmap PRs.** **PR-005** (pipeline schema), **PR-016** (skill manifest). Indirect: **PR-020** (pack version pinning).
- **Estimated difficulty.** **S/M** (representation largely decided; formalizing the evolution/migration policy is the real work).

---

## ADR-0005 — Executor Normalized Contract & Port/Adapter Conventions
**Priority: P1**

- **Decision.** Define the **normalized Executor request/response contract** that *all* executors share (shell, mock, Anthropic, OpenAI, Ollama): declared inputs, captured outputs-as-artifacts, the determinism flag (from ADR-0003), required-capability hooks (link to ADR-0006), and a common error taxonomy. Also ratify the **project-wide port/adapter convention**: ports are Go interfaces in `internal/ports`; adapters never import kernel internals; *introduce a port with its first adapter, generalize it with its second*. Explicitly **excludes** the out-of-process/plugin boundary (that is ADR-0008).
- **Motivation.** The mock→real executor swap (M2→M3) is only clean if this contract is right. Per the roadmap's anti-over-abstraction rule (§0), it is formalized when the **second** adapter (the mock, PR-019) appears — *not* pre-locked at PR-011 when only the shell executor exists.
- **Alternatives considered.** Lowest-common-denominator executor interface (discards provider power — rejected, ARCH §11.1). A bespoke interface per executor (no swappability — defeats the purpose). Locking the full LLM-shaped contract at PR-011 before any LLM exists (premature — violates roadmap §0).
- **Consequences.** A single contract must accommodate both a shell command and a streaming, tool-calling model — some impedance is accepted in exchange for drop-in provider replacement.
- **Affected components.** `ports/Executor`, `adapters/executor/*`, `kernel/orchestrator`.
- **Blocked roadmap PRs.** **PR-019** (mock executor — the second adapter), **PR-021** (provider port builds on it). *Constrains but does not block* **PR-011** (ship the shell executor with a deliberately minimal interface; do not lock it).
- **Estimated difficulty.** **M**.

---

## ADR-0006 — Provider Capability Descriptor, Routing & Negotiation
**Priority: P1**

- **Decision.** Ratify the **CapabilityDescriptor** schema (context window, supported features, cost model, rate limits, locality) and the **negotiation/degradation rules** (skills declare required + preferred capabilities; the router matches; advanced features degrade gracefully when absent). Settle the **routing policy taxonomy** (cost / latency / quality / privacy-constrained incl. `must_be_local`) and **failover** semantics.
- **Motivation.** This is *how* Foundry stays provider-agnostic without collapsing to a lowest-common-denominator (RFC §6.3, ARCH §11). The descriptor is a contract every adapter implements; changing it later churns every adapter at once.
- **Alternatives considered.** LCD interface (rejected). Hard-coded per-provider routing (no policy flexibility — rejected). Model-driven routing (violates deterministic control flow, RFC §6.2 — rejected).
- **Consequences.** Every provider adapter must publish and maintain an accurate descriptor, *including cost* (a real maintenance burden, versioned with the adapter). Enables privacy-constrained routing for sensitive repos as a first-class feature.
- **Affected components.** `ports/Provider`, the router, `adapters/executor/{anthropic,openai,ollama}`, `kernel/budget` (cost).
- **Blocked roadmap PRs.** **PR-021** (provider port + descriptor), **PR-023** (router), **PR-024** (failover), **PR-029** (degradation).
- **Estimated difficulty.** **M/L** (the capability taxonomy is broad and must anticipate future provider features without becoming a god-object).

---

## ADR-0007 — Knowledge & Semantic Store
**Priority: P2**

- **Decision.** Choose the **knowledge-graph persistence** — recommend **reusing the embedded SQLite engine** (ADR-0002) with a node/edge schema, over a dedicated graph DB, to preserve the single-binary/local-first story — *and* the **embedding/vector index + on-device embedding strategy** for air-gapped mode. Enforce the **Derived/Authored split at the storage layer** (derived = rebuildable, gitignored; authored = in-repo source of truth).
- **Motivation.** M4 is the differentiator. The store choice governs query expressiveness and the local-first guarantee; the embedding strategy governs air-gapped viability. It is genuinely deferrable to M4 because deterministic-first retrieval (PR-032) is sequenced *before* the semantic tier (PR-036) — the board endorses not deciding this early.
- **Alternatives considered.** Dedicated graph DB (Neo4j-class — richer traversal, but breaks single-binary/local-first — rejected for v1). SQLite-as-graph (good enough, portable — **recommended**). External vector DB (operational weight — rejected) vs embedded vector index (sqlite-vec/usearch-class — keeps local-first — **recommended**).
- **Consequences.** SQLite-as-graph limits traversal performance at extreme scale (acceptable until multi-repo, post-1.0). An on-device embedding model adds binary size or a download step for air-gapped mode.
- **Affected components.** `adapters/knowledge/{graph,extractors}`, the context engine's semantic tier, air-gapped mode.
- **Blocked roadmap PRs.** **PR-030** (graph store), **PR-036** (semantic tier). Explicitly does **not** block **PR-032** (deterministic retrieval) — that is the point of the sequencing.
- **Estimated difficulty.** **L** (two spikes: graph-on-SQLite ergonomics; on-device embedding).

---

## ADR-0008 — Extension Contract: Plugin Isolation & Port Versioning
**Priority: P2**

- **Decision.** Settle the **plugin isolation mechanism** — recommend **gRPC subprocess as the primary path** (language-agnostic, mature) with **WASM component model as a hardening track** — and the **port-versioning policy** that makes the extension surface a stable public contract (semver on ports; pre-1.0 may break with migration notes; post-1.0 frozen — ARCH §15.3), plus the **capability-permission model** for community plugins (default-deny).
- **Motivation.** The extension surface *is* the product for the community (ARCH §15), but it is an M6 concern; deciding the mechanism now would be speculative. It is listed here so the board is **on record that M6 cannot begin without it**, and so the public SDK seam (`pkg/sdk`, `api/proto`) is *reserved in the layout today* — which it already is — to avoid an M6 rewrite. This also discharges ADR-0001's obligation to keep the plugin boundary language-agnostic.
- **Alternatives considered.** In-process dynamic linking (language lock-in, host crashes, unsafe near credentials — rejected, ARCH §15.2). WASM-only (component ecosystem too immature near-term — deferred to hardening). No port versioning (no ecosystem can form — rejected).
- **Consequences.** Out-of-process adds IPC overhead (fine for plugins, not for hot paths). The capability model is real security work. The semver freeze is a hard, public commitment at v1.0.
- **Affected components.** `pkg/sdk`, `api/proto`, `extension/runtime`, *all* ports.
- **Blocked roadmap PRs.** **PR-045** (SDK), **PR-046** (runtime), **PR-047** (sandbox), **PR-048** (versioning) — i.e., all of M6.
- **Estimated difficulty.** **L/XL**.

---

## Explicitly NOT ADRs (decisions the board refused to inflate)

Recording these is part of "prefer fewer, stronger":

- **Library choices** (cobra, bubbletea, YAML lib, SQLite driver) — design-level and reversible. The YAML-library *position-info* requirement is an **acceptance criterion** on PR-003, not an ADR. The pure-Go SQLite *driver* is a **consequence captured under ADR-0002**, not its own decision.
- **Secrets handling** — already decided by ARCH §17 (keychain/`SecretsPort`, never in `.foundry/`). The only cross-cutting requirement (redact-before-persist) is **folded into ADR-0002**. No standalone ADR.
- **Registry / distribution format** — ARCH §15.4 already decided *git+manifest first, OCI later*. Revisit post-1.0. No pre-implementation ADR. *(This de-scopes Appendix A's ADR-0006.)*
- **Governance / license / RFC-process** — a **project** decision, not architectural; flagged as P0-2 in the RFC-0001 round-1 review. The **license** (Apache-2.0 placeholder) must land before PR-001 is public; full governance before M6 opens to the community. Tracked in the governance RFC, **out of ADR scope** — the board notes it as a hard prerequisite, not an architecture ADR.
- **Stage parallelism, observability sink selection, error/exit-code conventions** — design-level, deferrable; no early decision required.

---

# Recommended ADR implementation order

Ordered by implementation dependency. Tiers may be ratified in parallel within a tier.

```
Tier 0  (before PR-001)            ADR-0001  Language & Toolchain
                                       │  everything is Go (or not) from here
Tier 1  (before M0→early M1)       ADR-0002  Persistence/Identity/Layout   ──┐  (parallel)
        PR-004 / PR-005 / PR-007   ADR-0004  Manifest Schema & Evolution   ──┘
                                       │
Tier 2  (before PR-008)            ADR-0003  Replay & Determinism Contract
        depends on 0002 (reads ledger); feeds the executor determinism flag
                                       │
Tier 3  (before PR-019, M2)        ADR-0005  Executor Normalized Contract
        depends on 0003                │
                                       │
Tier 4  (before PR-021, M3)        ADR-0006  Capability Descriptor & Routing
        depends on 0005                │
                                       │
Tier 5  (before PR-030, M4)        ADR-0007  Knowledge & Semantic Store
        depends on 0002 (reuses storage engine)
                                       │
Tier 6  (before PR-045, M6)        ADR-0008  Extension Contract
        depends on stable ports from 0005/0006
```

**Reading of the order:**
1. **ADR-0001** is the gate to writing any code; ratify it before PR-001.
2. **ADR-0002 and ADR-0004 are parallelizable** and together unblock the entire M0→M1 critical path — 0002's *layout* aspect is needed first (PR-004), 0004 at PR-005, 0002's *ledger* aspect at PR-007.
3. **ADR-0003** depends on the ledger existing conceptually (0002) and must precede replay (PR-008); it also dictates a field on the executor port, so it precedes ADR-0005.
4. **ADR-0005 → ADR-0006** form the executor/provider chain for M2→M3; the mock executor is the forcing function for 0005, real providers for 0006.
5. **ADR-0007** (M4) only needs the storage engine from 0002 and is otherwise independent — safe to defer.
6. **ADR-0008** (M6) needs stable ports (0005/0006) and is the latest; deciding it earlier would be speculative.

**The practical gate to "start coding tomorrow":** only **ADR-0001** truly blocks PR-001. **ADR-0002 and ADR-0004** must be ratified before the M0 critical path reaches PR-004/PR-005 — days, not weeks, away — so they should be drafted immediately in parallel. Everything from ADR-0003 onward can be ratified just-in-time at its tier, consistent with the roadmap's depth-before-breadth, decide-late-when-cheap philosophy.

---

## Appendix — Mapping to `ARCHITECTURE.md` Appendix A

| This gate | Appendix A | Disposition |
|---|---|---|
| ADR-0001 Language | ADR-0001 | Unchanged |
| ADR-0002 Persistence/Identity/Layout | ADR-0004 (ledger) | **Expanded** — absorbs content-addressing, `.foundry/` layout, redaction seam |
| ADR-0003 Replay & Determinism | — | **Added by the board** (was missing) |
| ADR-0004 Manifest Schema & Evolution | ADR-0005 | Unchanged in spirit; emphasis moved to the *evolution policy* |
| ADR-0005 Executor Contract & Ports | — (partial) | **Added** — the normalized executor contract was implicit |
| ADR-0006 Capability & Routing | ADR-0007 | Unchanged |
| ADR-0007 Knowledge & Semantic Store | ADR-0003 (embedding) | **Merged** with the graph-store choice |
| ADR-0008 Extension Contract | ADR-0002 (isolation) | **Merged** with port-versioning |
| — | ADR-0006 (registry) | **De-scoped** — pre-decided by ARCH §15.4; revisit post-1.0 |

_End of gate review. Ratify ADR-0001 to unblock PR-001; draft ADR-0002 and ADR-0004 in parallel immediately behind it; ratify the rest just-in-time at their tiers. Accepted ADRs are written to `docs/adrs/`._
