# OQ-002 — Is the Pipeline one Strategy or the system's spine?

**Maturity: OPEN QUESTION** · informs [../02-architecture/execution.md](../02-architecture/execution.md) (PROVISIONAL)

## Problem
Foundry was originally conceived "pipeline-first" (declarative YAML graphs). A later review demoted the Pipeline to *one Strategy among several*. Is that demotion correct, or does it discard a genuinely central organizing idea?

## Context
The original product brief and roadmap were pipeline-centric. The demotion came from this project's own reasoning (archived review), prompted by a direct challenge to pipeline-centrality. It is a working hypothesis, not a ratified decision.

## Alternatives
1. **Pipeline as one Strategy** — the Engine produces Acts; a predeclared graph is one way; adaptive/deterministic/human-driven are others.
2. **Pipeline as the spine** — everything is a graph; "adaptive" work is modeled as dynamically-generated graphs.
3. **Two first-class strategies** — graph and adaptive are both privileged; others are niche.

## Arguments
- *For demotion:* much engineering (debugging, design, review) is not predeclared-graph-shaped; forcing it into a graph re-creates rigidity. The model is one executor; the pipeline is one strategy — symmetric lesson.
- *Against demotion:* a single, inspectable, replayable graph format may be the very thing that makes runs auditable and reproducible; multiple strategies multiply the surface that must be made trustworthy.
- *Unknown:* whether non-graph strategies can meet the same replay/audit guarantees as cheaply as a graph.

## Open questions
- Can an adaptive strategy be made as replayable/auditable as a predeclared graph without effectively becoming one?
- Does supporting many strategies dilute the determinism story (control flow ownership)?

## Current recommendation
Keep **Pipeline as one Strategy** as the working model, but treat the *graph* as the **first, best-supported** strategy and prove the others only when a real use case demands them (depth-before-breadth). PROVISIONAL.

## Status
**OPEN.** Closely coupled to [OQ-003](OQ-003-replay-across-versions.md) and [OQ-004](OQ-004-validator-determinism.md).
