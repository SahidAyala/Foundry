# ADR-0005 — Executor Contract & Capability Model

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-16 under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s governance process. Originally drafted per [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.3. |
| **Date** | Drafted 2026-07-14; Accepted 2026-07-16 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted per RFC-0004 |
| **Ratifies** | The ADR backlog entry named in [../03-adrs/README.md](README.md) ("Executor contract & capability model") — the normalized contract every Executor implements, per Decisions 1–5 below. |
| **Gates** | Piece 3 of [multi-executor-router-implementation-plan.md](../04-guides/multi-executor-router-implementation-plan.md) — writing a second real vendor `Executor`. Piece 3 (`executor/openai`) has since shipped against this contract. |
| **Process note** | Accepted under [ADR-0000](ADR-0000-governance-and-ratification-process.md). RFC-0004, the RFC this ADR was drafted from, remains Draft — Proposed in its own right; ratifying this ADR does not ratify that RFC. |

---

## Context

[Piece 1 of RFC-0004](../04-guides/multi-executor-router-implementation-plan.md) has shipped: `engine.Step` carries optional `Capability`, `Executor`, and `FeedsForward` fields; `engine.ExecutorRegistry` registers named Executors; `engine.Router` resolves a Step's explicit pin or falls back to the Engine's default; `project.LoadExecutorConfig` decodes `.foundry/executors.json` into `map[string]project.ExecutorConfig{Vendor, Model, APIKeyEnv}`. All of it is additive; today exactly one real `Executor` exists (`executor/claude.ClaudeExecutor`, a subprocess wrapping the Claude Code CLI), constructed once at the composition root.

`engine.Executor` itself is one method:

```go
type Executor interface {
	Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error)
}
```

This port is already vendor-agnostic — RFC-0003 §3.3 established that adding a vendor is "write an adapter," not "redesign the Engine," and RFC-0004 §2.3 does not reopen it. What RFC-0004 §2.3 names as genuinely undecided, and what motivates this ADR before Piece 3 (a second real vendor) is written:

1. **Invocation shape.** `ClaudeExecutor` shells out to a subprocess (matching [ADR-0001](ADR-0001-language-and-toolchain.md)'s non-Go extension boundary). A vendor reachable by a clean HTTP API (RFC-0004 §2.3's recommendation for the *second* vendor, before a literal Copilot adapter) has no subprocess, no local binary dependency, and a different failure taxonomy — HTTP status codes and rate limits, not process exit codes and stderr.
2. **Cost/telemetry.** `engine/budget.go`'s `tracker.charge` charges one flat estimate per attempt (`executeCostEstimateUSD`), regardless of which Executor ran or how many Steps an attempt contains. RFC-0004 §2.7 (Piece 5 of the implementation plan) recommends per-Step charging — that requires *some* Executor-reported or Executor-configured cost signal to charge against, and nothing today defines what that signal is or how an Executor exposes it.
3. **Capability truthfulness.** `.foundry/executors.json` entries and a Step's `capability` tag are both free-form, declared data; nothing checks that a vendor configured under `role: review` can actually review. Piece 1's Router is explicit-pin-only (RFC-0002 §7 layer 2's negotiation is deliberately out of scope), so today a false claim is a human's misconfiguration to catch, not the Router's — this ADR has to say whether that remains the posture once a second real vendor exists, or whether something must verify it.

This ADR resolves these three so Piece 3 has a contract to implement against, rather than inventing one implicitly the first time a second vendor is wired in.

---

## Decision

1. **`engine.Executor.Execute`'s signature does not change.** It remains the one contract every Executor — subprocess-backed or API-backed, real or scripted/test — satisfies. This ADR does not add a second required method to it.

2. **Invocation shape is entirely an adapter-internal concern.** Nothing in `engine`, `engine.Strategy`, or `engine.Router` may assume a subprocess, an HTTP client, or any other transport. `executor/claude` (subprocess) and a future `executor/<vendor>` (pure API, per RFC-0004 §2.3's recommendation to prove the mechanism with a clean-API vendor before a literal Copilot adapter) are both just types satisfying `Executor` — the Router (Piece 1) already resolves by name, never by shape.

3. **Cost reporting is an optional, type-asserted second interface, not a mandatory addition to `Executor`:**

   ```go
   // CostEstimator is an optional Executor capability: an Executor that can
   // report its own per-call cost estimate implements it. engine/budget.go's
   // per-Step accounting (RFC-0004 §2.7) type-asserts for it and falls back
   // to today's flat executeCostEstimateUSD constant when an Executor does
   // not implement it — so ClaudeExecutor and executor.ScriptedExecutor (the
   // test double) require zero changes to keep working exactly as they do
   // today.
   type CostEstimator interface {
       EstimateCostUSD(ctx context.Context, intent *domain.Intent, considered []string) (float64, error)
   }
   ```

   This keeps Piece 1's zero-forced-change discipline: no existing Executor is required to implement it, and per-Step budget accounting (Piece 5) degrades gracefully to today's behavior for any Executor that doesn't.

4. **Capability truthfulness is declared, not verified, for the lifetime of explicit-pin-only routing.** A `capability` tag on a Step or an entry in `.foundry/executors.json` is advisory metadata a human configures and a human is responsible for getting right — exactly the posture [extensibility.md](../02-architecture/extensibility.md) already states for Executors generally ("Extension output is untrusted like any Executor output — it passes the same verification and Judgment"). This ADR does not add a startup self-test, a capability-verification handshake, or any other trust mechanism, because nothing consumes a Capability for matching yet (RFC-0002 §7 layer 2's negotiation is out of scope until a real multi-Executor Pipeline in production motivates it, per the implementation plan's own §6 design decision 1). Building verification for a property nothing reads is premature.

5. **Constructing a real vendor Executor is a small, uniform adapter shape:** a new package `executor/<vendor>`, exposing a constructor taking a `project.ExecutorConfig` (or its decoded `{Model, APIKeyEnv}` fields) and reading its credential from the environment variable `APIKeyEnv` names **at construction time only** — never persisted to disk, never logged, never passed through `domain.Intent` or any recorded Evidence. This mirrors how `claude.NewClaudeExecutor` already reads its own credential from its own environment, unmanaged by Foundry (RFC-0004 §2.2's stated posture). The adapter is registered into an `engine.ExecutorRegistry` at the composition root — `cmd/foundry/commands/do.go`'s `wireEngine` and `session.Session` — the same place `ClaudeExecutor` is wired today.

---

## Alternatives Considered

### A universal transport-agnostic port with subprocess as a "fallback shim"
- **For:** One conceptual model for "how an Executor talks," rather than two unstated shapes.
- **Against:** No second concrete need exists yet to justify the abstraction; `Execute`'s existing signature already hides transport entirely from callers — an adapter's internals (subprocess vs. HTTP client) are invisible to `Engine`/`Router` today, with zero additional plumbing.
- **Verdict:** Rejected. Nothing is gained that isn't already true; it would add ceremony for a distinction only adapter authors ever see.

### Mandatory cost-reporting on every Executor (no optional `CostEstimator`)
- **For:** Guarantees every Executor, present and future, contributes to per-Step Budget accounting with no silent fallback.
- **Against:** Forces `ClaudeExecutor` and `executor.ScriptedExecutor` to change *today*, before Piece 5 is even sequenced, contradicting Piece 1's discipline that every prior commit required no change to existing Executors. A test double gains an obligation ("estimate a cost") that has no meaning for a scripted, zero-cost fixture.
- **Verdict:** Rejected in favor of the optional, type-asserted interface (Decision 3) — additive, not breaking, matching every other piece of this migration.

### Verify capability claims via a self-test at registration (e.g. `Executor.Ping()`)
- **For:** Would catch a misconfigured `.foundry/executors.json` entry (wrong model name, expired credential) before an Act ever runs against it.
- **Against:** Speculative for a policy that never negotiates on Capability (Decision 4) — verifying a property nothing reads is complexity paid for with no consumer. It also assumes every vendor exposes a cheap, side-effect-free self-test call, which is not true in general (some APIs bill even a minimal ping).
- **Verdict:** Rejected for this phase. Revisit only if/when RFC-0002 §7 layer 2 negotiation is actually built — that ADR (Routing & policy, backlog) would be the one to decide whether verification is worth its cost.

---

## Consequences

### What this decision makes EASIER
- **Piece 3 (a second real vendor Executor) has a concrete template**: implement `Execute`, optionally implement `CostEstimator`, read credentials from the named environment variable at construction, register into the existing `ExecutorRegistry`. No Engine, Strategy, or Router change is implied.
- **Piece 5 (per-Step Budget accounting)** has a defined seam to read a per-call cost from, without having designed Piece 5 itself yet.
- **A future Copilot adapter, or any other vendor**, is scoped by the same three questions this ADR answers, rather than each adapter author re-deciding invocation shape and cost-reporting conventions independently.

### What this decision makes HARDER
- **Two genuinely different failure taxonomies now coexist** (subprocess exit codes/stderr vs. HTTP status/rate-limit errors), and this ADR does not unify them — each adapter documents and handles its own. A future caller that wants to react uniformly to "the Executor is rate-limited" across vendors has no shared error type to switch on yet; that is left to whichever adapter author needs it first, or a later amendment if a real cross-vendor need appears.
- **Capability truthfulness stays a human's responsibility**, which will not scale past explicit-pin-only routing — the moment negotiation (RFC-0002 §7 layer 2) is built, this ADR's Decision 4 must be revisited, not silently inherited.

### Reversibility
High. `CostEstimator` is optional and additive; removing it later affects only Piece 5's accounting, not `Execute`'s contract. The adapter-shape guidance (Decision 5) is a convention, not an enforced interface — a future adapter that deviates from it breaks no compiled contract, only consistency.

---

## Migration Strategy

None required. `executor/claude.ClaudeExecutor` and `executor.ScriptedExecutor` need no change: both already satisfy `Executor`; neither is required to implement the new optional `CostEstimator`.

---

## Future ADR Dependencies

- **Routing & policy** (backlog, proposed as ADR-0006): inherits Decision 4 — "capability declared, not verified" — as its starting assumption. If capability-based negotiation (RFC-0002 §7 layer 2) is ever built, that ADR must explicitly revisit whether a claim needs verifying, rather than silently carrying this ADR's deferral forward.
- **Cost as a first-class constraint** (backlog, proposed as ADR-0011): inherits `CostEstimator` (Decision 3) as the mechanism Piece 5's per-Step charging reads from, and must decide whether it ever becomes mandatory once two or more real vendor Executors exist in-tree (see Open Questions).

---

## Open Questions

1. **Should `CostEstimator` become mandatory once ≥2 real vendor Executors ship in-tree?** Left open for the Cost ADR (backlog) to decide with real per-vendor cost data in hand, not speculatively here.
2. **Is a literal GitHub Copilot adapter in scope at all**, versus proving the mechanism with one clean-API vendor first? RFC-0004 §2.3 raises this and leaves it as an open call for whoever reviews that RFC; this ADR does not resolve it either — it only ensures whichever vendor is chosen second has a contract to implement against.
3. **Retry/backoff conventions per adapter** (e.g., how an HTTP-backed Executor handles a 429) are adapter-internal style, not decided here; promote to this ADR only if a real cross-adapter inconsistency causes a problem.

---

## Review Checklist

Walked through at ratification (2026-07-16), with Piece 3 (`executor/openai`) already shipped to check Decision 3 against a real adapter, not a hypothetical one:

- [x] **No contradiction with accepted documents.** Confirmed: does not contradict ADR-0001 (Go/toolchain — adapters remain in-process Go, per RFC-0004 §2.3's own framing that the extension boundary discussion is separate); does not contradict [extensibility.md](../02-architecture/extensibility.md)'s "Executor output is untrusted" clause (Decision 4 is consistent with it, not an exception to it).
- [x] **Decision 3's optional-interface shape actually holds.** `executor/openai` implements `CostEstimator` cleanly against the real second-vendor adapter; no reshaping was needed.
- [ ] **Decision 4 must be re-examined**, not silently inherited, the moment any negotiation/matching logic over Capability is proposed (RFC-0002 §7 layer 2 — not yet built).
- [ ] **RFC-0004 §2.3's own open call (Copilot's fit) is still open** — this ADR must not be read as having resolved it.
- [x] **Process caveat resolved.** Ratified under [ADR-0000](ADR-0000-governance-and-ratification-process.md); no longer blocked on OQ-006.

---

_This ADR fixes the contract a second real vendor Executor must satisfy. It keeps `Executor.Execute` untouched, adds cost-reporting as optional rather than forced, and explicitly declines to verify capability truthfulness while routing stays explicit-pin-only — deferring that harder question to whichever future ADR actually builds negotiation._
