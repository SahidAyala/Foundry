# Architecture — Knowledge

> **Maturity: PROVISIONAL** (the Authored/Derived split is strongly grounded; whether Knowledge — not the Act — is the domain *center* is [../06-open-questions/OQ-001-domain-center.md](../06-open-questions/OQ-001-domain-center.md)).
>
> **This document answers exactly one question: _What is durable knowledge?_**
> It does not describe how knowledge is produced ([execution.md](execution.md)) or how it is trusted ([trust.md](trust.md)). Terms are defined in [../05-reference/terminology.md](../05-reference/terminology.md).

## Why knowledge is first-class

The product's durable value is not the work Foundry does on any given day — it is what the project *accumulates* from that work. **Knowledge** is the medium in which value compounds: the project's owned, justified model of itself. It is the asset that would still matter if every model vanished. Treating it as durable capital, rather than a side effect, is what separates Foundry from a tool that executes and forgets.

Knowledge plays two roles in every Act ([domain.md](domain.md)):
- **Input** — it is the source of the *considered* half of Evidence ([trust.md](trust.md)).
- **Output** — accepted Acts deposit into it. This closing loop is the point: the project is better positioned for the next Act because of the last one.

## The Authored / Derived split

Knowledge has two strict layers that must never be conflated:

| | **Authored Knowledge** | **Derived Knowledge** |
|---|---|---|
| Origin | Created by humans, or proposed by an Act and human-approved | Computed from primary sources (code, history, the world) |
| Status | **Source of truth** | A cache |
| Durability | Owned, portable, survives any model generation | Recomputable, disposable |
| Examples | Decisions, conventions, rationale, recorded determinations | Structural maps, indexes, summaries |

The split is what makes the durability promise real: the *authored* layer is the engineering capital; the *derived* layer is only an accelerator that can always be rebuilt.

## Ownership and portability

Authored Knowledge belongs to the project, not to Foundry. It must remain readable and exportable independently of any single model or vendor — the organization can leave and keep its knowledge intact. This is a non-negotiable value ([../00-overview/principles.md](../00-overview/principles.md)).

## How knowledge changes

Knowledge is itself evolved through Acts, never silently rewritten:
- A change to Authored Knowledge is an **Outcome** of an Act, reviewed and accepted by an **Authority** like any other change.
- Derived Knowledge is recomputed by the system as primary sources change and requires no review (it is a cache).

> **Unresolved (human decision required):** the **format stability and cross-version migration of Authored Knowledge** has no owning decision yet. Because portability is a flagship promise, "Foundry can always read its own authored knowledge across its own versions" must be guaranteed by a future decision (see [../03-adrs/README.md](../03-adrs/README.md) and [../00-overview/roadmap.md](../00-overview/roadmap.md)). Until then, treat the promise as intended-but-unproven.
