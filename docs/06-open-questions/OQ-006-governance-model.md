# OQ-006 — How are decisions ratified?

**Maturity: OPEN QUESTION** · the prerequisite for anything reaching CANONICAL · informs all tiers

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

## Status
**OPEN — highest priority.** Until resolved, the maturity ceiling for the whole repository is PROVISIONAL.
