# Open Questions

> **Status: none of these are decided.** This tier holds architectural questions that are *too developed to be notes* but *not mature enough to be RFCs or ADRs*. It exists so that active deliberation has a home that is **explicitly non-canonical** — preventing reviews and working hypotheses from leaking into the canonical docs as if they were settled.
>
> A document here carries a **Current recommendation**, but a recommendation is *not a decision*. When one of these resolves, it graduates into an RFC or ADR and the canonical docs are updated; until then, the canonical docs must describe the question's *current working answer* as **PROVISIONAL** and link here.

## Why this tier exists

The first-pass refactor archived eight Architecture Review Board reviews. Their *conclusions* were integrated into the architecture docs — but several of those conclusions were **working hypotheses or my own design proposals**, not facts supported by accepted documents. Writing them as canonical architecture was an honesty failure. This tier is the correct home for that class of thinking.

## Index

| ID | Question | Current recommendation | Status |
|---|---|---|---|
| [OQ-001](OQ-001-domain-center.md) | What is the center of the domain — Act, Knowledge, or something else? | Act as the dynamic center, Knowledge as the durable medium | OPEN |
| [OQ-002](OQ-002-pipeline-as-strategy.md) | Is the predeclared graph (Pipeline) one Strategy or the system's spine? | One Strategy among several | OPEN |
| [OQ-003](OQ-003-replay-across-versions.md) | Does the replay guarantee hold across Engine versions? | Scope it to same-version initially | **RESOLVED** → [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md) |
| [OQ-004](OQ-004-validator-determinism.md) | What can the verification guarantee honestly promise? | Record-and-replay, not "pure function" | **RESOLVED** → [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md) (not per this page's own recommendation — see page) |
| [OQ-005](OQ-005-extension-isolation.md) | How are third-party extensions isolated and versioned? | Undecided — requirements only | OPEN |
| [OQ-006](OQ-006-governance-model.md) | How are decisions ratified? | Lightweight maintainer-led process first | **RESOLVED** → [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) |
| [OQ-007](OQ-007-canonical-terminology.md) | Is the proposed vocabulary (Act/Engine/Strategy/…) the right one? | Adopt provisionally, revisit at first implementation | OPEN |
| [OQ-008](OQ-008-in-progress-act-persistence.md) | Where does an interrupted Act's in-progress state live, given the Record is write-once? | Separate mutable checkpoint store, deleted on terminal Judgment | **RESOLVED** → [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md) |

## Relationship to other tiers

```
notes (informal) → 06-open-questions (deliberation, non-canonical) → 01-rfcs / 03-adrs (decisions) → 02-architecture (canonical description)
```

Each document follows the same shape: **Problem · Context · Alternatives · Arguments · Open questions · Current recommendation · Status.**
