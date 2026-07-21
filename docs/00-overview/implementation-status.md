# Implementation Status

> **A single, living dashboard of what's actually built vs. still planned — across the roadmap, every RFC, every ADR, and every architecture document — so a new implementation session can start here instead of reconstructing it from `git log` or re-reading five RFCs.** This is a status *index*, not a source of truth: each row links to the document that actually defines the thing. Update the relevant row (and add a dated line to the changelog at the bottom) whenever a milestone ships, an RFC's scope changes, or an ADR is accepted — the failure mode this file exists to prevent is exactly what happened to the old `M0-IMPLEMENTATION-BACKLOG.md` checklist (see [archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md](../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md)): left unread while the code moved on, until it actively misled.
>
> **Last full audit: 2026-07-18.** Two different axes are tracked separately per item, because they move independently in this project: **ratification** (has a human formally accepted this decision under [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)?) and **implementation** (does the code actually do this today?). A document can be fully implemented and still formally unratified — that is this project's normal state, not a defect, per [m0-plan.md](../04-guides/m0-plan.md)'s "nothing blocks writing code."

## 1. Roadmap milestones

The full milestone *plan* (themes, "usable system" definitions) lives in [roadmap.md](roadmap.md). Current shipped status:

| Milestone | Status | Notes |
|---|---|---|
| **M0** — Walking skeleton | Shipped, validated live | All of M0.0–M0.3 (`foundry do`, scripted + real Executor, budget, repair, `log`/`show`). Historical plan: [archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md](../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md). Confirmed working end-to-end with a live Executor on 2026-07-18 — see §7. |
| **M1** — Deterministic core | Shipped | Verified/judged/budgeted Acts, immutable filesystem Record, replay (`foundry replay`) and resume (`foundry resume`, mid-Pipeline checkpoints). |
| **M2** — Reusable production + mock executor | Shipped | Authored Pipeline documents (`.foundry/pipelines/*.json`) stand in for "reusable Act templates"; `executor.ScriptedExecutor` is the fixture Executor; bounded repair works. |
| **M3** — Real executors | Shipped, partially | Two real Executors behind a `Router` — Claude Code (default) and OpenAI (named). Routing is **explicit-pin-only**; capability-based negotiation is deferred (see RFC-0002 row below). |
| **M4** — Knowledge & evidence | Partial | Authored Knowledge write + naive lexical retrieval shipped. Derived Knowledge, semantic retrieval, and provenance scoring not started. |
| **M5** — Integration & visibility | Partial | VCS/PR publish shipped. Worktree isolation for concurrent Acts, a change-review UI, and observability not started. |
| **M6** — Extensibility | Not started | No third-party extension surface exists. |
| **M7** — Stability | Not started | No contract freeze; several open decisions below still gate it. |

Getting-started instructions for what's usable today live in [../04-guides/getting-started.md](../04-guides/getting-started.md).

## 2. RFCs — decision status vs. code status

All five RFCs remain **Draft — Proposed**: a governance process exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but none has been individually ratified through it yet. Their *implementation* status varies widely and is tracked here so "unratified" is never misread as "unbuilt."

| RFC | Ratified? | Implementation | Pending |
|---|---|---|---|
| [RFC-0001](../01-rfcs/RFC-0001-vision-and-product-philosophy.md) — Vision & product philosophy | No | N/A — a values/philosophy document, not a code deliverable; already reflected throughout the architecture docs and the system's actual behavior (trust gate, human approval, deterministic-first). | Formal ratification only. |
| [RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) — Pipeline execution runtime | No | **~85%.** Phases 1–6 and 8 shipped: Step trace on `Act`, `Strategy`/`PipelineStrategy`, Approve/Record as declared Step kinds, `FixedStrategy` fully retired, Router (explicit-pin), interactive shell. | **Phase 7** — capability-based negotiation/failover (cost/latency/quality weighting) — deliberately deferred, no real multi-Executor-per-Capability case yet. One of Phase 0's two prerequisite ADRs — Reusable-Act/Pipeline template format & evolution policy — still unwritten (§3 below). |
| [RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md) — Interactive assistant & multi-executor pipelines | No | **~90%.** Interactive session & slash commands (§3.1), project-local Pipeline authoring via `/init` (§3.2), VCS/PR automation (§4.1, [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md)) all shipped. **Product-shape direction confirmed 2026-07-18** (maintainer decision, informal): the interactive session is Foundry's primary interface; `foundry do` is CI/automation-only — see [ADR-0009](../03-adrs/README.md) below. | Same gap as RFC-0002: real capability-based routing validated against a multi-role lifecycle (§3.3) is not built — today's Router is explicit-pin, two vendors. |
| [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) — Multi-executor router & publish policy | No | **100% of its own 6 Pieces shipped** (Capability/Router/ExecutorRegistry, [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) accepted, second Executor OpenAI, Knowledge-lite capture, per-Step budget accounting, VCS/PR via [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md) accepted). | Nothing code-side; only the RFC document's own ratification. |
| [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) — Authored Knowledge retrieval | No | **100% of its own 4 pieces shipped** (formalized `.foundry/knowledge/` as the Authored store, `gatherer.Compose`, naive lexical Knowledge Gatherer, wired into both composition roots). | Nothing code-side for what this RFC proposed. Explicitly out of its scope, and not yet proposed by any RFC: Derived Knowledge indexing, semantic retrieval, Authored-knowledge format stability. |

## 3. ADRs

