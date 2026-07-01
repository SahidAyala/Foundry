# Irreducible Domain Model — First-Principles Review

| | |
|---|---|
| **Purpose** | Discover Foundry's irreducible domain model before any architecture is rewritten. Not a design. |
| **Method** | Start only from the product vision (RFC-0001) and the desired user experience. Discard all prior terminology — Pipeline, Workflow, Stage, Skill, Runtime, Provider, Executor — and re-derive. Prefer replacing terms over preserving them. |
| **Date** | 2026-06-29 |
| **Central claim** | The fundamental abstraction is **the Act** — a justified, accountable transition of project state. **The Pipeline is not the center; it is one Strategy for producing an Act.** |

---

## 1. What Foundry is for, reduced to one sentence

Stripping the RFC to its load-bearing core: Foundry exists to **turn human intent into engineering outcomes that can be trusted, because they are justified, accountable, and recorded — and that compound, because the project learns from each one.**

Every other claim in the RFC is a consequence of that sentence:
- *Trust is the product* → outcomes must be **justified** (evidence) and **accountable** (owned by a human).
- *Process is the unit of work; it compounds* → outcomes must be **recorded** and feed **knowledge**.
- *The model is replaceable substrate* → nothing in the core may depend on *how* the work was produced.
- *Approval, not autonomy* → a human must remain the **authority** over consequential outcomes.

So the domain is not "AI doing engineering." The domain is **the responsible evolution of a project's state.** AI is one way to propose evolutions; it is not part of the domain at all (it is substrate — §6).

## 2. The desired user experience, reduced

What does a user actually *do* with Foundry, stripped of mechanism?

1. They express **what they want**.
2. Something **does engineering work** on their behalf.
3. They are shown **a proposed change and why it should be trusted** (what was considered, what was checked, what it cost).
4. They **accept or reject** it — and are accountable for that choice.
5. The project **is better positioned for the next time** because what happened was kept.

Notice what is *absent* from this loop: the user does not care whether a pipeline, an adaptive agent, a deterministic script, or a human produced step 2. They care about steps 3–5: **the proposal, its justification, their judgment, and the record.** This is the first clue that the production mechanism (pipeline) is not the center — the *justified, reviewable, accountable, recorded unit* is.

## 3. The elimination test — what is actually irreducible

A concept is irreducible only if removing it stops Foundry from being Foundry. Apply the test honestly:

| Remove… | Does Foundry survive? | Verdict |
|---|---|---|
| The LLM / any specific executor | Yes — a deterministic, audited automation platform remains (the RFC *insists* on this) | **Substrate, not domain** |
| The Pipeline (predeclared graph) | Yes — adaptive or human-driven work that is justified and recorded is still Foundry | **Not the center** |
| Verification of outputs | No — outputs become untrusted; "trust is the product" collapses | **Irreducible** |
| The human as accountable authority | No — it becomes an autonomous agent, which the RFC explicitly refuses | **Irreducible** |
| The durable record of what happened and why | No — no audit, no replay, no accountability, *and knowledge cannot compound* | **Irreducible** |
| Knowledge as durable capital | No — work stops compounding; it reverts to a "fancier chat box" | **Irreducible** |
| Intent | No — nothing initiates, and accountability has no root cause | **Irreducible** |

The survivors are **intent, justification (evidence + verification), accountability, the record, and knowledge** — *not* pipeline and *not* the model. Any domain model that centers pipeline or executor has centered a survivor of neither column.

## 4. Challenging the Pipeline directly

A Pipeline is a **predeclared graph of steps** — a *strategy* that is optimal when the shape of the work is known in advance: implement → test → review → commit.

But the vision is to orchestrate the *entire* engineering lifecycle, and much of engineering is not predeclared-graph-shaped:
- **Debugging** is exploratory — the next step depends on the last result; the graph is discovered, not declared.
- **An RFC or design** is deliberative — it converges, it does not flow.
- **An adaptive agent** decides its own next action.
- **A human-in-the-loop session** is interactive.

