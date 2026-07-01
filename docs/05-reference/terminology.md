# Terminology

> **Maturity: PROVISIONAL vocabulary.** This file is the **single source of truth for what every Foundry term means** — but the *domain* vocabulary itself (Act, Strategy, Evidence, …) is a working proposal, not ratified, and its central noun depends on an open question ([../06-open-questions/OQ-007-canonical-terminology.md](../06-open-questions/OQ-007-canonical-terminology.md), [OQ-001](../06-open-questions/OQ-001-domain-center.md)). Use these terms — there must be exactly one vocabulary — but treat them as renameable until first implementation. Retired terms (bottom table) are **REJECTED**.
>
> Every other document uses these terms and must not redefine them. If a definition changes, it changes *here*, once. Relationships live in [concepts.md](concepts.md); rules in [invariants.md](invariants.md); the domain narrative in [../02-architecture/domain.md](../02-architecture/domain.md).
>
> Terms are grouped into **Domain** (what the system *is about*) and **Mechanism** (how it is implemented — *below the domain line*, replaceable). A faithful description of Foundry's purpose uses only Domain terms.

---

## Domain terms

**Foundry** — A system for turning human intent into engineering outcomes that can be *trusted* (justified, accountable, recorded) and that *compound* (the project learns from each one).

**Project State** — Everything true about a project that Foundry helps evolve: its **code** and its **Knowledge**. Acts propose transitions to Project State.

**Act** — *(proposed central unit of the working domain model — see [OQ-001](../06-open-questions/OQ-001-domain-center.md))* A bounded, immutable, accountable episode of engineering that carries an **Intent** toward an accepted **Outcome**, together with the **Evidence** that justifies it. An Act *is its own record*. On the current model, every feature (implement, review, design, secure, release, learn) is an Act.

**Intent** — The recorded reason an Act exists — what was wanted, and why. The root to which accountability traces.

**Strategy** — The pluggable *means* by which an Act is produced. A predeclared graph (a **Pipeline**), an adaptive agent, a deterministic procedure, and a human-driven session are all Strategies. No single Strategy is privileged; the graph is one option among several.

**Evidence** — The union of everything *considered* (the **Context** drawn from Knowledge and the world) and everything *checked* (verification results) for an Act. Evidence is what makes an Outcome trustworthy rather than merely produced.

**Judgment** — The verdict on an Act's Evidence and the accountable acceptance or rejection of its Outcome. Carries an **Authority**.

**Authority** — The human (or explicitly delegated policy) who *owns* a Judgment and answers for it. Accountability never leaves an Authority; this is why Foundry defaults to approval, not autonomy.

**Outcome** — The proposed transition to Project State that an Act yields: a change to **code**, a change to **Knowledge**, or a determination *about* something (e.g. a review's assessment). Possibly empty. It is what an Authority reviews and, once accepted, what is applied.

**Knowledge** — The durable, owned, justified model a project has of itself: its decisions, conventions, rationale, and understanding of its own code. The medium in which value compounds. Two strict layers:
- **Authored Knowledge** — created and owned by humans (or proposed by Foundry and human-approved); the source of truth; portable; survives any model generation.
- **Derived Knowledge** — recomputable from primary sources; a disposable cache; never the source of truth.

**Record / History** — The immutable preservation of Acts. Audit, replay, and the growth of Knowledge are all derived from it. (Not a separate store conceptually — the immutable set of Acts *is* the record.)

---

## Mechanism terms (below the domain line — replaceable, not part of the domain)

> These implement the domain. A correct domain description never needs them. They are defined here only so implementation docs share one vocabulary. See [concepts.md](concepts.md) for how each maps to the domain.

**Engine** — The component that produces Acts: it drives a Strategy, gathers Evidence, obtains a Judgment, applies an Outcome, and writes the Record. Owns control flow.

**Pipeline** — *One Strategy*: a predeclared directed-acyclic graph of **Steps**. Optimal when the shape of the work is known in advance. Not the center of the system.

**Step** — One unit of work inside a Pipeline strategy. (Replaces the historical term "Stage".)

**Executor** — A resource that performs a Step's work: a model, a deterministic command/tool, or a human action. *Substrate* — replaceable. A model is one Executor kind.

**Router** — Selects, per unit of work, which Executor and backend satisfies the required **Capabilities** under policy (cost / latency / quality / privacy), with failover.

**Validator** — A deterministic-first check over an Outcome's artifacts that produces structured findings; contributes to Evidence. Pure: reads and reports, never mutates.

**Gate** — A deterministic decision over findings that yields pass / fail / repair; the machine-verification half of a Judgment.

**Artifact** — A content-addressed output (a diff, file, report, message). Identity is its content; the unit of provenance and replay.

**Budget** — An enforceable ceiling (cost / time / iterations) on an Act, enforced as a constraint, not reported as a metric.

**Context** — The assembled, attributed, budgeted knowledge handed to a unit of work for one execution; the "considered" half of Evidence.

**Capability** — A declared, negotiable property an Executor advertises and a unit of work requires (e.g. tool use, context size, runs-locally). The matching currency between work and Executors.

---

## Archived terminology (do not use in canonical docs)

| Archived term | Replaced by | Why |
|---|---|---|
| Workflow / WorkflowDefinition / WorkflowRun | **Act** (domain) / **Pipeline** (mechanism) | "Workflow" conflated the unit of trust with one execution strategy |
| Stage | **Step** | Graph-native naming under the Pipeline strategy |
| Provider | **Executor** (a model is one Executor) | Model access is not a privileged execution path |
| Skill | reusable **Act** template / reusable Pipeline component | Derivative of Act; not irreducible |
| Runtime / Kernel | **Engine** | Plain product noun |

Historical documents using these terms live under [`../archive/`](../archive/) and are not canonical.
