# ADR-0015 — Domain center: Act as the dynamic center, Knowledge as the durable medium

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-26, the same day it was drafted. The maintainer confirmed directly, in response to a concrete question naming the exact working model already built (not an abstract "pick a center" exercise) — see Context. |
| **Date** | Drafted 2026-07-26; ratified 2026-07-26 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Resolves** | [OQ-001](../06-open-questions/OQ-001-domain-center.md) — "What is the center of the domain: Act, Knowledge, or something else?" — and [roadmap.md](../00-overview/roadmap.md) open decision 3. |
| **Note on mechanism** | [OQ-001](../06-open-questions/OQ-001-domain-center.md)'s own text suggests this should resolve "via an RFC." This ADR resolves it via an ADR instead, matching this project's own actual established practice: [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) closed [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md), [ADR-0003](ADR-0003-replay-and-determinism-contract.md) closed [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)/[OQ-004](../06-open-questions/OQ-004-validator-determinism.md), [ADR-0008](ADR-0008-extension-isolation-and-contract-versioning.md) closed [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) — every prior open question in this repository has been closed by a narrow ADR, never by ratifying an entire RFC wholesale. This ADR does **not** ratify [RFC-0001](../01-rfcs/RFC-0001-vision-and-product-philosophy.md) (which remains unratified in full) — only this one specific question. |

---

## Context

[domain.md](../02-architecture/domain.md) has, since this project's earliest reasoning, centered the domain on the **Act** — "a justified, accountable transition of Project State" — with **Knowledge** as the durable medium an Act reads from and writes into. That document has always carried an explicit honesty note: this is a **working hypothesis** originating in the project's own first-principles reasoning, not drawn from any ratified document, and [OQ-001](../06-open-questions/OQ-001-domain-center.md) itself names a credible alternative (Knowledge as the true center, with Acts as merely its transitions) and a third framing (dual-pole: neither subordinate).

OQ-001's own "Current recommendation" already named the answer this ADR ratifies: "Adopt Act as the dynamic center with Knowledge as the durable medium (alternative 1/3 blended) as the current working model... Treat it as PROVISIONAL." Every architecture document in this repository ([domain.md](../02-architecture/domain.md), [execution.md](../02-architecture/execution.md), [trust.md](../02-architecture/trust.md), [knowledge.md](../02-architecture/knowledge.md), [system-context.md](../02-architecture/system-context.md), [extensibility.md](../02-architecture/extensibility.md)) and the entire shipped codebase (`domain.Act`, `domain.Outcome`, `record.FileStore`, `knowledge.Gatherer`, `workspace.KnowledgeNoteApplier`) already builds on exactly this blend — the question was never "what should we build," it was "has anyone with authority to decide actually said yes to this."

The maintainer was asked directly, grounded in this exact already-adopted model (not an abstract "rank Act vs Knowledge" exercise): ratify the current working model as-is, push further toward Knowledge as the true center, or leave it open. The maintainer confirmed **ratify the current working model as-is**.

## Decision

1. **The Act is the domain's dynamic center; Knowledge is its durable medium.** Every capability reduces to an Act with no remainder (per [domain.md](../02-architecture/domain.md)'s own "What every feature reduces to" table); Knowledge is what an Act's Evidence draws on and what an accepted Act's Outcome deposits into — "the integral of all accepted Acts," in [OQ-001](../06-open-questions/OQ-001-domain-center.md)'s own phrasing, without either being subordinate to the other. This is [OQ-001](../06-open-questions/OQ-001-domain-center.md)'s alternative 1/3 blend, exactly as already built — nothing about the domain model's actual shape changes as a result of this ADR.

2. **This does not resolve every sub-question OQ-001 or OQ-002 name.** Specifically still open, and not addressed here:
   - Whether **Strategy** is itself a domain concept or merely the boundary to mechanism ([OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md)).
   - Whether a **rejected or abandoned Act** still deposits into Knowledge (a real question — does a Judgment's *rejection* still teach the project something, per OQ-001's own "does it break on rejected/abandoned Acts that still teach?").
   - Whether the terminology itself ([OQ-007](../06-open-questions/OQ-007-canonical-terminology.md)) is right — this ADR ratifies the *center*, not every term describing it.

   These remain genuinely open, tracked in their own OQ documents — this ADR must not be read as having silently closed them.

3. **[domain.md](../02-architecture/domain.md)'s maturity note is updated, not removed.** It no longer says the domain center is unresolved; it now cites this ADR as having ratified the working model, while still correctly noting the vocabulary itself ([OQ-007](../06-open-questions/OQ-007-canonical-terminology.md)) and the Strategy/rejected-Act sub-questions ([OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md)) remain open. Per [AGENTS.md](../../AGENTS.md)'s maturity rules, this does not make the domain model **CANONICAL** — the ceiling for any concept remains PROVISIONAL until a governance process exists beyond the current sole-maintainer ADR-0000 process ([OQ-006](../06-open-questions/OQ-006-governance-model.md)) — but it is no longer an unratified hypothesis either; it is now a **ratified** PROVISIONAL model, the same standing every other Accepted ADR in this repository already carries.

## Consequences

- **Positive:** the single most-cited "honesty note" in this repository's architecture docs — "the Act is *the* fundamental abstraction is a working hypothesis... not drawn from a ratified document" — is now inaccurate in the way that mattered: it *is* drawn from a ratified document, this one. A future contributor reading [domain.md](../02-architecture/domain.md) no longer needs to wonder whether the whole domain model could be overturned by a single unelaborated open question; the center is settled, only its edges (Strategy-as-domain-concept, rejected-Act semantics, vocabulary) remain open.
- **Cost:** none identified — this ratifies the model exactly as already built and documented; no code changes, no new compatibility surface, no term redefinitions.
- **Harder:** future readers must not over-read this ADR as having closed OQ-002/OQ-007 too — Decision 2 above exists specifically to prevent that overreach, since "the center is decided" is an easy sentence to misremember as "the whole domain model is decided."

## Migration Strategy

None — no code changes. Update [domain.md](../02-architecture/domain.md)'s maturity note, [OQ-001](../06-open-questions/OQ-001-domain-center.md)'s own Status section, its row in [open-questions/README.md](../06-open-questions/README.md), [concepts.md](../05-reference/concepts.md)'s "Act as domain center" row, and [roadmap.md](../00-overview/roadmap.md) open decision 3.

## Review Checklist

- [x] Maintainer confirmed ratifying the current working model (Act dynamic center, Knowledge durable medium) directly, rather than pushing toward Knowledge-as-center or leaving it open.
- [x] Confirmed this does not overclaim resolution of OQ-002 (Strategy as domain concept, rejected-Act semantics) or OQ-007 (vocabulary) — Decision 2 names these explicitly as still open.
- [x] Confirmed resolving via a narrow ADR rather than ratifying RFC-0001 wholesale matches this repository's own established practice for closing prior open questions (OQ-003/004/005/008 precedent).
