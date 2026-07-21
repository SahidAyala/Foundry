# ADR-0006 — Routing & Policy

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-20 under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s governance process. Originally drafted per [RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §4.4/§7 and [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.1–§2.3/§2.7. |
| **Date** | Drafted 2026-07-20; ratified 2026-07-20 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted per RFC-0002/RFC-0004 |
| **Ratifies** | The ADR backlog entry named in [../03-adrs/README.md](README.md) ("Routing & policy") — placement policy and failover over Capabilities. |
| **Gates** | RFC-0002 §7's three-layer routing design (explicit pin, negotiation, failover) and RFC-0004 §2.1–§2.3's concrete Phase 6 shape. Also the future-ADR dependency [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) explicitly named for itself ("if capability-based negotiation is ever built, that ADR must explicitly revisit whether a claim needs verifying") and the one [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 7 named ("must define their semantics... rather than reopening document versioning to do so"). |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. RFC-0002 and RFC-0004, the RFCs this ADR was drafted from, remain Draft — Proposed in their own right; ratifying this ADR does not ratify either RFC. |

---

## Context

Three already-accepted documents each named this ADR as the place a specific deferral gets revisited, rather than deciding it themselves:

- **[ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 7**: "The Router-reserved fields (`capability`, `executor`, `feeds_forward`, `target`) keep their current optional, unexercised status. Their semantics are out of scope here and belong to Routing & policy (backlog, proposed ADR-0006)." Its Future ADR Dependencies section adds: this ADR "must define their semantics within Decision 3's additive-only discipline rather than reopening document versioning to do so" — i.e., this ADR owns *what the Router does* with these fields, not whether their JSON shape can still evolve (ADR-0004 already owns that).
- **[ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 4**: "Capability truthfulness is declared, not verified, for the lifetime of explicit-pin-only routing... nothing consumes a Capability for matching yet." Its Future ADR Dependencies section adds: "If capability-based negotiation... is ever built, that ADR must explicitly revisit whether a claim needs verifying, rather than silently carrying this ADR's deferral forward." That revisit has not happened until this document.
- **[RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §7** names three layers, cheapest-first: (1) explicit pin — "the only layer built in Phase 6"; (2) capability-based negotiation — "built only once ≥2 real Executors advertise the same Capability in production"; (3) failover — an availability fallback, "recorded as Evidence when it fires." **[RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.1–§2.3** makes layer 1 concrete: a Step's optional `capability`/`executor` fields, a project-local `.foundry/executors.json`, and an `engine.Router`/`engine.ExecutorRegistry` pair resolving "explicit pin, or default" — "nothing more."

**What is already built, and this ADR ratifies rather than invents:**

- `engine.Router.Resolve(step)` (`engine/router.go`): returns the Step's pinned `Executor` (looked up in an `ExecutorRegistry` by name) if `step.Executor` is set — a clear, named error if the pin doesn't resolve — or the Engine's own default `Executor` otherwise. This is the entirety of the routing policy in the codebase today; there is no other code path that selects an `Executor`.
- `engine.ExecutorRegistry` (`engine/executor_registry.go`): register-once, look-up-by-name, mirroring `PipelineRegistry`'s shape — deliberately named to avoid the retired term "Provider" (terminology.md).
- `engine.Step.Capability map[string]string`, `.Executor string`, `.FeedsForward bool` (`engine/step.go`), and their matching optional, `omitempty` JSON fields on `PipelineDocument` (`engine/document.go`) — all additive, all zero-valued by default. `Capability` is carried but read by nothing: no matching or negotiation logic exists anywhere in the tree.
- `feeds_forward` is fully implemented (`engine/strategy.go`): when set, it appends the immediately-preceding Step's recorded output to the following Step's Context exactly once — never an arbitrarily named earlier Step, a deferral RFC-0004 §3 named and this ADR reaffirms rather than reopens.
- Per-Step Budget accounting (`engine/cost_estimator.go`, `engine/budget.go`) — RFC-0004 §2.7 / the multi-executor-router-implementation-plan's Piece 5 — is also fully shipped: `estimateExecuteCostUSD` type-asserts an `Executor` for the optional `CostEstimator` interface ADR-0005 Decision 3 defined, falling back to a flat estimate otherwise.
- `.foundry/executors.json` (`project.LoadExecutorConfig`, `project.BuildExecutorRegistry`) and two real vendor Executors (`executor/claude`, `executor/openai`) exist and are wired at both composition roots (`cmd/foundry/commands/do.go`, `session.Session`).

**What genuinely has not been built, confirmed by this audit:** no shipped Pipeline document (`default`, `review`, `feature`, `bugfix`, `release`) declares a `capability` or `executor` value — every one of the five runs against whichever single `Executor` the process/session default provides, exactly as before Piece 1 existed. No code anywhere matches a Step's `Capability` against more than one candidate `Executor`, weighs cost/latency/quality/privacy, or fails over from one `Executor` to another. RFC-0002 §7 layer 2 and layer 3 are, today, pure design, not code — matching every implementation-status.md audit's repeated statement that "today's Router is explicit-pin-only."

**A live defect this ADR also fixes, not just describes:** [README.md](README.md)'s own backlog table names this row's "Gates" column "Provider independence" — the retired term, used as live vocabulary in an active doc, exactly the defect AGENTS.md's historical-isolation guarantee #1 names. This ADR corrects it to "Vendor independence" in the same ratifying commit.

This ADR does not resolve **Knowledge & semantic store** (backlog, ADR-0007), **Extension isolation & contract versioning** (backlog, ADR-0008), or **Cost as a first-class constraint** (backlog, ADR-0011) — a third-party Executor's registration mechanism and whether cost ever becomes a mandatory or negotiation-weighted input are those ADRs' questions, not this one's.

---

## Decision

1. **Today's explicit-pin Router is ratified as Foundry's entire routing policy.** `engine.Router.Resolve`'s existing behavior — a Step's `Executor` pin, looked up by name in an `ExecutorRegistry`, or the Engine's default `Executor` when unset — is confirmed as the correct, current implementation of RFC-0002 §7 layer 1 / RFC-0004 §2.1–§2.2's Phase 6 design. This promotes an already-shipped mechanism into a governed decision under [ADR-0000](ADR-0000-governance-and-ratification-process.md), the same move [ADR-0009](ADR-0009-cli-and-output-contract.md) Decision 1 made for the interactive session.

2. **`Capability` remains declared, advisory metadata only — read by nothing.** A Step's `capability` tag and an `.foundry/executors.json` entry both stay free-form, human-authored data with no matching or negotiation logic consuming them. This is not a gap to close now; it is RFC-0002 §7's own explicit sequencing (layer 1 before layer 2), reaffirmed rather than reconsidered.

3. **Capability-based negotiation (RFC-0002 §7 layer 2) remains explicitly deferred — not built.** Its own named trigger — "once ≥2 real Executors advertise the same Capability in production" — has not fired: exactly two real vendor Executors exist (`executor/claude`, `executor/openai`), and no shipped Pipeline document declares two Executors competing for one Capability tag. This ADR does not build a cost/latency/quality/privacy weighting policy speculatively; the trigger, not a calendar date or this ADR's existence, decides when it gets built.

4. **Failover (RFC-0002 §7 layer 3) remains deferred alongside negotiation, for the same reason, and is built together with it rather than separately.** An availability fallback chain has no real shape to take when there is no second candidate `Executor` per Capability to fail over *to* — RFC-0002 §7 already sequences failover after negotiation, and this ADR does not reorder that.

5. **Capability truthfulness stays declared, not verified — [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 4 is reaffirmed, not silently inherited.** This is the actual walk-through ADR-0005's own Future ADR Dependencies section required ("must explicitly revisit whether a claim needs verifying"): since negotiation (Decision 3) is still not built, verifying a property nothing reads remains exactly the premature complexity ADR-0005 already rejected once. This deferral is re-examined again only alongside Decision 3, not before.

6. **The Router-reserved Step fields' policy semantics are exactly what is already implemented — closing [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 7's deferral.** `Capability` is a free-form `map[string]string` role/property tag with no closed vocabulary; `Executor` is an explicit name pinned against a project's `ExecutorRegistry`; `FeedsForward` threads only the immediately-preceding Step's output, never an arbitrarily named earlier Step (RFC-0004 §3's own naming-scope deferral, also reaffirmed here, not reopened). No new field and no change to any of these three is introduced. This decision is about *behavior*, not the JSON schema's own evolution policy — that stays ADR-0004's, unchanged.

7. **Routing & policy is not a sixth named compatibility surface in [release.md](../04-guides/release.md).** Its two halves are already each owned by an existing surface: the Router-reserved fields' schema-evolution policy is the reusable-Act template schema ([ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md)); the Router's and `ExecutorRegistry`'s own behavior is the Executor/extension contracts ([ADR-0005](ADR-0005-executor-contract-and-capability-model.md)). Adding a third named surface for the same concern would fragment one thing across three release-guide entries rather than clarify it; release.md is not edited by this ADR.

8. **Today's Router design keeps no vendor privileged, ratifying I12 ("the model is substrate") for the multi-Executor case.** `Router.Resolve` selects by explicit name against an `ExecutorRegistry`, never by vendor identity, adapter shape, or invocation mechanism (subprocess vs. HTTP, per [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 2) — a third vendor Executor requires zero `Router` or `Engine` changes, only a new adapter package and a registry entry. [README.md](README.md)'s own backlog table is corrected from "Provider independence" to **"Vendor independence"** in the same commit, retiring the last live use of that word in this row (a defect independent of, but surfaced by, drafting this ADR).

---

## Alternatives Considered

### Build capability-based negotiation now, since the schema already exists
- **For:** `Capability` is already a real field on every Step and in `.foundry/executors.json`; the schema cost is already paid.
- **Against:** A schema field existing is not the same as a real consumer needing it. No shipped Pipeline declares two Executors competing for one Capability — building a cost/latency/quality/privacy weighting policy against zero real cases is exactly the speculative-generality trap [ADR-0009](ADR-0009-cli-and-output-contract.md) avoided for `--json`, and RFC-0002 §7 itself already named the correct trigger.
- **Verdict:** Rejected. Ratified instead as Decision 3 — deferred until its own named trigger fires.

### Verify Capability claims via a self-test or registration-time handshake
- **For:** Would catch a misconfigured `.foundry/executors.json` entry (wrong model, expired credential, an honest mistake about what a vendor can do) before an Act ever runs against it.
- **Against:** This is the literal alternative [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) already rejected ("verifying a property nothing reads is complexity paid for with no consumer"), and its own precondition for reconsidering it — negotiation existing — still has not occurred. Reopening it now, with nothing new to justify reversing that call, would silently re-litigate an already-reasoned decision instead of confirming it still holds.
- **Verdict:** Rejected. Ratified instead as Decision 5, explicitly re-confirming ADR-0005 Decision 4 rather than silently inheriting it unexamined.

### Build failover independently, ahead of negotiation
- **For:** Availability (an Executor being unreachable) is a real, distinct concern from placement quality (which Executor is the *best* match) — a narrower failover-only mechanism could ship without a full weighting policy.
- **Against:** RFC-0002 §7 sequences failover as layer 3, strictly after negotiation (layer 2), because a fallback chain needs ≥2 candidate Executors per Capability to fail over between — exactly the condition negotiation's own trigger already tracks. Building failover first would mean designing "which Executor is next" logic in isolation, then re-deriving it once negotiation exists anyway.
- **Verdict:** Rejected. Ratified instead as Decision 4 — deferred alongside negotiation, built together.

### Give Routing & policy its own new compatibility-surface entry in release.md
- **For:** Explicit is better than implicit; a reader of release.md today has no single line pointing at "routing" by name.
- **Against:** Its two real components — the Pipeline-document schema fields, and the Router/Executor behavior — are already each owned by name (ADR-0004, ADR-0005). A third entry would describe the same underlying commitments a second time under a new heading, contradicting AGENTS.md's "one concept, one document" rule applied to compatibility surfaces specifically.
- **Verdict:** Rejected. Ratified instead as Decision 7 — release.md is not edited.

---

## Consequences

### What this decision makes EASIER
- **RFC-0002 §7 and RFC-0004 §2.1–§2.3's routing design finally has a governed answer**, rather than remaining informally "explicit-pin-only, for now" with no ADR backing it.
- **[ADR-0005](ADR-0005-executor-contract-and-capability-model.md)'s own named re-examination ("must explicitly revisit... Decision 4") is actually walked through** and answered (Decision 5), rather than silently inherited forever by omission.
- **[ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 7's deferral is closed** (Decision 6): the Router-reserved fields' behavior is now decided, not merely "reserved."
- **A real, live defect (README.md's "Provider independence") is fixed**, rather than persisting as background noise the next reader has to rediscover.
- **The backlog shrinks by one row** with zero code changes required — this ADR ratifies and declines, it does not build.

### What this decision makes HARDER
- **Nothing structurally.** This ADR changes no code and adds no new obligation; it confirms existing behavior and declines speculative work that was already correctly deferred by two prior RFCs.
- **The deferred decisions (negotiation, failover, capability verification) remain genuinely undesigned** — a future ADR (or an amendment to this one) still has to design a real weighting policy and failover chain from scratch once the trigger fires; this ADR does not pre-sketch that design, deliberately, per the same depth-before-breadth discipline RFC-0002 §7 itself uses.

### Reversibility
High. Decisions 1, 2, 6, and 8 ratify already-shipped, already-tested behavior — nothing to unwind. Decisions 3, 4, 5, and 7 decline to build something or add a document section; each is free to be revisited the moment its own named trigger fires, with no sunk cost from this ADR itself.

---

## Migration Strategy

No code changes. This ADR ratifies `engine.Router`, `engine.ExecutorRegistry`, `engine.Step`'s Capability/Executor/FeedsForward fields, and `.foundry/executors.json` exactly as already implemented and tested.

1. Correct [README.md](README.md)'s backlog table: "Provider independence" → "Vendor independence" (Decision 8), in the same commit that moves this row to the Accepted table.
2. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR and RFC-0002/RFC-0004 rows to record this ADR's ratification.

No data migration; no change to any Act, Pipeline, or Record format.

---

## Future ADR Dependencies

- **Cost as a first-class constraint** (backlog, ADR-0011): inherits this ADR's confirmation that per-Step `CostEstimator`-driven charging (already shipped, RFC-0004 §2.7) is the seam Router placement would read from — must decide whether cost becomes a negotiation input once Decision 3's trigger fires, not before.
- **Extension isolation & contract versioning** (backlog, ADR-0008): a third-party Executor extension still registers into the same `ExecutorRegistry` Decision 1 ratifies (Decision 8: no vendor is privileged) — that ADR decides *how* an out-of-process extension registers, not whether the Router itself changes to accommodate one.
- **Whichever future ADR builds capability-based negotiation** (Decision 3's trigger): inherits Decision 5's "declared, not verified" posture as its own starting assumption, and must explicitly re-examine it in turn, exactly as this ADR was required to re-examine ADR-0005 Decision 4 — a chain of explicit re-examinations, never a silent one.

---

## Open Questions

1. **What concrete signal counts as "a real multi-Executor Pipeline in production"** triggering negotiation (Decision 3)? Left for whoever builds it to judge against the actual Pipeline in question — not pre-specified here, the same discipline RFC-0002 §7 already applied.
2. **Should `Capability`'s free-form `map[string]string` shape change once negotiation is built** (e.g., a closed vocabulary of recognized keys, replacing today's anything-goes tags)? Deferred to whichever future ADR actually builds negotiation, per Decision 2.
3. **Is a literal GitHub Copilot adapter ever in scope**, versus the mechanism being proven and used with Claude Code plus one clean-API vendor indefinitely? [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)'s own Open Question 2 already left this open; this ADR does not resolve it either.

---

## Review Checklist

Walked through at ratification (2026-07-20):

- [x] **No contradiction with accepted documents.** Confirmed: [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 7's deferral is closed by Decision 6 here, without reopening document versioning; [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 4 is reaffirmed by Decision 5 here, not contradicted; [ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) has no overlap — VCS/PR's own `remote-pr` target is orthogonal to Executor routing.
- [x] **Decisions 1, 2, 6, and 8 verified against the actual shipped code.** Re-read at ratification: `engine/router.go` (`Router.Resolve` — pin or default, nothing else), `engine/executor_registry.go` (register-once/look-up-by-name), `engine/step.go` (`Capability map[string]string`, `Executor string`, `FeedsForward bool`, all zero-valued by default), `engine/strategy.go`'s `feeds_forward` handling (`appendFeedsForward`, immediate-predecessor only), `engine/cost_estimator.go` (optional `CostEstimator` type-assertion), `project/executor_config.go` (`.foundry/executors.json` decoding) — all match this ADR's description exactly; nothing had drifted since drafting.
- [x] **No shipped Pipeline document contradicts Decision 3's "no real multi-Executor-per-Capability case exists" claim.** Re-verified at ratification: none of the five (`default`, `review`, `feature`, `bugfix`, `release`) declare a `capability` or `executor` field.
- [x] **README.md's "Provider independence" correction (Decision 8) is applied** in this same ratifying commit, alongside the Status row and backlog-table move.
- [x] **Process caveat resolved.** Ratified under [ADR-0000](ADR-0000-governance-and-ratification-process.md); Status row above and the backlog table in [README.md](README.md) updated in the same ratifying commit.

---

_This ADR ratifies today's explicit-pin-only Router, `ExecutorRegistry`, and Router-reserved Step fields exactly as already shipped; formally re-examines and reaffirms ADR-0005 Decision 4's "capability declared, not verified" posture per that ADR's own named requirement to do so; explicitly declines to build capability-based negotiation or failover before either's own named trigger fires; declines to add Routing & policy as a new named compatibility surface in release.md, since its two halves are already owned by ADR-0004 and ADR-0005; and fixes one live, previously-unnoticed use of the retired term "Provider" in README.md's own backlog table._
