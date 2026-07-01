# ADR-0001 — Change List to Reach ACCEPT

| | |
|---|---|
| **Targets** | `docs/adrs/ADR-0001-language-and-toolchain.md` |
| **Source review** | `docs/reviews/ADR-0001-review.md` (verdict: ACCEPT WITH CHANGES) |
| **Goal** | Minimal set of changes to move the verdict to **ACCEPT** |
| **Date** | 2026-06-29 |

> This is a change *specification*, not a rewrite. Each item states the architectural change required; it does not propose prose. Only the three findings that **gated** promotion are listed. Non-gating findings (A2, A3, A4, F5, F6, F7, F8) are deliberately excluded — see "Excluded" — because the brief requires the *minimal* set, and those do not block ACCEPT.

---

## Change 1 — Decouple ADR-0001 from ADR-0008's isolation mechanism *(resolves F1 + F4)*

- **What must change (architecturally):** The ADR must stop treating "gRPC-subprocess primary, WASM as a hardening track" as a settled premise in its justification (currently in "What this decision makes HARDER" and "Future ADR Dependencies"). It must reduce its forward claim to the part it legitimately owns — the **language-agnostic boundary obligation** (clause 4) — and either (a) drop the mechanism assertion entirely, or (b) record it explicitly as an *unresolved tension* owned by ADR-0008. The language decision must not *reason from* the mechanism choice.
- **Why:** ADR-0001 is the least-reversible decision in the project. As written, its justification depends on an un-made downstream decision *and* asserts an outcome that runs against the accepted architecture. This couples a permanent decision to a contingent one and propagates a contradiction into every document that later cites ADR-0001.
- **Architectural impact:** Removes a false dependency edge (language → plugin-mechanism) from the decision graph. ADR-0008 regains an unconstrained design space for isolation. The Go decision is left resting only on R1–R4, which is sufficient and correct on its own.
- **Justifying document:** `ARCHITECTURE.md §15.2` ("WASM component model **preferred**; subprocess … as fallback") — the statement the ADR contradicts; `ARCHITECTURE.md` Appendix A (ADR-0002) and `reviews/pre-implementation-adr-gate.md` (ADR-0008) — which assign the isolation-mechanism decision elsewhere.
- **Normative or editorial:** **Normative.** It removes a claim and a dependency from the architectural record.

---

## Change 2 — Complete the `CGO_ENABLED=0` tension to include the vector index *(resolves F2 + A1)*

- **What must change (architecturally):** The acknowledged conflict between the `CGO_ENABLED=0` default build and downstream needs currently names only *on-device embeddings*. It must be widened to also name the **vector index** as a cgo pressure, so the constraint ADR-0007 inherits is complete. The point is the *constraint set*, not the prose: ADR-0007 must inherit "pure-Go default conflicts with **both** the embedding model **and** the vector index," not just the former.
- **Why:** The architecture's own semantic-retrieval approach relies on a vector index, and the leading embedded options are C extensions — a second, independent collision with the pure-Go default that the ADR omits. An incompletely stated constraint causes ADR-0007 to inherit a false sense of how much cgo pressure exists, which is exactly the kind of unacknowledged downstream cost an ADR exists to prevent.
- **Architectural impact:** Makes the pure-Go-default policy's true cost explicit at the point of decision, so ADR-0007 (and any build-policy decision) confronts the full tension rather than discovering half of it later.
- **Justifying document:** `ARCHITECTURE.md §9` (Context Engine — the semantic/embedding tier that requires a vector index); `reviews/pre-implementation-adr-gate.md` (ADR-0007 — "embedded vector index … keeps local-first"); `IMPLEMENTATION-ROADMAP.md §8` (semantic retrieval as postponed but committed).
- **Normative or editorial:** **Normative.** It changes the constraint formally handed to ADR-0007.

---

## Change 3 — Acknowledge the distribution-model irreversibility *(resolves F3)*

- **What must change (architecturally):** The ADR rates only the *language* as low-reversibility. It must additionally record that the **pure-Go single-artifact distribution model** (the consequence of the `CGO_ENABLED=0` default) carries its own moderate irreversibility once install/distribution and downstream packagers depend on one universal binary. This is a recorded consequence, not a new rule.
- **Why:** The ADR's "CGO behind a build tag" escape hatch implies cgo is cheaply reversible. At the *language* level it is; at the *distribution* level it is not — fragmenting into platform-specific cgo builds later breaks the single-artifact install model. Leaving this unstated understates the cost of the open CGO question (Open Question 1) and of any future build-policy reversal.
- **Architectural impact:** Correctly raises the recorded stakes of the still-open CGO/full-build question, so ADR-0007 and any build-policy ADR weigh a distribution-model change as moderately irreversible rather than trivial.
- **Justifying document:** `ARCHITECTURE.md §5.2` (same binary, local + CI) and `§5.3` (single static binary as the distribution unit); `IMPLEMENTATION-ROADMAP.md §7` (single-binary signed-release model).
- **Normative or editorial:** **Normative.** It adds an irreversibility to the recorded consequence set that downstream decisions must respect.

---

## Excluded (intentionally, to keep the set minimal)

These findings from the review are **not** required to reach ACCEPT and are omitted on purpose:

- **F6 (status rests on undefined governance)** — already honestly flagged in the ADR and carried on its own checklist; reconciled when governance lands, not a precondition for ACCEPT.
- **A2 (domain-purity ≠ portability), A4 (Node single-binary overstated)** — reasoning imprecisions whose *conclusions* are unaffected; no architectural decision changes.
- **A3 (Go governance neutrality), F7 (supply-chain surface), F8 (binary size / non-Go IDE clients)** — true but non-gating consequences; recordable later without altering any decision.
- **F5 (cgo "full build" leans into build-policy)** — already deferred via Open Question 1; no action needed to ACCEPT.

---

## Acceptance condition

With Changes 1–3 applied, the two P1 findings (F1/F4) and the two gating P2 findings (F2/A1, F3) are discharged. No P0 existed. The board's verdict moves from **ACCEPT WITH CHANGES** to **ACCEPT**, and ADR-0001 may be promoted from provisional to final **before ADR-0008 begins**, so the decoupling in Change 1 is in place before the plugin-isolation decision is taken.

All three changes are **normative**; none are editorial; none are stylistic.
