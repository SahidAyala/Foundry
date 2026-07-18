# ADR-0000 — Governance & Ratification Process

| | |
|---|---|
| **Status** | **Accepted** |
| **Date** | 2026-07-16 |
| **Deciders** | The project's sole maintainer (git identity `SheykoWk`), drafted AI-assisted per this repo's own posture on AI-authored decisions (RFC-0001) |
| **Ratifies** | [OQ-006](../06-open-questions/OQ-006-governance-model.md) — graduates it from Open Question to this ADR, adopting its own **current recommendation** (Alternative 1: lightweight maintainer-led) verbatim. Retroactively confirms [ADR-0001](ADR-0001-language-and-toolchain.md)'s "accepted under interim authority" as fully **Accepted** — the authority ADR-0001 lacked a name for was, all along, the same sole maintainer this ADR names. |
| **Gates** | Every future RFC → ADR / RFC → Accepted transition. This is the mechanism, not a one-time exemption: nothing before this ADR changes retroactively except ADR-0001 (Decision 5) and, in the same pass as this ADR, [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) and [ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) (both explicitly walked through this process's Decision 1, see their own updated Status rows). |
| **Process note** | This ADR is necessarily self-ratifying — see Decision 6 for why that is a deliberate, named resolution of the bootstrap problem, not an oversight. |

---

## Context

[OQ-006](../06-open-questions/OQ-006-governance-model.md) named this "the deepest honesty issue in the project": RFCs, ADRs, and a documentation precedence chain existed, but nothing defined who could accept one, by what threshold, or how the constitution itself could be amended. Every maturity banner in the repository capped at PROVISIONAL for exactly this reason ([documentation.md](../04-guides/documentation.md), [concepts.md](../05-reference/concepts.md)) — not because the underlying decisions were weak, but because **nothing could legitimately ratify them**, including [ADR-0001](ADR-0001-language-and-toolchain.md), which was itself "accepted under interim authority. by a board with no chartered power."

Two ADRs already sat fully drafted and implemented against — [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) (Executor contract) and [ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) (VCS/PR integration) — each ending its own Review Checklist with the identical unticked line: *"Reconcile this ADR's Proposed status once OQ-006's governance process exists."* This ADR is that reconciliation.

The project today has exactly one contributor: the maintainer who owns this repository. [Roadmap.md](../00-overview/roadmap.md) already names "near-term single-user value" as a live open question (item 9) — this ADR does not resolve that broader question, but it does mean OQ-006's Alternative 2 (a consensus board) has no board to seat yet, and Alternative 3 (ratification-as-a-dogfooded-Act) is circular for the same reason OQ-006 already named: the Knowledge/Act machinery this would run through does not yet exist to run it through.

---

## Decision

1. **Adopt OQ-006's own current recommendation: a lightweight, maintainer-led process.** While the project has exactly one maintainer, that maintainer accepts or rejects a Draft/Proposed RFC or ADR directly, by editing its Status row in the same commit that changes its disposition — the commit itself, dated and attributed, *is* the audit record (V3), standing in for a comment period no one else yet reads.

2. **No enforced open comment period while the project has a single maintainer.** OQ-006's recommendation names a comment period as part of the lightweight process; this ADR narrows that specifically for the single-maintainer phase — a mandatory waiting period with no second reader is ceremony without an audience (Principle 6, "ceremony must be earned by value"), not rigor. This is a deliberate, named narrowing of the recommendation, not a silent drop of it.

3. **The narrowing in Decision 2 expires the moment a second contributor exists.** "A second contributor" means: anyone other than the maintainer merges a change, is granted write access, or is named as a co-decider on any future RFC/ADR. At that point, this ADR is **out of date on its face** and must be amended (Decision 5) to define a real, minimum comment period (this ADR recommends no less than 72 hours, but does not bind that number — the amending ADR decides it with a real second reader in mind) before any RFC/ADR may move to Accepted. This is not a "someday" aspiration; it is this ADR's own expiration condition, and whoever adds a second contributor is responsible for triggering the amendment in the same change that adds them.

4. **Tie-breaking and board composition are out of scope while there is one decider.** OQ-006's Alternative 2 (consensus board, majority threshold, charter) is not designed here — there is no tie to break yet. Designing it now would be exactly the "over-engineering governance for a pre-implementation-scale project" risk OQ-006 itself flagged.

5. **This ADR is amended, not overridden, the same way any other Accepted ADR is: a new ADR that names this one and supersedes the specific decision that changed.** There is no special-cased "constitutional amendment" procedure distinct from the ordinary ADR process — the process governs its own change. A future governance ADR (e.g., adding the comment period from Decision 3, or seating a board) supersedes this one's relevant Decision, not the whole document, unless it replaces the model outright.

6. **This ADR is necessarily self-ratifying, and that is the resolution of the bootstrap problem, not a gap in it.** Every governance process has to come from somewhere before it can certify itself; OQ-006 named this directly (Alternative 3 was rejected as "circular until the system exists"). The alternative to self-ratification is infinite regress — a second process to ratify the first. This ADR breaks the regress the same way ADR-0001 already had to, quietly, without naming it: the sole maintainer's existing, actual authority over this repository is the authority that accepts this ADR. What changes is that the authority is now named and recorded, instead of left as "interim" indefinitely.

7. **What this ADR does NOT do:**
   - It does not resolve the **principle priority ordering** open decision (roadmap item 2) — a separate, substantive question this process could one day be used to decide, but does not decide here.
   - It does not retroactively ratify RFC-0001 through RFC-0005. Each remains Draft — Proposed until individually walked through Decision 1 by name, in its own commit. (In the same working session as this ADR, that step was taken for ADR-0005 and ADR-0010 specifically, at the maintainer's explicit direction — not automatically, and not extended to the RFCs themselves.)
   - It does not make every PROVISIONAL document CANONICAL. Maturity is orthogonal to precedence and to this process ([documentation.md](../04-guides/documentation.md)): this ADR removes the *systemic* barrier ("nothing can reach CANONICAL because no process exists"), but each document still requires its own explicit ratification to actually move.
   - It does not name a sponsoring entity, a license anchor beyond what already exists, or a vendor-capture charter (V6) — those remain open, tracked in [OQ-006](../06-open-questions/OQ-006-governance-model.md)'s own Open Questions, which this ADR narrows but does not close entirely (see below).

---

## Alternatives Considered

### Wait for a second contributor before defining any process
- **For:** Avoids designing governance for an audience of one; a real second reader would surface requirements this ADR cannot anticipate.
- **Against:** Leaves ADR-0005 and ADR-0010 — already drafted, already implemented against, already blocking nothing except their own Status label — stuck in "Proposed" indefinitely for a reason (no process) rather than a reason (unresolved disagreement). The roadmap already named this the "highest-priority blocker for finalizing any decision"; waiting indefinitely keeps that blocker in place by default rather than by choice.
- **Verdict:** Rejected. A minimal process that expires itself on a real trigger (Decision 3) captures the caution without the indefinite stall.

### Seat a consensus board now, in anticipation of future contributors
- **For:** Would not need revisiting when the project grows.
- **Against:** No one exists to seat. A board with one member is a maintainer wearing a costume — it adds ceremony (charter, threshold, quorum rules) with no functional difference from Decision 1, violating Principle 6 directly.
- **Verdict:** Rejected. Design the board ADR when a board has members, per Decision 4.

### Encode ratification as a dogfooded Foundry Act
- **For:** Elegant — the project's own governance would run through the same trust machinery ([trust.md](../02-architecture/trust.md)) it builds for everything else.
- **Against:** OQ-006 already named this circular: Foundry's Knowledge/Authored-decision machinery is itself only PROVISIONAL and partially built (M4, ~55% per the project's own status audits). Routing governance through a system whose own governance is in question is not a foundation.
- **Verdict:** Rejected for now. Worth revisiting once Knowledge retrieval and Authored-note curation (RFC-0005 and its named follow-ups) are further along — noted as a real future option, not dismissed permanently.

---

## Consequences

### What this decision makes EASIER
- **ADR-0005 and ADR-0010 can be marked Accepted in the same working session**, using the process this ADR defines — see their updated Status rows. This is the concrete "unblock" the maintainer asked for: M3 (real Executors) and M5 (VCS/PR integration) now rest on Accepted, not merely Proposed, decisions.
- **Every future RFC/ADR has an unambiguous next step**: the maintainer edits its Status row and commits. No more "blocked on OQ-006" as a permanent-feeling dead end.
- **The maturity ceiling is no longer systemically capped.** [documentation.md](../04-guides/documentation.md) and [concepts.md](../05-reference/concepts.md) can now say CANONICAL is reachable — narrowly, per-document, on explicit ratification — rather than "aspirational."

### What this decision makes HARDER
- **Nothing enforces disagreement surfacing while solo.** A single decider cannot be outvoted or checked by this process alone — the only check is the maintainer's own discipline, and whatever external review (this conversation, a future contributor, a future audit) catches. This is a named, accepted risk of Decision 2, not a solved one.
- **Decision 3's trigger relies on the maintainer noticing and self-reporting** the moment a second contributor exists. There is no automated enforcement (no CI check can detect "a human joined"). This is a real, honest limitation, not a mechanism — it is a norm this ADR states loudly so it is at least a known obligation.

### Reversibility
High. Decision 5 makes this ADR amendable by the same process it defines, with no special procedure to invent later. Nothing it ratifies (ADR-0001's status, ADR-0005, ADR-0010) becomes harder to revisit — "Accepted" under this process carries the same "supersede via a new ADR" reversibility every other ADR already has.

---

## Migration Strategy

None required at the code level — this is a documentation- and process-only ADR. Its effects are: (a) this file; (b) Status-row edits to ADR-0001 (implicitly, via Decision 1's retroactive confirmation — no changes to ADR-0001's own file are made, since its "interim authority" language remains accurate historical context, not a live contradiction, per [documentation.md](../04-guides/documentation.md) rule 4); (c) Status-row edits to ADR-0005 and ADR-0010, made alongside this ADR; (d) OQ-006's Status moving to RESOLVED; (e) removing the now-false "nothing can reach CANONICAL" framing from [documentation.md](../04-guides/documentation.md) and [concepts.md](../05-reference/concepts.md); (f) correcting now-inaccurate "no governance process exists yet" asides scattered across RFC-0002 through RFC-0005 and the router implementation guide, without upgrading those RFCs' own Status.

---

## Future ADR Dependencies

- **Any RFC's individual ratification** (RFC-0001 through RFC-0005 remain Draft — Proposed): each is a future, separate act of Decision 1 — this ADR enables it but does not perform it.
- **A real comment-period ADR** (Decision 3's trigger): whoever adds a second contributor must author or prompt this amendment in the same change.
- **Principle priority ordering** (roadmap item 2): a distinct open decision this process could resolve once someone drafts it — not addressed here.
- **A future consensus-board ADR** (Decision 4): designed only once a board has members.

---

## Open Questions

Carried forward from [OQ-006](../06-open-questions/OQ-006-governance-model.md), narrowed but not closed:

1. **What licence anchors ownership, and is there a sponsoring entity?** Not decided here. No sponsoring entity exists today as far as this ADR is aware; V6 (no vendor capture) is upheld by that absence, not by a designed mechanism yet.
2. **How does sole-maintainer authority formally transfer or expand** — e.g., if the maintainer becomes unavailable? Not designed here; a real bus-factor risk this ADR accepts rather than solves, consistent with OQ-006's own framing of that risk for Alternative 1.
3. **What exact comment-period length and quorum apply once a second contributor exists?** Deliberately left to the amending ADR in Decision 3, with a non-binding recommendation of "no less than 72 hours."

---

## Review Checklist

- [x] **Resolves OQ-006 as its own current recommendation states**, without inventing a heavier process the project's current scale does not need (Principle 6).
- [x] **Does not silently upgrade any RFC to Accepted.** Confirmed: RFC-0001 through RFC-0005 remain Draft — Proposed; only ADR-0005 and ADR-0010 change status, and only because the maintainer explicitly directed both in this same session.
- [x] **Does not resolve the principle-priority-ordering question** (roadmap item 2) or any OQ other than OQ-006.
- [x] **Names its own expiration condition** (Decision 3) rather than presenting a single-maintainer process as permanent.
- [x] **Consistent with V6 (no vendor capture)**: authority is held by the maintainer of record, not a vendor, sponsor, or model provider; no clause here grants any AI system, model vendor, or tool decision-making authority — drafting assistance is explicitly distinguished from deciding (see Deciders row).

---

_This ADR does not invent governance the project has not earned yet — it names the authority that was already deciding things informally (ADR-0001's "interim authority" was always the sole maintainer) and gives it a recorded, self-amending process, so that "Proposed" stops meaning "stuck" and starts meaning "awaiting the one decision this ADR now makes possible."_
