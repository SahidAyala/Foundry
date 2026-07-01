# Architecture — Domain

> **Maturity: PROVISIONAL — this is the *current working domain model*, not ratified architectural truth.** The choice of the Act as the center is an unresolved question ([../06-open-questions/OQ-001-domain-center.md](../06-open-questions/OQ-001-domain-center.md)); the vocabulary is provisional ([OQ-007](../06-open-questions/OQ-007-canonical-terminology.md)). Build on it, but do not treat it as settled. See [maturity levels](../04-guides/documentation.md#maturity-levels).
>
> **This document answers exactly one question: _What are the domain concepts?_**
> It does not describe how outcomes are produced (see [execution.md](execution.md)), how trust is established ([trust.md](trust.md)), what knowledge is ([knowledge.md](knowledge.md)), what may be extended ([extensibility.md](extensibility.md)), or where the boundaries are ([system-context.md](system-context.md)).
> Every term below is defined (provisionally) in [../05-reference/terminology.md](../05-reference/terminology.md) and must not be redefined here.

## The current working model

The working model centers the domain on a single proposed unit: the **Act** — *a justified, accountable transition of Project State.*

This is the result of reducing the product to first principles: Foundry exists to evolve a project's state *responsibly* — justified, accountable, recorded, and compounding. On this model, the thing that is justified, owned, recorded, and learned-from is the Act, and everything else in the domain is something an Act *contains*, *operates on*, or *deposits into*.

> **Honesty note.** "The Act is *the* fundamental abstraction" is a **working hypothesis** that originated in this project's own reasoning — it is *not* drawn from a ratified document, and a credible alternative centers the domain on **Knowledge** instead. The open question, its alternatives, and the current recommendation live in [../06-open-questions/OQ-001-domain-center.md](../06-open-questions/OQ-001-domain-center.md). Until that resolves through governance ([OQ-006](../06-open-questions/OQ-006-governance-model.md)), this document is the *current understanding*, not the final word.

Two consequences define the shape of the domain:

1. **The model is not a domain concept.** A faithful description of Foundry never needs to mention an LLM. Models, and the machinery that runs them, are *substrate* — see [system-context.md](system-context.md) and the mechanism terms in the glossary.
2. **The Pipeline is not the center.** A predeclared graph is one **Strategy** for producing an Act. Exploratory, deliberative, adaptive, and human-driven work are equally valid Strategies. Centering the domain on any one Strategy would re-create rigidity the product exists to avoid.

## The domain concepts and how they relate

An **Act** is composed of, and connected to, the following:

- **Intent** — every Act records the reason it exists. Accountability and "why did we do this?" trace back to Intent.
- **Strategy** — every Act is produced by some Strategy. The Strategy is pluggable; the Act is invariant.
- **Evidence** — every Act accumulates Evidence: what was *considered* and what was *checked*. Evidence is what distinguishes a trustworthy Outcome from raw output.
- **Outcome** — an Act yields an Outcome: a proposed transition to Project State (code or Knowledge), or a determination, or nothing.
- **Judgment** — every consequential Act passes a Judgment: a verdict on its Evidence plus an accountable acceptance or rejection.
- **Authority** — every Judgment is owned by an Authority who answers for it.

```
            Intent ─────┐
                        ▼
        Strategy ──▶  [ ACT ]  ──yields──▶ Outcome ──(if accepted)──▶ Project State
                        │  ▲                                              │
                 accumulates                                          deposits into
                        ▼  │                                              ▼
                    Evidence ──judged-by──▶ Judgment ──owned-by──▶ Authority      Knowledge
                                                                                     ▲
                                              Evidence draws on ──────────────────────┘
```

**Project State** is what Acts evolve: a project's **code** and its **Knowledge**. **Knowledge** is the durable medium — it both *informs* Acts (as the source of Evidence) and *grows from* accepted Acts (the compounding loop). Knowledge has its own document because it is the durable capital the product is built to accumulate ([knowledge.md](knowledge.md)).

The **Record** is not a separate concept: because every Act is immutable, the set of all Acts *is* the history. Audit, replay, and Knowledge growth are all read off that history.

## What every feature reduces to

Every present and future capability is an Act, with no remainder:

| Feature | Intent | Outcome |
|---|---|---|
| Implement a feature | "add X" | code change |
| Review a change | "assess this" | a determination (Knowledge artifact) |
| Draft an RFC / ADR | "decide X" | Knowledge change |
| Security / performance pass | "find risks" | findings |
| Prepare a release | "ship" | a high-blast-radius state transition |
| Update project knowledge | "record what we learned" | Knowledge change |

If a proposed feature cannot be expressed as an Act, that is a signal the domain model — not the feature — needs review.

## Open domain questions

This model rests on unresolved questions. They are **owned by the open-questions tier** (not restated here) so deliberation never leaks into canonical prose:

- [OQ-001 — The domain center: Act vs Knowledge](../06-open-questions/OQ-001-domain-center.md)
- [OQ-002 — Pipeline as Strategy](../06-open-questions/OQ-002-pipeline-as-strategy.md)
- [OQ-007 — Is the vocabulary right?](../06-open-questions/OQ-007-canonical-terminology.md)

Whether **Strategy** is a domain concept or merely the boundary to mechanism, and whether **rejected/abandoned Acts** deposit into Knowledge, are sub-questions of OQ-001/OQ-002 and must not be silently resolved in implementation.
