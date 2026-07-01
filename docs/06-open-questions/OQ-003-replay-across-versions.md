# OQ-003 — Does replay hold across Engine versions?

**Maturity: OPEN QUESTION** · informs [../02-architecture/execution.md](../02-architecture/execution.md), [trust.md](../02-architecture/trust.md) (PROVISIONAL)

## Problem
Replay is a flagship trust property: an Act can be re-executed from its Record. But deterministic work is *re-executed*, and the Engine's deterministic behavior changes across releases. Does an Act replay identically under a *future* Engine version, or only the one that produced it?

## Context
The archived freeze review flagged this as undefined. The "engineer three years later replays an old run" scenario is the literal use case behind the audit promise, so the answer materially shapes how strong that promise can honestly be.

## Alternatives
1. **Same-version only** — replay guarantees identity only under the Engine version that produced the Act; cross-version replay is best-effort.
2. **Cross-version guaranteed** — the Engine commits to bit-stable deterministic behavior across versions (heavy, constrains all future change).
3. **Record-everything** — even "deterministic" outputs are recorded, so replay never re-executes; identity is by playback, not recomputation (large records, weaker "we can re-derive" story).

## Arguments
- Same-version is cheap and honest but weakens long-horizon audit.
- Cross-version is the strongest promise but may freeze the Engine's evolution.
- Record-everything sidesteps recomputation drift but blurs the line between deterministic and recorded.

## Open questions
- What exactly does an auditor need — identical artifacts, or a verifiable account of how they were produced?
- Can deterministic-stage behavior be versioned so old Acts pin the producing version?

## Current recommendation
Scope the guarantee to **same-version** initially, and record enough that cross-version *verification* (not necessarily re-derivation) is possible. Make the scope explicit wherever replay is promised. PROVISIONAL.

## Status
**OPEN.** Owns a pending ADR (replay & determinism contract); see [../03-adrs/README.md](../03-adrs/README.md).
