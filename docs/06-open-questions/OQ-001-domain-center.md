# OQ-001 — What is the center of the domain?

**Maturity: OPEN QUESTION** · informs [../02-architecture/domain.md](../02-architecture/domain.md) (currently PROVISIONAL)

## Problem
The architecture currently centers the domain on the **Act** (a justified, accountable transition of project state). Is that the right center — or is **Knowledge** the truer center, or some third framing?

## Context
This conclusion originated in a first-principles review (now archived), authored as part of this project's own reasoning — **not** drawn from a ratified document. The review itself flagged the center as unresolved. Making "Act is the fundamental abstraction" read as canonical truth was an over-reach.

## Alternatives
1. **Act as center** — every feature is an Act; Knowledge is the medium Acts read from and write to.
2. **Knowledge as center** — the project's justified self-model is the durable asset; Acts are merely the transitions that evolve it ("Knowledge is the integral of all accepted Acts").
3. **Dual-pole** — Act is the *dynamic* fundamental, Knowledge the *durable* fundamental; neither is subordinate.
4. **Neither** — a different unit (e.g. the reviewable Change, or the Decision) is more fundamental.

## Arguments
- *For Act:* every capability decomposes into one with no remainder; it is where trust, accountability, and the record attach.
- *For Knowledge:* it is what the RFC names as durable capital and what compounds; it survives when Acts are forgotten.
- *For dual-pole:* the two may be facets of one idea, and forcing a single center may be a false economy.

## Open questions
- Can "Knowledge is the integral of accepted Acts" be made rigorous, or does it break on rejected/abandoned Acts that still teach?
- Does centering on Act bias the system toward *doing* over *understanding*?

## Current recommendation
Adopt **Act as the dynamic center with Knowledge as the durable medium** (alternative 1/3 blended) as the **current working model**, because it expresses the full lifecycle cleanly and is the least committal about mechanism. Treat it as PROVISIONAL.

## Status
**OPEN.** Resolution should come via an RFC once a governance process exists ([OQ-006](OQ-006-governance-model.md)). Until then, [domain.md](../02-architecture/domain.md) must label this as the current working model, not architectural truth.
