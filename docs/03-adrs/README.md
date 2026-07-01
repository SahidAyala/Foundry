# Architecture Decision Records

> ADRs record specific, binding architectural decisions. **Accepted ADRs are canonical and the architecture documents must never contradict them.** This index lists accepted ADRs and the backlog of decisions that are needed but not yet made. Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Accepted

| ADR | Decision | Notes |
|---|---|---|
| [ADR-0001](ADR-0001-language-and-toolchain.md) | Implementation language & toolchain (Go; single static binary; language-agnostic extension boundary) | Accepted under interim authority. **Has pending amendments**: it currently pre-states an extension *isolation mechanism* that is not its to decide and is left open in [../02-architecture/extensibility.md](../02-architecture/extensibility.md). The mechanism claim should be removed/deferred to the extension ADR. |

## Backlog — decisions required, not yet made

Each owns one or more **compatibility surfaces** (expensive to change after release). They are listed in dependency order. *Numbering is provisional and not yet ratified.*

| Proposed ADR | Owns | Gates |
|---|---|---|
| Persistence, content-addressing & on-disk layout | Record durability (must be durable, not cache), hash/canonicalization, what is committed vs cached | The Record, replay, audit |
| Replay & determinism contract | Which work is re-executed vs replayed; **cross-version replay scope**; verification's honest guarantee | The trust & replay promise |
| Reusable-Act template format & evolution policy | The authored template/definition schema and its versioning | Authoring & sharing |
| Executor contract & capability model | The normalized contract every Executor implements; capability negotiation | Model-agnostic execution |
| Routing & policy | Placement policy and failover over capabilities | Provider independence |
| Knowledge & semantic store | Knowledge persistence; **Authored-knowledge format stability & migration** | The durability/portability promise (V4) |
| Extension isolation & contract versioning | The isolation mechanism (deliberately undecided today) and port versioning | Third-party safety & ecosystem |
| CLI & output contract | The command/flag/output stability policy | Anything scripting Foundry or invoking it in CI |
| Cost as a first-class constraint | How cost is bounded and weighed | Economic viability |

## Status definitions

- **Accepted** — binding; architecture must reflect it.
- **Proposed/Backlog** — identified as needed; not yet written or ratified; must not be treated as decided.
- **Superseded / Rejected** — moved to [../archive/](../archive/); not canonical.

> The backlog above was harvested from a now-archived pre-implementation review ([../archive/reviews/pre-implementation-adr-gate.md](../archive/reviews/pre-implementation-adr-gate.md)) and a freeze review ([../archive/reviews/architecture-freeze-review.md](../archive/reviews/architecture-freeze-review.md)). Those reviews are historical; this index is the canonical statement of what remains to be decided.
>
> **No decisions may be formally ratified until a governance process exists** (see [../00-overview/roadmap.md](../00-overview/roadmap.md), open decision 1).
