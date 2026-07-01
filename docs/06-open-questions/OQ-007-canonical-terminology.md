# OQ-007 — Is the proposed vocabulary the right one?

**Maturity: OPEN QUESTION** · informs [../05-reference/terminology.md](../05-reference/terminology.md) (PROVISIONAL)

## Problem
The refactor introduced a vocabulary — **Act, Intent, Strategy, Evidence, Judgment, Authority, Outcome, Knowledge** (domain) and **Engine, Pipeline, Step, Executor, Router, …** (mechanism) — and retired *Workflow, Stage, Provider, Skill, Runtime/Kernel*. Is this the right vocabulary, or is it premature naming that will calcify?

## Context
Some terms are user-originated (Pipeline, Executor, "LLMs are one executor" came from the product brief). Others are entirely this project's proposals: **"Act" and "Engine" were coined here**, not drawn from any accepted document. Naming the central abstraction is one of the highest-leverage, hardest-to-reverse documentation decisions, so it deserves explicit scrutiny.

## Alternatives
1. **Adopt as-is** — accept Act/Engine/Strategy/etc. now.
2. **Keep mechanism terms, defer domain terms** — use Pipeline/Executor (user-originated) but hold off canonizing "Act" until the domain center (OQ-001) resolves.
3. **Different central noun** — e.g. Change, Decision, Engagement, Episode instead of Act.

## Arguments
- A shared vocabulary now prevents drift and is cheap to rename while there is no code.
- But canonizing a *coined* central noun ("Act") before the domain center (OQ-001) is decided risks entrenching a term the model may outgrow.
- Once code and external references use a term, renaming gets expensive (the very "compatibility surface" concern the project cares about).

## Open questions
- Should the *domain* vocabulary wait on OQ-001, while the *mechanism* vocabulary (already user-originated) proceeds?
- Is "Act" the best central noun, or merely the first that fit?

## Current recommendation
**Adopt provisionally** to stop drift, but mark domain terms PROVISIONAL and bind their finalization to OQ-001. Treat renaming as cheap until first implementation; re-confirm the central noun before any external interface uses it. PROVISIONAL.

## Status
**OPEN.** Coupled to [OQ-001](OQ-001-domain-center.md).
