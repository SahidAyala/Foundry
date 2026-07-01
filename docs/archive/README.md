# Archive

> **Historical context only. Nothing here is canonical.** Do not use these documents to understand the current system, and do not mix their terminology or conclusions into active docs. Git preserves full history; this archive keeps a few documents discoverable because their *reasoning* is referenced by the canonical docs.

## Contents

- **`obsolete/`** — documents that described a system that is no longer true.
  - `ARCHITECTURE.md` — the original monolithic blueprint, built on the retired *Workflow / Stage / Provider* model. Superseded by the split architecture in [`../02-architecture/`](../02-architecture/).
  - `IMPLEMENTATION-ROADMAP.md` — the detailed PR-level build plan, written in pre-canonical terminology. Its strategic content now lives in [`../00-overview/roadmap.md`](../00-overview/roadmap.md); it must be re-baselined to canonical terms before reuse.

- **`reviews/`** — Architecture Review Board reviews. **Reviews were never canonical.** Their accepted conclusions have been integrated into the canonical docs (see [the migration record](MIGRATION-2026-06-29.md)); they are kept only for traceability of *why* decisions were made.

- **`rejected-rfcs/`** — RFCs that were proposed and declined. (None yet.)

## Why keep an archive at all

The canonical docs occasionally cite an archived review for the *derivation* of a decision (e.g. why the domain centers on the Act). The archive exists to make those citations resolvable — not to be read as current documentation.
