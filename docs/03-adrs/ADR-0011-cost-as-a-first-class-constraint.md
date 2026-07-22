# ADR-0011 — Cost as a First-Class Constraint

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-21 under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s governance process, the same day it was drafted. |
| **Date** | Drafted 2026-07-21; ratified 2026-07-21 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | The ADR backlog entry named in [README.md](README.md) ("Cost as a first-class constraint") — whether `CostEstimator` ([ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 3) ever becomes mandatory, whether an Executor may report *actual* post-execution cost alongside today's pre-execution estimate, and whether `engine/budget.go`'s hardcoded ceilings should become project-configurable now. |
| **Gates** | [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)'s Open Question 1 ("Should `CostEstimator` become mandatory once ≥2 real vendor Executors ship in-tree?"); [roadmap.md](../00-overview/roadmap.md) open decision 9's cost half ("Cost as a first-class constraint"); `engine/budget.go`'s own comment ("Hardcoded until budgets become configurable, roadmap.md open decision 9"). |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. Unlike [ADR-0007](ADR-0007-knowledge-and-semantic-store.md) (which ratified already-shipped behavior with no code changes), Decision 2 below proposes a small, genuinely new, additive domain change — see Migration Strategy. |

---

## Context

**What is already built, and most of this ADR ratifies rather than invents:**

- `engine.CostEstimator` (`engine/cost_estimator.go`, [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 3): an optional, type-asserted Executor capability. `estimateExecuteCostUSD` calls it when present, falling back to the flat `executeCostEstimateUSD` constant ($0.50) otherwise.
- `executor/openai.Executor` implements it with a real per-model pricing table (`costPerMillionTokensUSD`) over a characters-per-token heuristic — a genuine, if approximate, **pre-execution** estimate.
- `executor/claude.ClaudeExecutor` does **not** implement it: the Claude Code CLI subprocess exposes no cost signal at all (confirmed — no `cost`/`usage` handling anywhere in `executor/claude`), so it uses the flat fallback.
- `engine/budget.go`'s `tracker.charge`: hardcoded `defaultMaxIterations = 4`, `defaultMaxCostUSD = $2.00`, enforced **per Step, not per attempt** ([RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.7, shipped). `domain.Budget`'s own doc comment is explicit: "enforced as a constraint, never merely reported as a metric" — the ceiling must gate, not just log.
- `act.CostEstimateUSD` (`domain.Act`) is a running total of every charged **pre-execution estimate** across the Act's whole lifetime — never reconciled against what a call actually cost.

**A real, concrete gap this ADR names for the first time:** `executor/openai.Executor.Execute` (`executor/openai/openai.go:137`) calls OpenAI's Chat Completions API and decodes only `choices[].message.content` from the response (`chatCompletionResponse`, line 119) — it does not decode the response's own `usage` object (`prompt_tokens`, `completion_tokens`, `total_tokens`), which OpenAI's API already returns on every call at no extra cost. Foundry today has no path for an Executor to report what a call *actually* cost, even when that data is sitting unused in a response it already received. `act.CostEstimateUSD` is therefore permanently an estimate — accurate enough to gate spend before the fact, but never checked against reality after it.

**The trigger [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Open Question 1 named has technically fired, but the substantive question is unchanged:** two real vendor Executors now ship in-tree (`executor/claude`, `executor/openai`), the condition that ADR named for revisiting whether `CostEstimator` should become mandatory. But the reason `ClaudeExecutor` doesn't implement it was never "nobody got to it yet" — it is that the Claude Code CLI subprocess has no billing API to read from. Two vendors existing does not create a signal the second one structurally cannot produce.

This ADR does not resolve [roadmap.md](../00-overview/roadmap.md) open decision 9's other, adjacent half — "**near-term single-user value** vs the long compounding bet" — a product-strategy question about Foundry's overall value proposition, not a cost-mechanism question this ADR's scope covers. Naming it here so it is not silently conflated with the decisions below, not deciding it.

---

## Decision

1. **`CostEstimator` remains optional, not mandatory — closing [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Open Question 1 with a firm "no," not a further deferral.** The Claude Code CLI subprocess has no per-call billing signal to report; mandating the interface would force `ClaudeExecutor` to either fabricate a number or break compilation, for a vendor where no real number exists. `executor.ScriptedExecutor` (the test double) has the same structural non-answer. Revisit only if `ClaudeExecutor`'s own invocation shape changes in a way that exposes real cost data (e.g. Claude Code's CLI itself starts reporting token usage) — not on Executor count alone.

2. **An Executor may optionally report the *actual* cost of a completed `Execute` call, distinct from and additional to `CostEstimator`'s pre-execution estimate.** `domain.Outcome` gains one new optional field:

   ```go
   type Outcome struct {
       Patch string
       // ActualCostUSD is the real, post-execution cost of the Execute call
       // that produced this Outcome, if the Executor can report one — nil
       // when it cannot (e.g. executor/claude.ClaudeExecutor, whose
       // subprocess exposes no billing signal). Never used to gate Budget
       // (domain.Budget's own doc comment: enforced as a constraint, not
       // merely reported) — CostEstimator's pre-execution estimate remains
       // the sole enforcement signal, for the obvious reason that spend
       // must be bounded before it happens, not after.
       ActualCostUSD *float64
   }
   ```

   `executor/openai.Executor.Execute` is extended to decode the Chat Completions response's existing `usage.prompt_tokens`/`usage.completion_tokens` fields (data it already receives and currently discards) and populate `ActualCostUSD` from the same per-model price table `EstimateCostUSD` already uses. `executor/claude.ClaudeExecutor` and `executor.ScriptedExecutor` are unchanged — both continue to return `Outcome{Patch: ...}` with `ActualCostUSD` left nil, exactly [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 3's own "zero forced change" discipline.

3. **`domain.StepRecord` and `domain.Act` each gain a matching optional accumulator, purely for reporting — never for enforcement.** `StepRecord.ActualCostUSD *float64` records one generate Step's own value (nil if its Executor didn't report one). `Act.ActualCostUSD *float64` sums every non-nil `StepRecord.ActualCostUSD` across the Act; `Act.ActualCostStepCount`/`Act.GenerateStepCount`-style bookkeeping (exact shape left to implementation) lets a caller honestly render "actual cost reported for N of M generate Steps" rather than silently implying a total that may be partial. This is Evidence in the sense [trust.md](../02-architecture/trust.md) already uses the word — additional, attributable, honestly-scoped data about what happened — never a second Budget gate.

4. **`engine/budget.go`'s hardcoded ceilings (`defaultMaxIterations = 4`, `defaultMaxCostUSD = $2.00`) are ratified as sufficient for now — not made project-configurable in this ADR.** No real project has yet hit either ceiling while attempting legitimate work (the dogfooding log in [implementation-status.md](../00-overview/implementation-status.md) §7 shows no such case). Making these configurable (e.g. via `.foundry/config.json`, following the exact convention `docs_path`/`require_approval_before_remote_publish` already established) is a small, well-precedented change *when* a real project needs a different ceiling — not designed speculatively here. The named trigger: a legitimate multi-Executor Pipeline's genuine work is refused by the current ceiling.

5. **No cross-vendor cost reconciliation, alerting, or divergence detection is built.** If a future Act's `ActualCostUSD` total diverges materially from its `CostEstimateUSD`, nothing compares them, flags a mismatch, or adjusts future estimates — this ADR only makes the actual number visible (`foundry show`, the structured log stream shipped for M5), the same "surface honest data, do not build the analysis on top of it speculatively" posture [ADR-0003](ADR-0003-replay-and-determinism-contract.md) Decision 4 already took for replay-divergence classification.

---

## Alternatives Considered

### Make `CostEstimator` mandatory now that two real vendor Executors exist
- **For:** Guarantees every future Executor contributes a real cost signal to Budget's accounting, closing ADR-0005's own named open question definitively in the other direction.
- **Against:** `ClaudeExecutor` cannot satisfy this — not "hasn't yet," but structurally, since its subprocess exposes no billing API. Mandating it would either force a fabricated, meaningless number (dishonest) or break the default Executor's compilation (a real regression). The Executor *count* triggering ADR-0005 OQ1 does not change what the *specific* second Executor can report.
- **Verdict:** Rejected. Ratified instead as Decision 1 — a firm, reasoned "no," not merely "not yet."

### Reconcile actual vs. estimated cost by adjusting future estimates (a learning/calibration loop)
- **For:** Would make `EstimateCostUSD`'s pre-execution numbers more accurate over time, for vendors where a real signal exists.
- **Against:** No consumer needs this yet — nothing today reads a divergence between estimate and actual, and today's flat pre-execution estimate is only ever used to decide *whether a call fits under the ceiling*, not to communicate precise expected spend. Building a calibration loop before anyone has observed a real, material divergence is exactly the premature-generality pattern this project's prior ADRs consistently decline.
- **Verdict:** Rejected for now. Ratified instead as Decision 5 — surface the number, do not act on it speculatively.

### Make Budget ceilings project-configurable now, via `.foundry/config.json`
- **For:** `engine/budget.go`'s own comment already anticipated this; a multi-Executor Pipeline plausibly needs a higher ceiling than a single-Executor one.
- **Against:** No real project has hit the current ceiling while attempting legitimate work. Building configurability ahead of a concrete, named case that needs it is the same "no consumer, no schema" trap [ADR-0006](ADR-0006-routing-and-policy.md) and [ADR-0009](ADR-0009-cli-and-output-contract.md) each already declined for their own surfaces.
- **Verdict:** Rejected for now. Ratified instead as Decision 4 — a named trigger, not a speculative config surface.

### Report actual cost via a second optional Executor interface (mirroring `CostEstimator`'s shape) instead of an `Outcome` field
- **For:** Consistent with `CostEstimator`'s own "optional, type-asserted capability" pattern (ADR-0005 Decision 3).
- **Against:** Actual cost is known only *after* `Execute` returns, and an interface called post-hoc (e.g. `Executor.LastActualCostUSD()`) would require an Executor to retain mutable state between calls — a real correctness risk, since `session.Session` reuses one Executor instance across an entire session's lifetime and multiple Acts may run against the same Executor. A field on the `Outcome` that call already returns is per-call, inherently race-free, and requires no new interface at all.
- **Verdict:** Rejected. Ratified instead as Decision 2 — a plain field on `Outcome`, the simpler and safer shape.

---

## Consequences

### What this decision makes EASIER
- **[ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Open Question 1 is closed** with a reasoned, permanent answer grounded in a real constraint (Claude Code's subprocess has no billing signal), not left open indefinitely.
- **Real spend becomes visible for the first time** for any Executor that can report it (`openai`) — `foundry show`'s per-Step trace (shipped for M5) and the structured `FOUNDRY_LOG` event stream (also shipped for M5) both gain a natural place to surface `ActualCostUSD` once implemented, without either of those M5 mechanisms needing to change shape.
- **[roadmap.md](../00-overview/roadmap.md) open decision 9's cost half is closed**, cleanly separated from its adjacent, unresolved product-strategy half.

### What this decision makes HARDER
- **Nothing structurally** — Decision 1 declines new work with a firm, reasoned answer; Decisions 4–5 decline speculative work with named triggers; only Decision 2–3 add a small, genuinely additive field, which changes nothing for any Executor that leaves it nil.
- **`domain.Outcome`, `domain.StepRecord`, and `domain.Act` each grow one more optional field** — a real, if small, addition to the core domain's surface area, unlike every prior ADR's Migration Strategy in this series (which required no code at all). Named honestly, not hidden in "no code changes."
- **A partial actual-cost total (some Steps reported one, some didn't) is a real presentation problem** Decision 3 requires solving honestly (show "N of M Steps"), not one this ADR gets to wave away.

### Reversibility
High for Decisions 1, 4, and 5 (reasoned declines, nothing built to unwind). Medium for Decisions 2–3: additive optional fields are cheap to add and cheap to ignore, but once `act.json` files exist in the wild carrying a populated `ActualCostUSD`, removing the field later is a real (if small) compatibility consideration — mitigated by [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) Decision 8's existing "no version field, additive-only evolution" posture, which already covers exactly this kind of addition.

---

## Migration Strategy

Unlike [ADR-0007](ADR-0007-knowledge-and-semantic-store.md), this ADR requires real code changes, sequenced as a follow-up implementation PR once ratified:

1. Add `Outcome.ActualCostUSD *float64` (`domain/act.go`).
2. Add `StepRecord.ActualCostUSD *float64` and `Act.ActualCostUSD *float64` (`domain/act.go`), plus whatever minimal bookkeeping Decision 3 needs to report "N of M" honestly (e.g. a computed helper rather than a stored count, to avoid a fourth new stored field for something derivable from the Steps slice itself).
3. Extend `executor/openai.Executor.Execute` to decode `usage` from the Chat Completions response and populate `Outcome.ActualCostUSD` using the existing `costPerMillionTokensUSD` table — no change to `EstimateCostUSD`.
4. Wire `engine/strategy.go`'s generate-Step handling (`runSteps`) to carry `outcome.ActualCostUSD` into `recordStep` and accumulate it onto `act.ActualCostUSD`.
5. Extend `cli/cli.go`'s `formatAct` and `engine.SlogReporter`'s `act.execute.start`/verify-adjacent events to surface actual cost when present, alongside the existing estimate — additive rendering, no change to either's existing output when `ActualCostUSD` is nil (every Executor that doesn't report one).
6. Update [roadmap.md](../00-overview/roadmap.md) open decision 9: mark its cost half RESOLVED, leaving the "near-term single-user value" half explicitly still open.
7. Update [README.md](README.md): move this row from Backlog to Accepted upon ratification.
8. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR section, M3/M5 rows (actual-cost reporting touches both — a real Executor capability and a `foundry show`/logging surface), and changelog.

---

## Future ADR Dependencies

- **Extension isolation & contract versioning** (backlog, ADR-0008): no dependency — a third-party Executor may or may not implement `CostEstimator`/populate `ActualCostUSD`; neither is required, mirroring how neither is required of any in-tree Executor today.

---

## Open Questions

Carried forward, not resolved here:

1. **At what point does a real, material divergence between estimated and actual cost become worth acting on** (recalibrating estimates, warning a human, refusing a Pipeline)? Not decided — Decision 5 declines to build this speculatively.
2. **Should Budget ceilings ever become configurable**, and if so, per-project (`.foundry/config.json`) or per-Pipeline? Left to whichever future moment a real project hits today's ceiling (Decision 4's named trigger).
3. **Exact shape of the "N of M generate Steps reported actual cost" bookkeeping** (Decision 3) — a computed helper vs. a stored field — is an implementation detail left to the follow-up PR, not decided at the architecture level here.

---

## Review Checklist

Walked through at ratification (2026-07-21):

- [x] **No contradiction with accepted documents.** Checked against [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) (Decision 1 here directly answers its Open Question 1) and [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)/[ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) (additive-field precedent correctly applied, not reopening either's own scope).
- [x] **Decision 2's claim about the OpenAI response is verified against the real API.** OpenAI's Chat Completions API returns a top-level `usage: {prompt_tokens, completion_tokens, total_tokens}` object on every successful response — confirmed against OpenAI's own published API reference; the follow-up implementation PR decodes it exactly as described.
- [x] **`domain.Budget`'s "enforced as a constraint, never merely reported" doc comment is honored** — `ActualCostUSD` is specified as reported Evidence only; the follow-up implementation PR must not wire it into `tracker.charge` as a second gate.
- [x] **Process caveat resolved.** Ratified under [ADR-0000](ADR-0000-governance-and-ratification-process.md); this Status row, [README.md](README.md)'s backlog table, and the Migration Strategy's downstream docs all updated in the same ratifying pass.

---

_This ADR closes [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)'s open question on whether `CostEstimator` should become mandatory — no, because the Claude Code subprocess structurally cannot report a real cost signal, not merely "not yet." It introduces one small, genuinely additive domain change: an optional `ActualCostUSD` on `Outcome` (and matching accumulators on `StepRecord`/`Act`) so an Executor that can report real post-execution cost (today, `executor/openai`) may do so — surfaced as honest, possibly-partial reporting Evidence, never as a second Budget gate. It declines to make Budget's ceilings configurable, and declines any cost-reconciliation or calibration logic, until each has a real, named trigger — the same discipline this project's entire ADR series has applied consistently._
