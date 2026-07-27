# ADR-0014 — Principle Priority Ordering

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-26, the same day it was drafted. The maintainer confirmed both decisions below directly, in response to concrete, grounded questions (not an abstract ranking exercise) — see Context. |
| **Date** | Drafted 2026-07-26; ratified 2026-07-26 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | [roadmap.md](../00-overview/roadmap.md) open decision 2 ("Principle priority ordering") — partially: see Decision 2 below for exactly what remains open. |
| **Gates** | [principles.md](../00-overview/principles.md)'s own "Open governance question" callout, which this ADR closes for the one relationship it actually resolves (Values vs. Principles), not for a full total order. |

---

## Context

[principles.md](../00-overview/principles.md) names 8 numbered **Core principles** and 6 lettered **Values that are never compromised** (V1–V6), and states plainly: "the principles do not yet have a ratified priority ordering for when two of them conflict." That is [roadmap.md](../00-overview/roadmap.md) open decision 2.

Asking "rank these 14 items" cold, with no grounding, is exactly the kind of abstract exercise this project's own documentation discipline warns against (PROVISIONAL statements dressed up as settled truth). Instead, the maintainer was asked two concrete, already-real questions:

1. **Do the six Values (V1–V6) always outrank the eight numbered Principles when they conflict?** The document's own naming already implies this — Values are labeled "never compromised," Principles are described only as things that "exist to reject proposals" (a functional test, not an absolute ranking) — so this question asks whether to *formalize* an implication already sitting in the text, not invent a new one.
2. **A concrete worked example, already latent in the shipped system:** should a trivial Act (e.g. a one-line typo fix) ever be allowed to skip human approval if the deterministic Gate passes — trading Principle 6 ("ceremony must be earned by value") against Value 1 ("Accountability stays with a human") and Principle 4 ("approval, not autonomy, by default")?

The maintainer answered both directly: **yes, Values always win**, and **never skip approval, no exceptions, not even a per-project opt-in**.

A third, related question was also asked (invest in Derived Knowledge/semantic retrieval now, speculatively, vs. keep waiting for [ADR-0007](ADR-0007-knowledge-and-semantic-store.md)'s own named trigger) — the maintainer chose to **keep waiting**. That changes nothing about ADR-0007's own text or Decision, so it is not recorded as a Decision here; it is a confirmation of the status quo, not a new one.

## Decision

1. **When a numbered Principle (1–8) and a Value (V1–V6) conflict, the Value always wins.** This formalizes principles.md's own implicit hierarchy rather than inventing one: the "never compromised" label on V1–V6 is now literally true in a comparative sense, not only a description of each Value taken alone.

2. **This does not establish a total order.** Specifically out of scope, and still genuinely open:
   - Any ranking *among* the 8 numbered Principles when two of them conflict with each other (neither Value is involved).
   - Any ranking *among* the 6 Values themselves, on the rare occasion two Values might conflict with each other.

   [roadmap.md](../00-overview/roadmap.md) open decision 2 should be updated to reflect this as a **partial** resolution — the Values-outrank-Principles relationship is settled; a full order within either tier remains an open question for whoever needs it next, resolved the same grounded way this one was (a concrete conflict, not an abstract ranking).

3. **Worked example, ratified as a concrete instance of Decision 1, not a separate rule:** an Act must never skip human approval to save ceremony, however trivial the change looks. V1 (Accountability stays with a human) and Principle 4 (approval, not autonomy, by default) always outrank Principle 6 (ceremony must be earned by value) in this specific tension — with no per-project opt-in exception. This requires **no code change**: `cli.CLI.Do`'s existing direct-apply fallback already prompts for interactive approval before applying, even for the built-in `default`/`review` Pipelines, which declare no explicit `approve` Step — there was never a live path that skipped it. This Decision ratifies that already-shipped behavior as the permanent rule, not merely today's implementation choice.

## Consequences

- **Positive:** [principles.md](../00-overview/principles.md)'s own "Open governance question" callout can be closed for the relationship it actually names (Values vs. Principles); a future proposal that pits a Value against a numbered Principle has a ratified answer instead of an escalation. The worked example gives a concrete, citable precedent for the next person who proposes a "let low-risk changes skip approval" feature — it is not an oversight to catch, it is already decided.
- **Cost:** none identified — this ratifies already-shipped behavior (Decision 3) and formalizes an already-implied reading of an existing document (Decision 1); no code changes, no new compatibility surface.
- **Harder:** the partial-resolution framing (Decision 2) must be carried correctly into [roadmap.md](../00-overview/roadmap.md) — marking open decision 2 as fully resolved would misrepresent what was actually decided; a future reader must be able to tell "Values beat Principles" is settled while "Principle 3 vs. Principle 7" is not.

## Migration Strategy

None — no code changes. Update [principles.md](../00-overview/principles.md)'s "Open governance question" callout and [roadmap.md](../00-overview/roadmap.md) open decision 2 to reflect the partial resolution.

## Review Checklist

- [x] Maintainer confirmed Decision 1 (Values always outrank Principles) directly.
- [x] Maintainer confirmed Decision 3's worked example (never skip approval, no opt-in) directly.
- [x] Confirmed no code change is required — the worked example ratifies already-shipped behavior (`cli.CLI.Do`'s existing approval prompt), not a new mechanism.
- [x] Confirmed this does not overclaim a full priority order — Decision 2 states plainly what remains open.
