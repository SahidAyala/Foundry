# Architecture Review Board — ADR-0001 (Language & Toolchain)

| | |
|---|---|
| **Reviews** | `docs/adrs/ADR-0001-language-and-toolchain.md` |
| **Against** | RFC-0001, `ARCHITECTURE.md`, `IMPLEMENTATION-ROADMAP.md`, `reviews/RFC-0001-review-round-1.md`, `reviews/pre-implementation-adr-gate.md` |
| **Nature** | Architecture review (correctness), not editorial |
| **Date** | 2026-06-29 |
| **Verdict** | **ACCEPT WITH CHANGES** |

> Scope note: no wording feedback, no rewrites. Findings concern architectural correctness, consistency, and unstated coupling only. The core decision (Go + static-binary toolchain policy) is **endorsed**; PR-001 may proceed. The required changes are scoping and acknowledgment fixes, not a reversal.

---

## 1–4. Consistency with accepted documents

**RFC-0001 (philosophy):** Consistent. RFC-0001 is mechanism-free by construction (§0), so it imposes no language constraint to violate. The ADR's honesty about its primary cost (naming Rust the strongest alternative, stating the type-system loss plainly) actively *upholds* RFC value V5 ("honest about tradeoffs"). The contributor-ramp justification (R3) is coherent with RFC goal 7. No conflict.

**ARCHITECTURE.md:** Consistent on the central decision (§5.4 recommends Go; the ADR ratifies it) and reinforcing on the single-static-binary requirement (the ADR adds the `CGO_ENABLED=0` toolchain clause that §5.4 left implicit — a genuine strengthening). **One inconsistency**, see Finding F1: ADR-0001's justification asserts a plugin-isolation direction (gRPC-subprocess primary, WASM as a hardening track) that runs against `ARCHITECTURE.md §15.2` ("WASM component model preferred; subprocess … as fallback").

**IMPLEMENTATION-ROADMAP.md:** Consistent. Roadmap PR-001 is gated on this ADR; roadmap §8 already names the pure-Go SQLite expectation and the CGO concern; the ADR formalizes both. No conflict.

**Previous board reviews:** Consistent with `pre-implementation-adr-gate.md` (ADR-0001 = language, P0, blocks PR-001). Partially in tension with `RFC-0001-review-round-1.md` P0-2 (no governance exists to confer "Accepted") — see F6. Note the gate review is itself the *source* of the gRPC-primary lean in F1; the divergence from `ARCHITECTURE.md §15.2` was introduced there and never reconciled by an amendment, and ADR-0001 inherits it.

---

## 5. Hidden assumptions

- **A1 — A pure-Go embedded storage *and vector* stack exists and suffices.** The "Easier" section asserts pure-Go SQLite as settled fact. Pure-Go SQLite exists, but the architecture's own recommended vector approach (sqlite-vec class, `ARCHITECTURE.md §9`/gate-review ADR-0007) is a **C extension**. The ADR assumes the embeddings problem is the only cgo pressure; the *vector index itself* is a second, unnamed one. **(P2)**
- **A2 — "Pure, well-tested domain layer" ⇒ portable across a language rewrite.** The Migration Strategy leans on this for reversibility. Tests-as-specs are portable; **Go domain *code* is not mechanically portable**. The reversibility mitigation is weaker than implied. **(P3)**
- **A3 — Go's open governance remains neutral/stable for a decade.** Betting the kernel on a single-vendor-stewarded language is an unstated assumption. Faint relative to V6 (which is model-provider-scoped), but unacknowledged. **(P3)**
- **A4 — Node "fails R1 outright."** The alternatives analysis predates modern single-executable compilers (Deno/Bun `--compile`). The *conclusion* (reject on R4 cold-start/memory + dynamic typing) still holds; the *premise* ("no genuine single binary") is overstated. Correctness of reasoning, not of outcome. **(P3)**

---

## 6. Architectural contradictions

- **F1 (P1) — The ADR pre-decides ADR-0008's isolation mechanism, against ARCHITECTURE §15.2.** In "Harder" and "Future ADR Dependencies," the ADR treats "gRPC-subprocess primary, WASM hardening track" as a *settled premise* and reasons from it ("this ADR makes that ordering harder to invert"). That outcome (a) belongs to ADR-0008 (see F4), and (b) contradicts the stated preference in `ARCHITECTURE.md §15.2`. The least-reversible decision in the project is thereby coupled, in its own justification, to an un-made and contrary downstream decision. The *language* decision does not actually depend on that premise — so the coupling is gratuitous as well as contradictory.
- **F2 (P2) — `CGO_ENABLED=0` default vs the architecture's own knowledge/semantic stack.** Combined with A1, the pure-Go default build is on a collision course with both on-device embeddings *and* the vector index ADR-0007 is expected to choose. The ADR acknowledges the embedding half ("ADR-0007 must resolve") but not the vector-index half, so the contradiction is only partially surfaced.

No contradiction found with RFC-0001 or the roadmap.

---

## 7. Irreversible decisions not adequately acknowledged

- **F3 (P2) — The pure-Go *distribution model* is itself sticky, separately from the language.** The ADR rates the *language* low-reversibility (correct) and frames CGO as an open question reversible "behind a build tag." But once install/distribution (single universal `curl | sh` artifact) and packagers depend on one pure-Go binary, moving to platform-specific cgo builds fragments the distribution matrix — a moderate irreversibility the ADR does not name. The language reversibility is acknowledged; the *toolchain-policy* reversibility is not.

