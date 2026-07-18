# Roadmap

> The strategic build sequence and the open decisions that gate it. The detailed build plan (PR-level steps, testing, CI, repository layout) lives in [../04-guides/development.md](../04-guides/development.md). Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Guiding rule

**Depth before breadth.** Build one complete, trustworthy loop before widening the lifecycle. Build the deterministic core before any model is integrated — a model is one **Executor**, added late.

## Milestones

| Milestone | Theme | "Usable system" state |
|---|---|---|
| **M0** | Walking skeleton | The engine produces a trivial, fully deterministic Act end-to-end |
| **M1** | Deterministic core | Verified, judged, budgeted Acts; immutable Record; replay & resume — **no model involved** |
| **M2** | Reusable production + mock executor | Author reusable Act templates; produce Acts against a fixture Executor, deterministically; bounded repair |
| **M3** | Real executors | Model-backed Executors behind capability routing; record/replay against real models |
| **M4** | Knowledge & evidence | Authored/Derived Knowledge; evidence assembly with provenance; knowledge-update as a reviewed Act |
| **M5** | Integration & visibility | Worktree isolation, change review/approval, version-control integration, observability |
| **M6** | Extensibility | Open substrate edge: third-party Strategies, Executors, Validators, Context Sources |
| **M7** | Stability | Frozen core contracts, hardening, multi-user / enterprise readiness |

Each milestone is independently useful. A project could stop at M1 and have a rigorous deterministic engine with audit and replay.

## Current implementation status

The table above states the *plan*. For what has actually shipped per milestone, per RFC, and per architecture document — kept current as a living index rather than duplicated here — see [implementation-status.md](implementation-status.md). Getting-started instructions for what's usable today (install, dependencies, first run) live in [../04-guides/getting-started.md](../04-guides/getting-started.md).

## Open decisions that require a human (architectural / governance)

These are unresolved and **must not be settled silently in implementation**. They are the prerequisites to any architecture "freeze". The ones that are architectural (not pure governance) are treated in depth, with alternatives and a current recommendation, in [../06-open-questions/](../06-open-questions/) — that tier is the home for this deliberation so it never leaks into canonical docs.

1. ~~**Governance & ratification process.**~~ **RESOLVED 2026-07-16** — see [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md): a lightweight, sole-maintainer-led process, with no enforced comment period while the project has one maintainer, and a named trigger (a second contributor) that forces a real comment period to be defined. The founding RFC (RFC-0001) itself remains unratified — resolving *how* decisions are ratified does not itself ratify any pending RFC.
2. **Principle priority ordering.** No ratified rule for resolving conflicts between core principles (see [principles.md](principles.md)).
3. **The center of the domain — Act vs Knowledge** (see [../02-architecture/domain.md](../02-architecture/domain.md)).
4. **Cross-version replay scope** (see [../02-architecture/execution.md](../02-architecture/execution.md)).
5. **Validator determinism limits & the honest verification guarantee** (see [../02-architecture/trust.md](../02-architecture/trust.md)).
6. **Record durability classification** — durable, not a cache (see [../02-architecture/trust.md](../02-architecture/trust.md)).
7. **Authored-knowledge format stability & migration** (see [../02-architecture/knowledge.md](../02-architecture/knowledge.md)).
8. **Extension isolation mechanism & contract versioning** (see [../02-architecture/extensibility.md](../02-architecture/extensibility.md)).
9. **Cost as a first-class constraint**, and **near-term single-user value** vs the long compounding bet.
10. **Concurrency / scale model** and whether remote operation is truly additive (see [../02-architecture/system-context.md](../02-architecture/system-context.md)).
11. **The project name** ("Foundry" is provisional).

Several of these have proposed owning ADRs; see [../03-adrs/README.md](../03-adrs/README.md).

## What may begin now

M0 work may proceed under the accepted language decision ([../03-adrs/ADR-0001-language-and-toolchain.md](../03-adrs/ADR-0001-language-and-toolchain.md)). It is insulated from the open decisions above, which gate later milestones — not the first commit.
