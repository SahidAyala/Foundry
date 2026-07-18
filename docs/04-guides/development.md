# Development Guide

> **M0 is complete; [m0-plan.md](m0-plan.md) is kept as design rationale, not an active plan.** This guide covers the durable build approach (testing, CI, structure) that outlives any single milestone; current status per milestone is in [../00-overview/roadmap.md](../00-overview/roadmap.md). Want to install and run Foundry today? See [getting-started.md](getting-started.md). Terms: [../05-reference/terminology.md](../05-reference/terminology.md). The accepted language/toolchain decision is [../03-adrs/ADR-0001-language-and-toolchain.md](../03-adrs/ADR-0001-language-and-toolchain.md).

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

## Current implementation status

**M0 is complete**, and the codebase has since shipped well past it (multi-executor router, VCS/PR publish, Knowledge capture and retrieval). See [../00-overview/roadmap.md](../00-overview/roadmap.md) for the current status per milestone, and [getting-started.md](getting-started.md) to install and run what exists today.

M0's original PR-by-PR execution plan is archived at [../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md](../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md) / [M0-QUICK-REFERENCE.md](../archive/obsolete/M0-QUICK-REFERENCE.md), kept for traceability of how M0 was actually sequenced — not as a current plan.

A historical plan predating the current ([Act-centric](../02-architecture/domain.md)) terminology is archived at [../archive/obsolete/IMPLEMENTATION-ROADMAP.md](../archive/obsolete/IMPLEMENTATION-ROADMAP.md) for reference only.