The module/import-path irreversibility (post-external-importer) *is* acknowledged — good.

---

## 8. Decisions that belong in another ADR

- **F4 (P1, same root as F1) — Plugin isolation mechanism belongs to ADR-0008.** ADR-0001 should record the *obligation* it creates (keep the boundary language-agnostic — clause 4, correctly placed) but must not assert the *mechanism* (gRPC vs WASM, primary vs fallback). Asserting it here pre-empts ADR-0008 and imports the F1 contradiction.
- **F5 (P3) — The optional "full build with cgo features" question is build/release policy.** The ADR correctly flags a possible separate supply-chain/build ADR; the operational CGO-behind-build-tags detail (Open Question 1) leans toward that future ADR's territory. Acceptable to anchor the *principle* here; flag so it does not accrete build-policy scope.

The pure-Go constraint imposed on ADR-0002 is *correctly* placed (it is a consequence/dependency, not a foreign decision).

---

## 9. Consequences omitted

- **F2/A1** — vector-index cgo pressure (above).
- **F7 (P3) — Dependency/supply-chain surface.** Choosing Go's module ecosystem for a tool that holds credentials (RFC trust thesis; `ARCHITECTURE.md §17`) expands the supply-chain attack surface. The ADR mentions a *possible future ADR* but does not list this as a *consequence* of the choice.
- **F8 (P3) — Baseline binary size / IDE-client language split.** Go static binaries (plus embedded SQLite, later embedded models) are large — a distribution-UX consequence not noted. Relatedly, the eventual IDE/editor clients (RFC/ARCH) will not be Go and must speak to the daemon over its API — a consequence for M5 that is unstated (though benign, since the daemon API is the intended boundary).

---

## 10. Philosophy violations

- None substantive. The ADR is well-aligned with RFC §6.8 ("boring where it counts" — a conservative, mainstream language) and V5 (honesty).
- **F6 (P2) — `Status: Accepted` rests on authority that does not yet exist.** `RFC-0001-review-round-1.md` P0-2 establishes that no ratification process is defined. The ADR honestly flags this ("interim board authority"), which keeps it consistent with V5 — but a status field whose conferring process is undefined is procedurally unfounded. This is process more than architecture, hence P2, and the honest acknowledgment is a mitigating factor. It must be reconciled when governance lands (the ADR's own checklist already carries this).

---

## Findings summary

| ID | Finding | Class |
|---|---|---|
| F1 | Pre-decides ADR-0008 isolation mechanism; contradicts ARCH §15.2 | **P1** |
| F4 | Isolation-mechanism decision belongs in ADR-0008 (same root as F1) | **P1** |
| F2 | CGO=0 default vs vector index / embeddings; only half-acknowledged | P2 |
| F3 | Irreversibility of the pure-Go distribution model not named | P2 |
| F6 | "Accepted" status rests on undefined governance (acknowledged) | P2 |
| A1 | Assumes pure-Go vector stack; sqlite-vec is C | P2 |
| A2 | Domain-purity ⇒ cross-language portability overstated | P3 |
| A3 | Assumes decade-stable neutral Go governance | P3 |
| A4 | Node "fails R1 outright" overstated (Deno/Bun compile) | P3 |
| F5 | Optional cgo "full build" leans into build-policy ADR | P3 |
| F7 | Supply-chain consequence of Go module ecosystem omitted | P3 |
| F8 | Binary size / non-Go IDE-client consequence omitted | P3 |

**No P0 (project-blocking) findings.** The Go decision is architecturally correct, internally coherent, consistent with the accepted set on its core claim, and sufficient to unblock PR-001.

---

## Verdict

### ACCEPT WITH CHANGES

**Why not ACCEPT:** Two coupled P1 findings (F1/F4) put the *least-reversible decision in the project* into contradiction with an accepted document (`ARCHITECTURE.md §15.2`) and pre-empt a downstream ADR — within the justification of an ADR that does not need that premise. An ADR that reasons from an un-ratified, doc-contradicting assumption is not yet safe to finalize, even though the conclusion it protects (Go) is correct. The change is bounded: the language decision must stop asserting the plugin-isolation outcome and must either confine itself to the language-agnostic *obligation* (clause 4, which is correct) or explicitly flag the §15.2 tension as unresolved and defer it to ADR-0008.

**Why not REJECT:** The decision is right, the requirement-driven alternatives analysis is decision-grade and honest, the static-binary toolchain clause is a real strengthening over §5.4, and nothing here blocks PR-001 from a language standpoint. Rejecting a correct, well-reasoned core decision over advisory-section over-reach would be disproportionate.

**Conditions for promotion to ACCEPT (architectural only):**
1. **F1/F4** — remove the assertion of ADR-0008's isolation mechanism from this ADR's justification, or explicitly mark it as the unresolved tension with ARCH §15.2 that it is. The language decision must not depend on it.
2. **F2/A1** — extend the acknowledged cgo tension to include the vector index, not only embeddings, so ADR-0007 inherits the full constraint.
3. **F3** — acknowledge the distribution-model irreversibility distinct from the language irreversibility.

P2/P3 items are recommended, not gating. **PR-001 is not blocked** by these conditions; they gate the ADR's promotion from provisional to final, which should happen before ADR-0008 begins (so the F1 contradiction is not propagated forward).

---

_The board endorses Go. It declines to finalize an ADR that smuggles a contrary, not-yet-made plugin decision into the justification of the project's most permanent choice. Fix the coupling; the decision stands._
