# Principles

> **Maturity: PROVISIONAL** (strongly grounded in RFC-0001, which is unratified). Current statement of the principles and values that govern every Foundry decision. Their operational form (the rules code must uphold) is [../05-reference/invariants.md](../05-reference/invariants.md); their full argued form is [../01-rfcs/RFC-0001-vision-and-product-philosophy.md](../01-rfcs/RFC-0001-vision-and-product-philosophy.md). Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Core principles

Each principle exists to *reject* proposals, not to decorate. If it cannot kill a feature, it is not a principle.

1. **Trust is the product.** Foundry manufactures trust (provenance + verification + accountability + reproducibility), not code. A faster output that is less explainable, verifiable, attributable, or reproducible is moving the wrong way.
2. **Software engineering over prompt engineering.** Control flow is owned by deterministic process; model output is untrusted until verified; the process is the durable artifact, the prompt is disposable.
3. **Knowledge is durable capital; the model is replaceable substrate.** Invest in what survives a model generation. Exploit a model's full power, but never let a project's value depend on any one model.
4. **The human is the source of intent and the holder of accountability.** Approval, not autonomy, by default. Autonomy is a dial, never a destination.
5. **Reproducibility over reproduction — honest determinism.** Promise *process* determinism and replay; never imply identical model output.
6. **Ceremony must be earned by value.** Discipline is proportional to the stakes; the trivial path stays trivial.
7. **Earn each capability.** A capability is added because it compounds value for users who already benefit — not because a roadmap lists it.
8. **Boring where it counts.** The part that records, verifies, and holds accountability is conservative; cleverness lives at the replaceable edge.

## Values that are never compromised

- **V1 — Accountability stays with a human.**
- **V2 — A project's source of truth is never silently mutated.**
- **V3 — Every consequential action is auditable.**
- **V4 — A project owns its knowledge and process, and they are portable.**
- **V5 — Honesty about what AI can and cannot guarantee.**
- **V6 — No vendor capture.**

## Filesystem-first persistence

The canonical storage format for Foundry is the project's filesystem. Every durable artifact (Acts, Knowledge, Evidence, configuration, and history) is represented as versionable files. Databases, indexes, caches, and remote services are optional derived storage layers and must never become the canonical source of truth.

## How decisions are evaluated

Every significant decision (RFC, ADR, design) should be answerable against: durable-vs-disposable, provenance/audit, control flow, accountability, the simple path, vendor capture, honesty, the compounding loop, and reversibility.

> **Open governance question (human decision required):** the principles do not yet have a ratified *priority ordering* for when two of them conflict. Until resolved, treat conflicts as escalations, not as something to settle silently. Tracked in [roadmap.md](roadmap.md) (item 2).
>
> A separate, related question — *whether there was any process at all for accepting decisions* — is now resolved: [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) defines a lightweight, sole-maintainer-led ratification process. That ADR settles *who can accept a decision and how*; it does not settle *which principle wins when two conflict*, which remains the open question above.
