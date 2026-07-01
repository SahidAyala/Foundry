# Architecture — System Context

> **Maturity: PROVISIONAL.** The four boundaries are the current working model; the scale/concurrency and "remote is additive" claims are unproven (see end of doc).
>
> **This document answers exactly one question: _What are the system boundaries?_**
> It does not define the domain ([domain.md](domain.md)) or execution mechanics ([execution.md](execution.md)). Terms are defined in [../05-reference/terminology.md](../05-reference/terminology.md).

## Boundaries, not layers

Foundry is organized by four conceptual **boundaries**. Each is a line work crosses, not a stack of layers.

### 1. The determinism boundary
- **Inside (deterministic core):** the **Engine** (control flow), **Gate** evaluation, **Budget** enforcement, **Router** policy, and the **Record**.
- **Outside (non-deterministic edge):** **Executors** (models, tools, humans), **Validators** (which invoke real tools), and **Context Sources** (which read the live world).
- **The rule:** everything crossing the boundary is recorded by content, so the non-deterministic edge is *reproducible by replay* even though it is not deterministic. This is how reproducibility is promised without promising deterministic model output.

### 2. The trust boundary
- An **Outcome** is **untrusted** when produced and becomes trusted only after verification and an **Authority**'s acceptance ([trust.md](trust.md)). This boundary is internal to every **Act**. The human is the trust **Authority** at this boundary.

### 3. The durability boundary
- **Durable & owned:** Authored **Knowledge**, the **Record** (the set of Acts), and the recorded form of accepted Outcomes.
- **Disposable & recomputable:** Derived **Knowledge**, **Context** bundles, indexes, and all **Executor** backends.
- This boundary *is* the "knowledge is capital, the model is substrate" thesis made structural: the durable side must remain meaningful if every Executor vanished.

### 4. The extension boundary
- **Open/replaceable:** Strategies, Executors, Validators, Context Sources, Router policies ([extensibility.md](extensibility.md)).
- **Closed/conservative:** the Engine, the Record, Gate/Judgment semantics, and the Act lifecycle.

## What is below the system (substrate)

The following are explicitly *outside the domain* and below the system's conceptual surface — replaceable infrastructure the Engine and Strategies employ:

- **Models / Providers** — one kind of Executor. Foundry's identity never depends on any one.
- **Tools** — deterministic Executors.
- **Storage** — the implementation of the Record's durability and of caches.

A correct statement of what Foundry *is* mentions none of these.

## The human

The human is not external tooling; the human is the **Authority** (the source of Intent and the holder of accountability) and spans the trust boundary. Foundry amplifies engineering judgment; it does not replace it.

## Deployment posture

- **Local-first by default.** A single project, on a developer's machine or in CI, with the same behavior in both.
- **Remote/shared operation is additive, not a different architecture** — it is a later deployment of the same core.

> **Unresolved (human decision required):** the claim that remote/shared and large-scale (many projects, many concurrent Acts) operation requires no redesign is **plausible but unproven** — only some seams have been shown to be remoteable, and a concurrency model is not yet defined. Tracked in [../00-overview/roadmap.md](../00-overview/roadmap.md).