If the Pipeline is the center, then *all* of these must be forced into a predeclared graph — re-creating, at the orchestration layer, exactly the rigidity the RFC warns against, and re-centering the system on a **mechanism** the same way the old model wrongly centered it on the **model**. The lesson of "the model is one executor among many" has a twin: **the pipeline is one strategy among many.**

Crucially, dethroning the Pipeline does **not** weaken determinism or control. The invariant the RFC actually requires is that **control flow is owned by the platform, not by a model, and that the act is recorded and replayable** — and that invariant holds for an adaptive strategy just as well as a graph strategy, as long as the platform (not the model) drives it and records every decision. "Adaptive" is not "uncontrolled." So the Pipeline can be demoted without sacrificing anything the RFC demands.

**Conclusion:** Pipeline is a **Strategy** — first-class as an *option*, never as the *center*.

## 5. The irreducible domain model

The smallest set of first-class concepts on which every future feature depends. Names are chosen for the concept, not inherited; where a name is secondary I state the semantic target so it can be renamed without losing the idea.

### The center

- **Act** — *the fundamental abstraction.* A bounded, **immutable**, **accountable** episode of engineering that carries an **Intent** toward an accepted **Outcome**, together with the **Evidence** that justifies it. An Act *is its own record* — there is no separate ledger concept; the immutable history of Acts **is** the audit trail, the replay source, and the substrate from which Knowledge grows. Every feature Foundry will ever have — implement, review, design, secure, release, learn — is an Act. *(Semantic target: "a justified, accountable transition of project state." If "Act" reads thin, the team may name it otherwise; the concept must not change.)*

### What an Act necessarily contains

- **Intent** — the recorded reason the Act exists. First-class because accountability and "why did we do this?" both trace to it, and because how raw human desire becomes Intent is itself an unsolved, important question (RFC OQ3).
- **Strategy** — the **pluggable means** by which the Act is produced. A predeclared graph (the former "Pipeline"), an adaptive agent, a deterministic procedure, and a human-driven session are all Strategies. First-class precisely so that no single strategy — least of all the graph — is privileged. *The Strategy is the seam between the domain and all mechanism below it.*
- **Evidence** — the union of *everything considered* (the knowledge and world-state brought to bear) and *everything checked* (verification results). Evidence is what converts an Outcome from "output" into "trustworthy output." It subsumes what older models split into "context" and "validation."
- **Judgment** — the verdict on the Evidence **and** the accountable acceptance or rejection. It carries an **Authority**: the human (or explicitly delegated policy) who *owns* the decision and answers for it. First-class because "approval, not autonomy" is a non-negotiable value; the trust gate is strategy-independent — a graph and an agent face the identical Judgment.
- **Outcome** — the (possibly empty) **proposed transition to the project's state** that the Act yields: a change to **code**, or a change to **Knowledge**, or a determination *about* something (e.g. a review's assessment). It is what the user reviews and what, once judged, is applied. First-class because applying a transition to the real world has blast radius and (ir)reversibility that the domain must reason about.

### The durable medium

- **Knowledge** — the project's **owned, justified model of itself**: its decisions, conventions, rationale, and understanding of its own code. Knowledge is *both* an input to Evidence (it informs Acts) *and* the product of accepted Outcomes (Acts deposit into it). It is the medium in which value **compounds** — the closing loop of the vision — and the durable capital that outlives any model generation. The authored core of Knowledge is owned and portable (the durability promise); the rest is recomputable.

That is the whole irreducible set: **Act (carrying Intent, produced by a Strategy, justified by Evidence, decided by a Judgment under an Authority, yielding an Outcome) evolving Knowledge.**

## 6. What falls *below* the domain line

The strongest evidence that this model is correctly centered: **a faithful domain model of Foundry never needs to mention an LLM.** These are substrate/mechanism a *Strategy* employs, not domain concepts:

