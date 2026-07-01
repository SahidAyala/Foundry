# Contributing Guide

> How to contribute to Foundry. Read [../../README.md](../../README.md) and [../00-overview/](../00-overview/) first. Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Before you contribute

1. Read the [vision](../00-overview/vision.md) and [principles](../00-overview/principles.md) — contributions are evaluated against them.
2. Learn the canonical vocabulary in [../05-reference/terminology.md](../05-reference/terminology.md). **Use canonical terms only**; never reintroduce retired ones (Workflow, Stage, Provider, Skill, Runtime).
3. Understand the [invariants](../05-reference/invariants.md). A change that breaks one is wrong by definition.

## What makes a good contribution

- It is a vertical slice that leaves the repository working ([development.md](development.md)).
- It strengthens the durable layer (process, knowledge, trust, record) rather than deepening dependence on any one model.
- It comes with tests and, where it touches a substrate boundary, conformance coverage.
- It preserves accountability and provenance — it does not make output look more trusted or autonomous than it is.

## Disagreeing with the architecture

Disagreement is welcome and is itself a contribution. The right way to challenge a principle, invariant, or architecture statement is to open a decision proposal (an RFC or ADR) that cites the document and section — **not** to route around it in code. A principle that survives challenge is stronger; one that cannot should change.

## Decision precedence

When documents disagree, the higher-precedence document wins (see [../../AGENTS.md](../../AGENTS.md)). If you find a contradiction, report it; never silently resolve it in code.

## Governance status

There is **not yet a ratified process** for accepting RFCs/ADRs (see [../00-overview/roadmap.md](../00-overview/roadmap.md), open decision 1). Until there is, treat architectural decisions as provisional and escalate conflicts rather than settling them.
