# Architecture — Trust

> **Maturity: PROVISIONAL** (the trust *principles* are strongly grounded in RFC-0001; the *verification guarantee's* exact strength is open — [../06-open-questions/OQ-004-validator-determinism.md](../06-open-questions/OQ-004-validator-determinism.md)).
>
> **This document answers exactly one question: _How is trust established?_**
> It does not define the domain ([domain.md](domain.md)) or describe production mechanics ([execution.md](execution.md)). Terms are defined in [../05-reference/terminology.md](../05-reference/terminology.md).

## What "trust" means here

Trust is what Foundry actually manufactures. A trustworthy Outcome is one for which the project can answer: *what was intended, what was considered, what was checked, who accepted it, and can it be reproduced.* Trust is therefore a property of the **Act**, not of the Executor that produced it — a model becoming more capable does not, by itself, make its output trustworthy.

Trust is established along one axis with four contributions, all of which an Act records:

1. **Provenance** — what informed the Outcome (the *considered* Evidence: Context drawn from Knowledge and the world, each piece attributed to its source).
2. **Verification** — what was checked (the *checked* Evidence: **Validator** findings, deterministic-first).
3. **Accountability** — who owns the decision (the **Authority** behind the **Judgment**).
4. **Reproducibility** — whether the path can be re-run (the immutable Record + replay).

## The trust gate is internal to every Act

An Outcome is **untrusted** the moment it is produced. It becomes trustworthy only by passing through:

```
Outcome produced (untrusted)
   └─▶ Verification (Validators → findings) ──▶ Gate verdict: pass | fail | repair
          └─▶ Authority accepts (accountable)  ──▶ trusted, may be applied
```

This gate is **Strategy-independent**: a Pipeline and an adaptive agent face the identical verification and the identical accountable acceptance. Changing *how* an Outcome was produced never changes *how* it earns trust.

## Deterministic-first verification

Verification prefers deterministic checks (compilers, type-checkers, tests, structural rules) over model-based judgment, because deterministic checks are cheaper, explainable, and reproducible. A model-based check is a last resort for the irreducibly subjective, never the default.

> **Unresolved (human decision required):** real Validators invoke external tools whose outputs are not pure functions of the artifact (tool version, environment, ordering). The exact guarantee — which checks are re-executed on replay vs replayed from the Record, and the honest strength of "the same change yields the same verdict" — must be fixed by a pending decision ([../03-adrs/README.md](../03-adrs/README.md)). Do not assert unqualified validator determinism until then.

## Accountability and authority

Foundry defaults to **approval, not autonomy**: an **Authority** (a human, or an explicitly delegated policy) accepts consequential and outward Outcomes. Accountability never leaves a human silently. Autonomy is a dial an Authority may raise for low-blast-radius work; it is never the default and never removes ownership. This is a value, not a capability limit ([../00-overview/principles.md](../00-overview/principles.md)).

## Reproducibility

The immutable Record makes any Act re-inspectable and (within the limits above) replayable. This is the basis of audit: the answer to "how was this built and why should we trust it?" is read directly from the Act, not reconstructed after the fact.

> **Resolved:** the durability classification of the Record itself — it is durable, not a disposable cache, per [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md) (ratifying I8). A project should commit `.foundry/acts/` to its own repository.