Accepted vs. backlog is tracked canonically in [../03-adrs/README.md](../03-adrs/README.md) — this section only flags what's implementation-relevant beyond that table:

- **Accepted (4):** [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) (governance), [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) (Go), [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) (Executor contract — implemented), [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md) (VCS/PR — implemented).
- **Backlog, implementation-relevant:** "Reusable-Act template format & evolution policy" and "Routing & policy (ADR-0006)" are the two backlog rows §2 above names as the actual pending-implementation gaps (Pipeline/Act template versioning is undecided; capability-based routing beyond explicit-pin is unbuilt). "CLI & output contract (ADR-0009)" now has its product-shape question informally settled (interactive primary) per §2's RFC-0003 note — writing and ratifying the ADR itself is what remains.

## 4. Architecture documents (`docs/02-architecture/`)

All PROVISIONAL (nothing in the repository is CANONICAL yet — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) made the ceiling reachable, but each document still needs its own ratification). Maturity nuance for each is tracked in [../05-reference/concepts.md](../05-reference/concepts.md)'s maturity index — this table adds *implementation* coverage, which that index doesn't track:

| Document | Answers | Implementation coverage |
|---|---|---|
| [domain.md](../02-architecture/domain.md) | What are the domain concepts? | High — Act, Intent, Evidence, Outcome, Judgment, Authority, Record all exist as real types (`domain/act.go`) and are exercised end-to-end. |
| [execution.md](../02-architecture/execution.md) | How does the system produce outcomes? | High — Pipeline-as-Strategy, Steps, repair are all real (`engine/strategy.go`, `engine/document.go`). |
| [trust.md](../02-architecture/trust.md) | How is trust established? | High for the mechanics (Verify/Gate/Judgment/Approve all run today); the *verification guarantee's* exact strength remains [OQ-004](../06-open-questions/OQ-004-validator-determinism.md), unresolved by design. |
| [knowledge.md](../02-architecture/knowledge.md) | What is durable knowledge? | Partial — Authored Knowledge (write + naive retrieval) is real; Derived Knowledge is pure concept, no code. |
| [system-context.md](../02-architecture/system-context.md) | What are the system boundaries? | Partial — today's actual (local, single-user) boundary is accurately described; the "remote is additive" and concurrency claims are unproven, no code exercises them. |
| [extensibility.md](../02-architecture/extensibility.md) | What may be extended? | Low — requirements only. No third-party extension surface exists (M6 not started); the isolation mechanism is explicitly undecided ([OQ-005](../06-open-questions/OQ-005-extension-isolation.md)). |

## 5. Guides (`docs/04-guides/`)

| Guide | Status | Use it for |
|---|---|---|
| [getting-started.md](../04-guides/getting-started.md) | Active | Installing and running Foundry today — the actual first stop for a new user. |
| [pipelines.md](../04-guides/pipelines.md) | Active | Authoring/reading a Pipeline document. |
| [development.md](../04-guides/development.md) | Active | Durable build/test/CI approach (not milestone-specific). |
| [m0-plan.md](../04-guides/m0-plan.md) | Historical (kept as design rationale) | *Why* M0 was scoped the way it was — not a current plan. |
| [multi-executor-router-implementation-plan.md](../04-guides/multi-executor-router-implementation-plan.md) | Historical (all 6 Pieces shipped) | Reference for *how* RFC-0004 was actually sequenced. |
| [interactive-session-implementation-plan.md](../04-guides/interactive-session-implementation-plan.md) | Check before reuse | Sequencing plan for RFC-0003's interactive session — verify against §2 above before treating any step as still-pending. |
| [contributing.md](../04-guides/contributing.md), [documentation.md](../04-guides/documentation.md), [release.md](../04-guides/release.md) | Active | Process guides, not milestone-specific — not audited for staleness here. |

## 6. Reference & open questions — pointers only

- Concept-by-concept maturity: [../05-reference/concepts.md](../05-reference/concepts.md) (do not duplicate here — that file is the canonical maturity index).
- Canonical vocabulary: [../05-reference/terminology.md](../05-reference/terminology.md).
- Invariants: [../05-reference/invariants.md](../05-reference/invariants.md).
- Open architectural questions (non-canonical, active deliberation): [../06-open-questions/README.md](../06-open-questions/README.md).

## 7. Real-world validation log

Distinct from the audits above (which read code and docs): this tracks actual runs of the built binary against a real repository with a live Executor — the strongest evidence available that the walking skeleton still works end-to-end, not just that its unit tests pass in isolation.

- **2026-07-18** — Ran a full interactive session (`/init` → `/bug "<intent>"` → real Claude Code Executor → `go build`/`go test` verification → human approval → apply → record) against a throwaway Go-module sample repository. Confirmed working end-to-end: real patch generation, verification, approval, Act recording, and both `foundry log`/`foundry show` history inspection all matched their documented behavior. Surfaced one real bug in the process — the `go-build` validator (`verify/detect.go`) left a compiled binary artifact in the repository on every Act against a `package main` module — fixed same day (`fix(verify): Stop go-build validator from leaking a binary into the repo`).

## Changelog

- **2026-07-18** — Initial audit. Archived the stale M0 checklist docs; added this file; registered the confirmed interactive-primary product-shape direction against ADR-0009 in [../03-adrs/README.md](../03-adrs/README.md).
- **2026-07-18** — First real dogfooding run (§7): validated the walking skeleton end-to-end with a live Claude Code Executor; found and fixed a `go-build` validator bug that leaked a compiled binary into the repository on every Act.
