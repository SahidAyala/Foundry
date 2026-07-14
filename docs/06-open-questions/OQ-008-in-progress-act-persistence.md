# OQ-008 — Where does an interrupted Act's in-progress state live?

**Maturity: OPEN QUESTION** · informs [../02-architecture/execution.md](../02-architecture/execution.md), [trust.md](../02-architecture/trust.md) (PROVISIONAL)

## Problem
`foundry resume` needs to persist an Act's progress before it reaches a terminal Judgment, so a crash or kill mid-Pipeline leaves something a later invocation can continue. But `record.FileStore.Write` is deliberately write-once (`os.O_EXCL`, `record.ErrAlreadyExists` on a second write): an Act, once recorded, is immutable. Where does state that is *not yet* the Record live?

## Context
`engine/checkpointer.go`'s own doc comment already names this tension: a Pipeline that declares more than one `record` Step for the same Act sees the second write fail. Today's Pipelines declare at most one, so it has never mattered — until resuming an interrupted Act requires writing progress *before* that terminal `record` Step ever runs. This connects to [../00-overview/roadmap.md](../00-overview/roadmap.md)'s open decision 6 ("Record durability classification — durable, not a cache") and the ADR backlog's "Persistence, content-addressing & on-disk layout" row ([../03-adrs/README.md](../03-adrs/README.md)) — this question feeds that eventual ADR, it does not preempt it.

## Alternatives
1. **Separate mutable checkpoint store** — a distinct, overwritable file (e.g. `checkpoint.json`) alongside the immutable `act.json`, deleted once the Act reaches a real terminal disposition (pass, exhausted repair, rejected, budget-exceeded). The Record's "immutable once written" guarantee is untouched; a checkpoint is explicitly *not* the Record.
2. **A mutable "draft" phase in the Record itself** — let an Act be written multiple times until a terminal freeze. Heavier: touches every place that currently relies on `FileStore.Write` being write-once, and blurs "recorded" with "in progress."
3. **Record-everything as an event-sourced ledger** — every Step becomes its own immutable, appended event; identity is by replaying the event log, not by a single mutable file. Resembles the archived `kernel/ledger` design this codebase does not have; too heavy for M1's "no model involved" scope.

## Arguments
- A separate checkpoint store is the smallest change: it adds one new mutable artifact, touches nothing about how the Record is read or written, and disappears the moment an Act is legitimately done.
- A mutable Record phase is tempting (one file, not two) but risks callers accidentally relying on a "recorded" Act that can still change underneath them — the exact failure mode "immutable once written" exists to prevent.
- Record-everything is the most principled long-term answer (it would also strengthen OQ-003's cross-version replay story) but is a bigger, event-sourced rebuild not justified by resume alone.

## Open questions
- Should the checkpoint's on-disk shape ever be promoted directly into `act.json`, or does the terminal write always re-derive from the finished in-memory Act? (Today: the latter — the checkpoint is discarded, not renamed.)
- Does a future multi-Pipeline-attempt (e.g. resuming across a repair boundary) change which alternative is right? Not resolved here — see the resume PR's stated scope boundaries.

## Current recommendation
Alternative 1: a separate, mutable `record.CheckpointStore`, sharing the same root as `record.FileStore` so a checkpoint sits alongside its eventual `act.json`, written after every Step and deleted once a terminal Judgment is reached. PROVISIONAL.

## Status
**OPEN.** Owns the same pending ADR as OQ-003 (replay & determinism contract) and the "Persistence, content-addressing & on-disk layout" ADR; see [../03-adrs/README.md](../03-adrs/README.md).
