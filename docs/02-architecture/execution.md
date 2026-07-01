# Architecture — Execution

> **Maturity: PROVISIONAL.** Current working model; "Pipeline is one Strategy" is a working hypothesis ([../06-open-questions/OQ-002-pipeline-as-strategy.md](../06-open-questions/OQ-002-pipeline-as-strategy.md)).
>
> **This document answers exactly one question: _How does the system produce outcomes?_**
> It does not define the domain concepts ([domain.md](domain.md)) or how trust is established ([trust.md](trust.md)). Terms are defined in [../05-reference/terminology.md](../05-reference/terminology.md).

## The principle: produce Acts, by any Strategy

The system's job is to produce **Acts** (see [domain.md](domain.md)). The **Engine** drives that production; a **Strategy** determines *how* a given Act is produced. The Engine owns control flow; a Strategy fills in the work; no model ever decides what happens next.

This separation is the whole point of the execution layer: it lets the *how* vary (a graph, an adaptive agent, a deterministic procedure, a human-driven session) while the *what* (a justified, accountable, recorded Act) stays invariant.

## The execution lifecycle of an Act

Regardless of Strategy, producing an Act follows the same shape:

1. **Open** — an Act is opened from an **Intent**.
2. **Gather Evidence (considered)** — the Engine assembles **Context** for the work from Knowledge and the world, bounded by **Budget**.
3. **Produce** — the chosen Strategy does the work, drawing on **Executors** selected by the **Router** to satisfy required **Capabilities**. The result is one or more **Artifacts**. *Output is untrusted at this point.*
4. **Gather Evidence (checked)** — **Validators** check the Artifacts, producing findings.
5. **Judge** — a **Gate** evaluates findings (machine verdict); an **Authority** accepts or rejects (accountable verdict). Together these form the **Judgment**.
6. **Repair (bounded)** — on a `repair` verdict, findings feed back into the Strategy; bounded by **Budget** and a must-make-progress rule.
7. **Apply** — an accepted **Outcome** is applied to Project State (code or Knowledge).
8. **Record** — every step is written to the immutable history as part of the Act.

## Strategies

A **Strategy** is the pluggable means of production. Foundry recognizes (at least) these kinds; the set is extensible (see [extensibility.md](extensibility.md)):

- **Pipeline** — a predeclared directed-acyclic graph of **Steps**. Best when the work's shape is known in advance (e.g. implement → test → review → commit). *Historically Foundry was conceived as pipeline-first; it is not. The Pipeline is one Strategy.*
- **Adaptive** — the next unit of work is chosen from the last result. Best for exploratory work (e.g. debugging). Still Engine-driven and fully recorded; *adaptive is not uncontrolled.*
- **Deterministic procedure** — a fixed, model-free sequence.
- **Human-driven** — an interactive session where a person performs or directs the work.

All Strategies emit the same thing — an Act with Evidence, an Outcome, and a Judgment — so the trust machinery ([trust.md](trust.md)) and the Record are identical across them.

## Executors and routing (substrate)

**Executors** are the resources that perform work: models, deterministic tools, humans. They are *substrate* — replaceable, below the domain line ([system-context.md](system-context.md)). The **Router** places each unit of work on an Executor by matching required **Capabilities** to advertised ones under a policy (cost / latency / quality / privacy) with failover. Routing is a deterministic function of capabilities and policy; it never makes control-flow decisions.

## Determinism and replay

- **Control flow is owned by the Engine**, never by a model.
- **Replay re-derives the deterministic and replays the recorded:** units of work declared deterministic are re-executed and must reproduce identical Artifacts; non-deterministic Executor outputs are replayed from the Record, never re-derived for the identity guarantee. This is *process* determinism, not output determinism.

> **Unresolved (human decision required):** the *cross-version* scope of the replay guarantee — whether an Act replays identically under a future Engine version — is not yet decided. Tracked in [../00-overview/roadmap.md](../00-overview/roadmap.md) and gated by a pending ADR (see [../03-adrs/README.md](../03-adrs/README.md)).
