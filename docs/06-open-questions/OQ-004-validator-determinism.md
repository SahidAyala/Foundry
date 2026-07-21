# OQ-004 — What can verification honestly guarantee?

**Maturity: OPEN QUESTION** · informs [../02-architecture/trust.md](../02-architecture/trust.md) (PROVISIONAL)

## Problem
Trust rests on verification. An earlier statement claimed "validators and gates are deterministic functions of artifacts — the same change always yields the same verdict." But Validators invoke real external tools (compilers, linters, test runners) whose output depends on tool version, environment, and ordering. What can the guarantee honestly be?

## Context
Flagged in the archived consistency and freeze reviews as an over-claim. The honesty value (V5) forbids promising determinism the system cannot deliver.

## Alternatives
1. **Pure-function claim** — assert validators are deterministic functions (false in practice for external tools).
2. **Record-and-replay** — validator *invocations* and their outputs are recorded; replay reproduces the recorded verdict rather than re-running the tool.
3. **Pinned-environment determinism** — guarantee determinism only within a pinned tool/environment fingerprint, recorded with the Act.
4. **Gate-only determinism** — promise determinism for the *Gate* (pure function of findings) but not for the Validators that produce findings.

## Arguments
- The pure-function claim is the cleanest story and the least true.
- Record-and-replay matches how non-deterministic Executors are already handled and keeps replay coherent.
- Pinned-environment is the strongest *forward* guarantee but pushes environment capture into the Record.
- Gate-only is honest and minimal but admits verdicts can differ if tools differ.

## Open questions
- Is the environment fingerprint part of the Evidence/Record?
- Where is the line between "deterministic, re-executed" and "non-deterministic, replayed" for validators specifically?

## Current recommendation
Adopt **gate-only determinism + record-and-replay for validator outputs** (4 + 2): the Gate verdict is a pure function of recorded findings; validator findings are recorded, not re-derived for the identity guarantee. Drop the "deterministic function of artifacts" phrasing. PROVISIONAL.

## Status
**RESOLVED — 2026-07-20.** Graduated to [ADR-0003 — Replay & Determinism Contract](../03-adrs/ADR-0003-replay-and-determinism-contract.md), Accepted, jointly with [OQ-003](OQ-003-replay-across-versions.md) as anticipated — but **not** with this page's own "current recommendation." ADR-0003 Decision 3 ratifies what was actually shipped instead: Validators and the Gate *are* re-executed for real on replay (this page's Alternative 1, "pure-function claim," in practice — not Alternative 2/4's "record-and-replay for validator outputs," which was never built), with a resulting divergence always surfaced as honest data, never hidden or asserted impossible. This page's own open questions (environment fingerprint as Evidence; the deterministic/replayed line for Validators specifically) are carried forward in that ADR's own Open Questions section — this page is kept for historical context, not as a live question.
