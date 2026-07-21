# ADR-0003 — Replay & Determinism Contract

| | |
|---|---|
| **Status** | **Proposed** — drafted per the ADR backlog ([README.md](README.md)), [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md), and [OQ-004](../06-open-questions/OQ-004-validator-determinism.md); not yet ratified. |
| **Date** | Drafted 2026-07-20 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | The ADR backlog entry named in [README.md](README.md) ("Replay & determinism contract") — which work is re-executed vs. replayed; cross-version replay scope; verification's honest guarantee. |
| **Gates** | [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) and [OQ-004](../06-open-questions/OQ-004-validator-determinism.md), both explicitly assigned to this one ADR by their own Status sections. Also [docs/02-architecture/trust.md](../02-architecture/trust.md)'s "Unresolved: real Validators invoke external tools..." callout, [execution.md](../02-architecture/execution.md)'s "Unresolved: cross-version scope" callout, [roadmap.md](../00-overview/roadmap.md) open decisions 4–5, and [invariants.md](../05-reference/invariants.md) I3/I6's "pending owning ADR" notes. |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. Inherits [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s ratified on-disk Record format as the concrete bytes `foundry replay` reads. |

---

## Context

**What is already built, and this ADR ratifies rather than invents:** `replay.Verify` (`replay/replay.go`) walks a recorded Act's Step trace; for each `verify` StepRecord, it takes the immediately preceding `generate` StepRecord's recorded `Produced` patch, builds a fresh `domain.Outcome` from it, and calls a **real** `engine.Verifier` — the same Gate and Validators (`go build`, `go test`, etc.) that ran originally, staged via `workspace.StagedVerifier` so the developer's own checkout is never touched. **The Executor is never invoked again** — only the recorded Patch is replayed. The resulting verdict is compared to the recorded one and reported as `StepResult{Reproduced, RecordedVerdict, ReplayedVerdict, RecordedChecked, ReplayedChecked}`; a **mismatch is data, not an error** — `replay_test.go`'s `TestVerify_DivergedVerdictIsNotReproduced` and `TestVerify_DivergedCheckedSameVerdictStillReproduced` both exercise this directly. The package's own doc comment already states its scope precisely: *"a same-version replay guarantee only... it proves verification is reproducible under the Engine build that runs it today, not that it would reproduce identically under a future Engine version."*

This is already an exact, working instance of two PROVISIONAL invariants — [I2](../05-reference/invariants.md) ("process determinism, not output determinism") and [I3](../05-reference/invariants.md) ("deterministic work is re-executed and must match; non-deterministic Executor output is replayed from the Record") — this ADR grounds them with a real ratified decision rather than leaving them as unproven intent.

**A real discrepancy this ADR must resolve, not silently inherit:** [OQ-004](../06-open-questions/OQ-004-validator-determinism.md)'s own "current recommendation" is *"record-and-replay for validator outputs"* — i.e., never re-run the actual tool; compare only against the findings already recorded. **That was never built.** What shipped instead — Decision 2 below — actually re-executes the Gate and every Validator for real against the staged, reapplied patch, exactly the risk OQ-004 itself named ("Validators invoke real external tools whose output depends on tool version, environment, and ordering"). This is not an oversight to quietly paper over: the shipped design is *more useful* than the recommendation (it genuinely re-verifies "does this still build/pass tests today," not merely "did the recorded text match itself") and is already honest about divergence (it reports a mismatch, never hides or asserts one away) — this ADR ratifies what was actually built, explicitly superseding OQ-004's unbuilt recommendation, the same way [ADR-0006](ADR-0006-routing-and-policy.md) ratified an already-shipped Router that had likewise diverged from an earlier document's fuller sketch.

**A second, smaller drift this ADR also corrects:** [execution.md](../02-architecture/execution.md)'s own "Determinism and replay" section (line 46) still cites [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md) as an open "current recommendation" and its closing "Unresolved" callout (line 48) still lumps OQ-008 in with cross-version replay scope as jointly pending — but [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) already resolved OQ-008 separately. [trust.md](../02-architecture/trust.md)'s own "Unresolved" callout naming the Validator-determinism question (line 35, corrected wording pending this ADR) is the one this ADR closes.

This ADR does not resolve **Resume**'s own separate scope boundaries (no repair-boundary crossing, no mid-flight Pipeline-Step-list changes) — those are already documented in [execution.md](../02-architecture/execution.md) as Resume's own limits, a distinct mechanism (continuing an *interrupted* attempt) from Replay (re-verifying a *completed* one), and are not reopened here.

---

## Decision

