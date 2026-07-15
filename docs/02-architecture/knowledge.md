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

## Today's concrete shape (provisional, not the format-stability decision above)

Independent of the unresolved format-stability question, Authored Knowledge has one concrete, working shape today: a plain Markdown file per contributing Act under `.foundry/knowledge/`, named `<act-id>-<slug>.md` ([RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6's Knowledge-lite capture). It is written only through a Pipeline's `apply` Step declaring `target: "knowledge-note"` (`workspace.KnowledgeNoteApplier`) — never edited in place. A later Act that revisits a topic deposits a *new* note rather than rewriting an old one, exactly as "never silently rewritten" above requires; superseding or curating an earlier note is left to whichever Executor reads both, or a future, deliberate human-authored correction — not a mechanism this shape provides.

This is deliberately the *simplest* thing that satisfies "Input... Output" above: no index, no structured front-matter, no retrieval yet — [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 named it "a write, not a memory system." [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) closes the read side: a naive, lexical Context Source retrieves from this same directory back into a later Act's *considered* Evidence, with no change to this storage shape. Naming this concrete location here does **not** decide the format-stability question above — front-matter, tags, or a structured schema remain open, and any future migration story still needs the pending decision this document already flags.
