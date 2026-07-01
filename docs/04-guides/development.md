# Development Guide

> **Building M0 now? The active, concrete plan is [m0-plan.md](m0-plan.md).** This guide covers the durable build approach (testing, CI, structure) that outlives any single milestone; milestones are in [../00-overview/roadmap.md](../00-overview/roadmap.md). Terms: [../05-reference/terminology.md](../05-reference/terminology.md). The accepted language/toolchain decision is [../03-adrs/ADR-0001-language-and-toolchain.md](../03-adrs/ADR-0001-language-and-toolchain.md).

## Build approach

- **Vertical slices, not horizontal layers.** Every change leaves the repository building, tested, and usable. No big-bang merges.
- **Deterministic core before any model.** Build the **Engine** that produces **Acts** — verification, **Judgment**, **Record**, replay — entirely without a model. A model is one **Executor**, integrated only after the core is trustworthy.
- **Earn each abstraction.** Introduce a boundary only when its first real implementation exists; generalize only when a second appears.
- **Honor the invariants** in [../05-reference/invariants.md](../05-reference/invariants.md) — especially: control flow in the Engine, outputs untrusted until verified, the Record durable and immutable.

## Structure principles (conceptual, not a fixed layout)

- A **conservative core** (Engine, Record, Judgment, Gate semantics) with the substrate (**Executors**, **Validators**, **Context Sources**, Strategies) at a replaceable edge — see [../02-architecture/system-context.md](../02-architecture/system-context.md) and [../02-architecture/extensibility.md](../02-architecture/extensibility.md).
- The core depends only on contracts; everything touching the outside world is an adapter behind one. Substrate never reaches into the core.

## Testing strategy

- **Unit** — pure domain logic, table-driven.
- **Golden/snapshot** — for parser/renderer/output surfaces.
- **Integration** — run real example Acts end-to-end and assert the Record.
- **Property/invariant** — flagship checks: an Act replays identically; repair always terminates; a Judgment is a pure function of its Evidence.
- **Contract/conformance** — each substrate boundary ships a suite every adapter must pass.
- **Record/replay (cassettes)** — non-deterministic Executors are recorded; CI replays them. **No live model calls in PR CI.**

## CI strategy

- Every PR must build, pass tests (incl. race detection), and lint — green is required to merge.
- Branch protection; `main` is always releasable.
- Cross-platform binary build in CI; conventional commits to enable generated changelogs.
- Live-model checks run only in a gated, non-PR job.

## M0 Implementation Plan

**The active, executable plan for M0 is [M0-IMPLEMENTATION-BACKLOG.md](M0-IMPLEMENTATION-BACKLOG.md)** — a PR-by-PR breakdown, dependencies, acceptance criteria, and success signals. For a condensed checklist, see [M0-QUICK-REFERENCE.md](M0-QUICK-REFERENCE.md).

A historical plan predating the current ([Act-centric](../02-architecture/domain.md)) terminology is archived at [../archive/obsolete/IMPLEMENTATION-ROADMAP.md](../archive/obsolete/IMPLEMENTATION-ROADMAP.md) for reference only.
