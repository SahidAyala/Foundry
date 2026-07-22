# Concept Reference

> Quick-reference map of how Foundry's concepts relate, and how mechanism maps to the domain. **Definitions live once, in [terminology.md](terminology.md); the narrative model lives in [../02-architecture/domain.md](../02-architecture/domain.md).** This file only cross-references — it does not define.

## Maturity index (status of every major concept)

See [maturity levels](../04-guides/documentation.md#maturity-levels). **CANONICAL is now reachable, narrowly**: [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) resolved the systemic blocker (no ratification process existed). Most of the repository is still PROVISIONAL, not because CANONICAL is unreachable in principle, but because each document still needs its own explicit ratification — nothing upgrades automatically. Only [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md), [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md), [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md), [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md), [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md), [ADR-0006](../03-adrs/ADR-0006-routing-and-policy.md), [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md), [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md), [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md), [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md), and [ADR-0011](../03-adrs/ADR-0011-cost-as-a-first-class-constraint.md) are Accepted so far — **the entire ADR backlog is now empty**; RFC-0001 through RFC-0005 remain Draft — Proposed.

| Concept / decision | Maturity | Owner | Open question |
|---|---|---|---|
| Trust = product; model is substrate; knowledge is durable | PROVISIONAL (strongly grounded in RFC-0001) | [vision.md](../00-overview/vision.md), [principles.md](../00-overview/principles.md) | — |
| Deterministic-first; control flow in the Engine; untrusted-until-verified; human accountability | PROVISIONAL (grounded in RFC-0001) | [trust.md](../02-architecture/trust.md), [invariants.md](invariants.md) | — |
| Authored/Derived knowledge split | PROVISIONAL (grounded); note format & durability ACCEPTED via [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md) | [knowledge.md](../02-architecture/knowledge.md) | — |
| **Act as domain center** | PROVISIONAL (working hypothesis) | [domain.md](../02-architecture/domain.md) | [OQ-001](../06-open-questions/OQ-001-domain-center.md) |
| **Pipeline = one Strategy** | PROVISIONAL (working hypothesis) | [execution.md](../02-architecture/execution.md) | [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md) |
| **Vocabulary** (Act/Engine/Strategy/…) | PROVISIONAL (proposal; some coined here) | [terminology.md](terminology.md) | [OQ-007](../06-open-questions/OQ-007-canonical-terminology.md) |
| Extension isolation mechanism | OPEN (requirements only; ratified as a deliberate non-decision via [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md)) | [extensibility.md](../02-architecture/extensibility.md) | [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) — RESOLVED (no mechanism chosen) |
| Governance / ratification | **RESOLVED** | [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) | — |
| Language = Go | ACCEPTED | [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) | both pending amendments resolved (persistence via ADR-0002, extension isolation via ADR-0008) |
| Persistence, content-addressing & on-disk layout | ACCEPTED | [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md) | — |
| Replay & determinism contract | ACCEPTED | [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md) | — |
| Reusable-Act template format & evolution policy | ACCEPTED | [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md) | — |
| Executor contract & capability model | ACCEPTED | [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) | — |
| Routing & policy (explicit-pin Router; negotiation/failover deferred) | ACCEPTED | [ADR-0006](../03-adrs/ADR-0006-routing-and-policy.md) | — |
| CLI & output contract | ACCEPTED | [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md) | — |
| VCS/PR integration & Apply targets | ACCEPTED | [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md) | — |
| Knowledge & semantic store (note format, `.foundry/knowledge/` durability; Derived Knowledge/semantic retrieval declined pending trigger) | ACCEPTED | [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md) | — |
| Cost as a first-class constraint (`CostEstimator` stays optional; additive `ActualCostUSD` reported Evidence; Budget ceilings stay hardcoded pending trigger) | ACCEPTED | [ADR-0011](../03-adrs/ADR-0011-cost-as-a-first-class-constraint.md) | — |
| Extension isolation & contract versioning (no mechanism chosen; corrects ADR-0001 clause 4's stale gRPC/protobuf pre-commitment) | ACCEPTED | [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md) | — |
| Workflow / Stage / Provider / Skill / Runtime | REJECTED | [archive](../archive/) | — |

## Domain concepts at a glance

| Concept | Role | Owning document |
|---|---|---|
| Act | The unit of justified, accountable, recorded engineering | [domain.md](../02-architecture/domain.md) |
| Intent | Why an Act exists | [domain.md](../02-architecture/domain.md) |
| Strategy | How an Act is produced (pluggable) | [execution.md](../02-architecture/execution.md) |
| Evidence | What was considered + checked | [trust.md](../02-architecture/trust.md) |
| Judgment | Verdict + accountable acceptance | [trust.md](../02-architecture/trust.md) |
| Authority | Who owns a Judgment | [trust.md](../02-architecture/trust.md) |
| Outcome | The proposed transition an Act yields | [domain.md](../02-architecture/domain.md) |
| Knowledge | The durable medium that compounds | [knowledge.md](../02-architecture/knowledge.md) |
| Record | The immutable set of Acts | [trust.md](../02-architecture/trust.md), [system-context.md](../02-architecture/system-context.md) |

## How mechanism maps to the domain (the "below the line" map)

Mechanism terms exist only to *implement* the domain. Use this table to translate implementation vocabulary back to what it serves:

| Mechanism term | Serves which domain concept | Note |
|---|---|---|
| Session | hosts the interactive lifecycle a user runs Acts through | the primary interface, per [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md) |
| Slash Command | the per-instruction unit a Session dispatches, each usually backing one Act | vocabulary a Session understands |
| Engine | produces the **Act**, owns control flow | not a domain concept |
| Pipeline | **one Strategy** | not the center of the system |
| Step | a unit inside the Pipeline strategy | replaces "Stage" |
| Executor | does a unit of work (produces toward an **Outcome**) | a model is one Executor; substrate |
| Router | places work on Executors | deterministic; no control flow |
| Validator | produces the *checked* half of **Evidence** | pure; never mutates |
| Gate | the machine half of a **Judgment** | deterministic verdict |
| Artifact | the content-addressed form of an **Outcome**/Evidence | identity = content |
| Context | the *considered* half of **Evidence** | per-Act selection from Knowledge |
| Budget | a constraint on an **Act** | enforced, not reported |
| Capability | the matching currency between work and Executors | declared + negotiated |

## Relationship summary

- An **Act** carries an **Intent**, is produced by a **Strategy**, accumulates **Evidence**, yields an **Outcome**, passes a **Judgment** owned by an **Authority**, and is preserved in the **Record**.
- **Knowledge** is both an input to **Evidence** and an output of accepted **Outcomes** — the compounding loop.
- All mechanism (Engine, Pipeline, Executor, …) is replaceable substrate beneath these.