1. **Replay is ratified as a same-version guarantee only, closing [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) with its own Alternative 1.** `foundry replay <act-id>` proves verification is reproducible under the exact Engine build, tool versions, and environment that run it *today* — it makes no claim, implicit or explicit, about a future Engine version. Cross-version replay (OQ-003's Alternative 2) and record-everything (Alternative 3) are both declined, not designed against speculatively.

2. **"Deterministic work is re-executed; non-deterministic Executor output is replayed from the Record" ([I3](../05-reference/invariants.md)) is ratified as the concrete mechanism `replay.Verify` already implements.** A `verify` Step's real Gate and Validators are re-run for real against the recorded `Produced` patch; the Executor that originally produced that patch is never invoked again. This closes the "which work is re-executed vs. replayed" half of this ADR's charter with the already-shipped answer, not a new design.

3. **[OQ-004](../06-open-questions/OQ-004-validator-determinism.md) is closed with the answer actually shipped, explicitly superseding its own unbuilt recommendation.** Validators and the Gate *are* re-executed for real on replay (Decision 2) — OQ-004's "record-and-replay for validator outputs" alternative was never built and is not adopted now. The honest guarantee this ADR ratifies instead: replay promises the *same Steps run again, in the same structure, against the same recorded Patch* (**process** determinism, [I2](../05-reference/invariants.md)) — it does not promise, and `replay.Result` does not assert, that a real Validator's verdict cannot differ from its recorded one. A divergence is always surfaced as first-class data (`Reproduced() == false`, both verdicts and both finding sets retained) — **never** hidden, silently retried, or asserted impossible.

4. **No environment fingerprint (tool version, OS, architecture) is captured or compared, and none is added by this ADR.** When a replay diverges, its root cause (environment drift vs. a genuine regression) is left to a human reading `RecordedChecked` against `ReplayedChecked` — the same finding text already available. Building automatic fingerprint capture/comparison is real, additional work with no requesting consumer today; declined for the same reason [ADR-0006](ADR-0006-routing-and-policy.md) and [ADR-0009](ADR-0009-cli-and-output-contract.md) each declined their own speculative extensions.

5. **No version field is added to `act.json`, closing the question [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) Decision 8 explicitly deferred here.** Decision 1 scopes the replay guarantee to same-version only — there is no cross-version behavior for a version field to gate. Revisit only if cross-version replay itself is ever built; until then, a version field would tag Acts against a guarantee Foundry does not make.

6. **Resume's own scope boundaries are unaffected and not re-litigated here.** Resume (continuing an interrupted attempt) and Replay (re-verifying a completed one) remain the distinct mechanisms [execution.md](../02-architecture/execution.md) already documents them as; this ADR rules on Replay and the determinism guarantee only.

7. **[execution.md](../02-architecture/execution.md)'s "Determinism and replay" section is corrected**, not left to drift further: its OQ-008 references are updated to cite [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) by name, and its "Unresolved" callout is narrowed to reflect that only Resume's own named limits (not cross-version replay, not OQ-008) remain open after this ADR.

---

## Alternatives Considered

### Adopt OQ-004's original recommendation: record validator findings, never re-run the tool
- **For:** Perfectly honest — a recorded-and-compared finding can never itself diverge, since nothing is re-executed.
- **Against:** This is not what shipped, and adopting it now would be a real regression, not a refinement: replay's entire practical value today is genuinely re-verifying "does this patch still build and pass tests," which a pure record-and-compare approach would lose completely (it would trivially always "reproduce" the recorded findings, having never actually checked anything). Nobody has asked for this weaker guarantee; the stronger one already works and is already honest about its own limits.
- **Verdict:** Rejected. Ratified instead as Decisions 2–3 — the shipped, more useful behavior, made honest by surfacing divergence rather than hiding the risk.

### Commit to cross-version replay determinism now
- **For:** The strongest possible audit promise — "an Act replays identically no matter how old."
- **Against:** [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)'s own argument stands: this would freeze the Engine's evolution around a bit-stability promise, and no real "an engineer three years later replays an old run" case has occurred yet to justify paying that cost now.
- **Verdict:** Rejected. Ratified instead as Decision 1 — same-version only, the honest and currently sufficient scope.

### Capture and compare an environment fingerprint (tool versions, OS) on replay
- **For:** Would let a divergence be automatically classified as "environment drift" vs. "genuine regression" instead of leaving that to a human.
- **Against:** No consumer needs this distinction automated today; a human reading two finding sets side by side already gets the same information. Building it now is exactly the premature-generality pattern this project's prior ADRs have consistently declined.
- **Verdict:** Rejected for now. Ratified instead as Decision 4 — left to human judgment until a real need for automation surfaces.

### Add a version field to `act.json` now
- **For:** Would let a future cross-version replay attempt at least detect and name the version gap, rather than failing opaquely.
- **Against:** Decision 1 makes no cross-version promise at all — there is no guarantee today for a version field to gate or improve the failure mode of. Adding one now designs infrastructure for a scenario this ADR explicitly does not support.
- **Verdict:** Rejected. Ratified instead as Decision 5 — deferred until cross-version replay is itself ever built.

---

## Consequences

### What this decision makes EASIER
- **[OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) and [OQ-004](../06-open-questions/OQ-004-validator-determinism.md) are both closed** in one ADR, exactly as their own Status sections anticipated.
- **[I2](../05-reference/invariants.md) and [I3](../05-reference/invariants.md) are grounded** with a real, ratified decision instead of remaining stated intent.
- **[trust.md](../02-architecture/trust.md)'s and [execution.md](../02-architecture/execution.md)'s stale "Unresolved" callouts are corrected**, including a small drift left over from [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s own ratification (execution.md's OQ-008 references).
- **A future cross-version-replay ADR, if one is ever drafted, inherits a clean, explicit same-version baseline** to extend from, rather than ambiguity about what today's guarantee even was.

### What this decision makes HARDER
- **Nothing structurally** — like [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) and [ADR-0006](ADR-0006-routing-and-policy.md), this ratifies already-shipped, already-tested behavior and declines speculative extensions.
- **A replay divergence caused by pure environment drift (e.g. a compiler patch version) still reads identically to a genuine regression** in `foundry replay`'s output — Decision 4 accepts this as a real, named limitation rather than solving it speculatively.

### Reversibility
High. Every decision here either ratifies already-shipped, already-tested behavior (Decisions 1–3, 6) or declines to build something new (Decisions 4–5) — nothing to unwind later.

---

## Migration Strategy

No code changes; no data migration. This ADR ratifies `replay.Verify` exactly as already implemented and tested.

1. Correct [execution.md](../02-architecture/execution.md)'s "Determinism and replay" section: cite this ADR (not OQ-003/OQ-004's own recommendations) for the replay/determinism guarantee, cite [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) (not OQ-008) for where in-progress state lives, and narrow the closing "Unresolved" callout to only what remains genuinely open (nothing, after this ADR, on this document's own subject).
2. Correct [trust.md](../02-architecture/trust.md)'s "Unresolved: real Validators invoke external tools..." callout to reflect resolution.
3. Resolve [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) and [OQ-004](../06-open-questions/OQ-004-validator-determinism.md): update their own Status sections and [open-questions/README.md](../06-open-questions/README.md)'s index rows.
4. Update [roadmap.md](../00-overview/roadmap.md) open decisions 4 ("Cross-version replay scope") and 5 ("Validator determinism limits") to RESOLVED.
5. Update [invariants.md](../05-reference/invariants.md)'s closing note to remove I3's cross-version scope and I6's validator-determinism-limits caveats from the "pending owning ADR" list.
6. Update [README.md](README.md): move this row from Backlog to Accepted.
7. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR section and changelog.

---

## Future ADR Dependencies

- **A future cross-version-replay ADR**, if the trigger named in Decision 1 (a real "replay an old Act under a materially newer Engine" need) ever fires, inherits this ADR's same-version baseline as the thing it would extend, and must explicitly decide the version-field question Decision 5 declined here.
- **Knowledge & semantic store** (backlog, ADR-0007): no dependency — Knowledge's own retrieval determinism, if any, is that ADR's question.
- **Cost as a first-class constraint** (backlog, ADR-0011): no dependency.

---

## Open Questions

Carried forward, not resolved here:

1. **What exactly does an auditor need** — identical artifacts, or a verifiable account of how they were produced? ([OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)'s own open question, still genuinely open.)
2. **Is the environment fingerprint ever worth capturing as part of Evidence/the Record**, once a real need for automated divergence classification appears? Deferred per Decision 4.

---

## Review Checklist

To be completed at ratification:

- [ ] **No contradiction with accepted documents.** Confirm against [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) (this ADR inherits its on-disk format and Decision 8's deferral correctly) and [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md)/[ADR-0005](ADR-0005-executor-contract-and-capability-model.md)/[ADR-0006](ADR-0006-routing-and-policy.md) (no overlap).
- [ ] **Decisions 1–3 verified against the actual shipped code** — `replay/replay.go`, `replay/replay_test.go` (specifically `TestVerify_DivergedVerdictIsNotReproduced` and `TestVerify_DivergedCheckedSameVerdictStillReproduced`) — re-read at ratification to confirm nothing has drifted since drafting.
- [ ] **execution.md's and trust.md's corrected text reads accurately** once amended.
- [ ] **Process caveat resolved.** Ratify under [ADR-0000](ADR-0000-governance-and-ratification-process.md); update this Status row and the backlog table in [README.md](README.md) in the same ratifying commit.

---

_This ADR ratifies `replay.Verify` exactly as already shipped: same-version-only replay, real Validators and the Gate re-executed against a recorded Patch (never the Executor), and a verdict divergence always surfaced as honest data rather than hidden or assumed impossible. It closes OQ-003 and OQ-004 together, explicitly superseding OQ-004's own recommendation with the stronger, more useful design that was actually built, and declines both automated environment-fingerprinting and a Record version field until either has a real, named consumer._