- **Models, providers, executors, routers** — resources a Strategy uses to do work. The RFC makes the model replaceable substrate; the domain model honors that by not naming it at all.
- **Steps, stages, nodes, graphs** — internal structure of the *graph Strategy* specifically; invisible to other strategies.
- **Storage, ledgers-as-databases, content-addressing** — the *implementation* of an Act's immutability, not the Act itself.
- **Skills** — a *named, reusable Act template* (a packaged Intent+Strategy+Evidence-requirements+Judgment-policy). Useful, but derivative of Act; not irreducible.

If a concept only matters to *one* Strategy, it is below the line. The Pipeline, the Stage, the Node, and the Executor are all below the line. This is the precise sense in which the prior "greenfield" review (which made Executor first-class) was operating one altitude too low: it modeled the dominant Strategy's mechanism as if it were the domain.

## 7. Coverage — every future feature is an Act

| Future feature | As an Act |
|---|---|
| Implement a feature | Intent="add X"; Strategy=graph or agent; Outcome=code change; Evidence=context+tests+lint; Authority approves |
| Review a PR | Intent="assess this"; Outcome=an assessment (a Knowledge artifact); Evidence=what was inspected |
| Draft an RFC / ADR | Intent="decide X"; Outcome=a Knowledge change; Evidence=options considered |
| Security / performance pass | Intent="find risks"; Outcome=findings; Evidence=what was scanned, by what |
| Prepare a release | Intent="ship"; Outcome=a high-blast-radius state transition; Authority gate is strong, irreversible |
| Update project knowledge | Intent="record what we learned"; Outcome=a Knowledge change; the closing loop |

Each decomposes cleanly into the same six concepts with no remainder. None requires "pipeline" to exist. That is the test of an irreducible model: the whole lifecycle expresses in it, and nothing smaller does.

## 8. Reasoning summary and what this supersedes

From first principles:
1. Foundry's purpose reduces to *responsible evolution of a project's state* — justified, accountable, recorded, compounding.
2. The elimination test leaves **intent, justification, accountability, record, knowledge** — and discards model and pipeline.
3. The UX loop centers on *the proposal, its justification, the judgment, the record* — not on the production mechanism.
4. Therefore the fundamental abstraction is the **Act** (the justified, accountable state transition), the **Pipeline is one Strategy** for producing it, and the **model is substrate** the Strategy employs.

This **supersedes the centering of all prior reviews**: the inherited "Workflow/Stage/Provider" model and even the prior greenfield "Pipeline/Node/Executor/Runtime" model both centered a *mechanism* (a graph and its executors). The irreducible model centers the *unit of trust*. When architecture is written, the graph machinery returns — but as the implementation of *one Strategy*, under the Act, not as the spine of the system.

### Open first-principles questions (genuinely unresolved, not deferrals)

1. **Is the Act or Knowledge the truer center?** The Act is the *dynamic* fundamental (every feature is one); Knowledge is the *durable* fundamental (the thing that compounds). They may be two faces of one concept — "Knowledge is the integral of all accepted Acts." Worth resolving before naming the system's spine.
2. **Is "Strategy" inside the domain or the boundary itself?** It is the seam where domain meets mechanism. Whether it is a first-class domain concept or merely the name of the boundary changes how much the core must know about *how* work is done.
3. **Does a rejected or abandoned Act still deposit into Knowledge?** If failure teaches (RFC §8.4), then even un-accepted Acts have durable value, and the record/Knowledge relationship is richer than "accepted Outcomes only."
4. **Where does Intent come from** (RFC OQ3) — is it captured, elicited, or inferred? The model says Intent is first-class but not where its boundary with the user sits.

---

_This is a discovery of the irreducible model, not a design. The finding: center the system on the **Act** — a justified, accountable transition of project state — and treat the **Pipeline as one Strategy** and the **model as substrate**. Architecture, when written, must implement the Act first and let every mechanism (graphs, executors, stores) be a replaceable servant of it._
