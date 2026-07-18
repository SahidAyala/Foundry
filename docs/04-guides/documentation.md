# Documentation Guide

> How Foundry's documentation is organized and maintained. This repository is **documentation-first**: the docs are the source of truth and the code implements them. Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## The rules

1. **Single source of truth.** Every concept is defined in exactly one place. Definitions live in [../05-reference/terminology.md](../05-reference/terminology.md); other docs *reference*, never redefine.
2. **One concept = one document.** Architecture is split by responsibility — each file answers exactly one question (see below). Do not create another monolithic architecture document.
3. **No duplication.** If you need to explain a concept that already exists, link to it.
4. **No history in active docs.** Active documentation describes only the current system. Superseded material is archived, not updated; git preserves history.
5. **Canonical terminology only.** Retired terms (Workflow, Stage, Provider, Skill, Runtime) appear only in [../archive/](../archive/) and the retired-terms table.

## Where things live

| Need | Location |
|---|---|
| Why Foundry exists; principles; glossary; roadmap | `00-overview/` |
| Foundational decisions & reasoning | `01-rfcs/` |
| Specific binding decisions | `03-adrs/` |
| What the system is, split by responsibility | `02-architecture/` |
| Canonical definitions, concept map, invariants | `05-reference/` |
| How to work in the repo | `04-guides/` |
| Superseded / historical material | `archive/` |

## Architecture split (one responsibility each)

- `domain.md` — *What are the domain concepts?*
- `execution.md` — *How does the system produce outcomes?*
- `knowledge.md` — *What is durable knowledge?*
- `trust.md` — *How is trust established?*
- `extensibility.md` — *What may be extended?*
- `system-context.md` — *What are the system boundaries?*

Never mix these concerns in one file.

## Maturity levels

Every major concept and document carries one of four maturity levels, shown in a banner at the top of the document and indexed in [../05-reference/concepts.md](../05-reference/concepts.md):

- **CANONICAL** — accepted through governance; future work must follow it. *Reachable, narrowly: [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) resolved the systemic blocker (no ratification process existed), but each document still needs its own explicit ratification — nothing is upgraded automatically.*
- **PROVISIONAL** — the current best understanding; sound enough to build on, but may evolve. **Most of this repository is here.**
- **OPEN QUESTION** — unresolved; lives in [../06-open-questions/](../06-open-questions/) with a *current recommendation* that is explicitly not a decision.
- **REJECTED** — historical only; lives in [../archive/](../archive/); never mixed into active docs.

Maturity is **orthogonal to precedence**: precedence decides which document wins a conflict; maturity decides how settled the content is. A PROVISIONAL document still wins over a lower-precedence one — but neither may be presented, or relied on, as CANONICAL truth. **Never silently upgrade a PROVISIONAL or OPEN QUESTION into canonical prose.**

## Precedence

README → Overview → RFCs → Accepted ADRs → Architecture → Reference → Guides → Archive. When two active documents disagree, the higher-precedence one wins and the lower must be updated or archived. **Never leave a contradiction in active docs.**

## Adding or changing a concept

1. Define or amend it once in `terminology.md`.
2. Reference it from the architecture/reference docs that use it.
3. If it changes the model, also update the relevant single-responsibility architecture file and the invariants — and check nothing now contradicts it.
4. If a document becomes obsolete, **move it to `archive/`; do not leave it updated-but-stale.**
