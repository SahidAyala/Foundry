# Vision

> **Maturity: PROVISIONAL** (strongly grounded in RFC-0001, which is itself unratified — see Status). Current statement of *why Foundry exists*. The full reasoning, alternatives, and tradeoffs are recorded in [../01-rfcs/RFC-0001-vision-and-product-philosophy.md](../01-rfcs/RFC-0001-vision-and-product-philosophy.md); this is the distilled version. Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## What Foundry is

Foundry turns human intent into engineering outcomes that can be **trusted** — justified, accountable, and recorded — and that **compound**, because the project learns from each one.

It is **not** an IDE, a chatbot, an autonomous agent, a prompt tool, or a wrapper around a model. It is a system for the *responsible evolution of a project's state*.

## The bet

The durable value in AI-assisted engineering is not generating code — that is commoditizing. It is everything around generation: the **process**, the **knowledge** it accumulates, the **verification** that makes output safe to depend on, and the **record** of how a system reached its current state. The model is the cheapest, most replaceable part of the stack; Foundry's value lives in the layer model vendors structurally cannot own — a project's own process and knowledge.

## Why a better model does not make this redundant

Trust and accountability are properties of *process*, not of intelligence. A model can become arbitrarily capable and still not answer what an organization must answer to merge a change: what informed it, what was checked, who approved it, can we reproduce it. Better models *increase* the volume of machine work that must be made trustworthy — so they make this layer more valuable, not less.

## Who it is for

Teams and individuals who treat engineering process and knowledge as assets worth compounding — and who accept a small up-front discipline in exchange for work that is auditable and that gets better over time. It is deliberately *not* optimized for throwaway work where no record is ever needed.

## What success looks like

- Work survives multiple model generations with the durable layer intact.
- A project can demonstrate that its knowledge accumulated and later work benefited.
- "How was this built and why should we trust it?" is answerable from the record alone.

## Status

The vision is accepted in direction but its founding RFC is **not yet ratified** and carries open questions (governance, adoption, the precise center of the domain). See [roadmap.md](roadmap.md) for the open decisions and [../01-rfcs/RFC-0001-vision-and-product-philosophy.md](../01-rfcs/RFC-0001-vision-and-product-philosophy.md) for the full record.
