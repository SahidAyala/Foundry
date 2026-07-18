# OQ-006 — How are decisions ratified?

**Maturity: RESOLVED** · graduated to [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) (Accepted 2026-07-16) · this page is now historical context for that ADR, not a live question

## Problem
The repository has RFCs, ADRs, and a documentation precedence chain — but **no defined process for accepting an RFC or ADR, amending the constitution, or resolving conflicts between equal-precedence documents.** Without it, nothing can legitimately be ratified, and "CANONICAL" is a status nothing can currently reach.

## Context
This is the deepest honesty issue in the project. The founding RFC is itself unratified and under major revision; ADR-0001 is "accepted under interim authority" — by a board with no chartered power. The first-pass docs implied a settledness the governance vacuum does not support.

## Alternatives
1. **Lightweight maintainer-led** — a named maintainer (or small group) accepts decisions after an open comment period; simplest; bus-factor and capture risk.
2. **Consensus board** — a small board ratifies by majority; slower; needs charter.
3. **RFC-process-as-code** — acceptance encoded as a Foundry Act (dogfooding); elegant but circular until the system exists.

## Arguments
- Something minimal must exist *before* any decision can be more than provisional.
- It must answer: who accepts, by what threshold, how is the constitution amended, who breaks ties, how is vendor capture prevented (V6).
- Over-engineering governance for a pre-implementation project is itself a risk.

## Open questions
- Who holds final authority initially, and how does that scale to a community?
- What is the relationship between the project and any sponsoring entity (relevant to V4/V6)?
- What licence anchors ownership?

## Current recommendation
Adopt a **lightweight maintainer-led process with an open comment period** as an explicit interim governance ADR, *and label everything it has not yet ratified as PROVISIONAL.* Until that ADR exists, treat all "accepted" statuses (including ADR-0001) as interim. PROVISIONAL.

**Adopted, with one named narrowing**: [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) takes this recommendation as its Decision 1 verbatim, but suspends the open comment period specifically while the project has a single maintainer (its own Decision 2) — a scope this recommendation did not anticipate because it did not distinguish "solo" from "small group." ADR-0000 names the exact trigger (a second contributor) that ends the narrowing and forces a real comment period to be defined.

## Status
**RESOLVED — 2026-07-16.** Graduated to [ADR-0000 — Governance & Ratification Process](../03-adrs/ADR-0000-governance-and-ratification-process.md), Accepted. The maturity ceiling this question capped is no longer systemically PROVISIONAL — CANONICAL is reachable per-document, on explicit ratification, per ADR-0000. The Open Questions this page named (licence/sponsoring entity; authority transfer on maintainer unavailability; comment-period length and quorum once a second contributor exists) are carried forward, unresolved, in ADR-0000's own Open Questions section — this page is kept for historical context, not as a live question.
