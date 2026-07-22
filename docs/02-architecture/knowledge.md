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

Authored Knowledge belongs to the project, not to Foundry. It must remain readable and exportable independently of any single model or vendor — the organization can leave and keep its knowledge intact. This is a non-negotiable value ([../00-overview/principles.md](../00-overview/principles.md)). Per [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md), a project's `.foundry/knowledge/` directory should be committed to that project's own git repository by default — the same convention [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md) already established for `.foundry/acts/` — since a Knowledge note is the Outcome of an accepted Act, not a disposable cache.

## How knowledge changes

Knowledge is itself evolved through Acts, never silently rewritten:
- A change to Authored Knowledge is an **Outcome** of an Act, reviewed and accepted by an **Authority** like any other change.
- Derived Knowledge is recomputed by the system as primary sources change and requires no review (it is a cache).

> **Resolved:** the **format stability and cross-version migration of Authored Knowledge**, per [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md). Authored Knowledge's note format is deliberately unstructured Markdown prose — no front-matter, no schema, nothing decoded back into a typed structure. This closes the format-stability question by removing what would need to be stable: unparsed text cannot fail to decode, so any future Foundry version, external tool, or human can always read a note, satisfying [I11](../05-reference/invariants.md) as literally as possible. If a future decision ever adds machine-parsed structure to a note, that decision must specify additive-only evolution with a descriptive rejection of unrecognized structure, mirroring [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 4's precedent — not a concern today, since nothing parses a note yet.

## Today's concrete shape

Authored Knowledge has one concrete, working shape today, ratified as sufficient by [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md): a plain Markdown file per contributing Act under `.foundry/knowledge/`, named `<act-id>-<slug>.md` ([RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6's Knowledge-lite capture). It is written only through a Pipeline's `apply` Step declaring `target: "knowledge-note"` (`workspace.KnowledgeNoteApplier`) — never edited in place. A later Act that revisits a topic deposits a *new* note rather than rewriting an old one, exactly as "never silently rewritten" above requires; superseding or curating an earlier note is left to whichever Executor reads both, or a future, deliberate human-authored correction — ADR-0007 explicitly declines to build a formal curation mechanism until a real, named need for one exists.

This is deliberately the *simplest* thing that satisfies "Input... Output" above: no index, no structured front-matter — [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 named it "a write, not a memory system." [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) closes the read side: a naive, lexical Context Source (`knowledge.Gatherer`) retrieves from this same directory back into a later Act's *considered* Evidence, with no change to this storage shape. [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md) explicitly declines to build Derived Knowledge indexing, semantic (embeddings-based) retrieval, or provenance scoring beyond simple attribution — each is deferred until a named, concrete trigger (a measured corpus-scale cost, or a demonstrated lexical-matching miss) actually fires, not designed speculatively ahead of one.
