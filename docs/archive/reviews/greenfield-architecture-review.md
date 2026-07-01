# Greenfield Architecture Review — Conceptual Model

| | |
|---|---|
| **Basis** | Product inferred from RFC-0001, `IMPLEMENTATION-ROADMAP.md`, the ADR gate/freeze reviews, and repository direction. `ARCHITECTURE.md` treated as **historical context only**. |
| **Scope** | Domain model · execution model · system boundaries. **No implementation** (no languages, storage, protocols, layouts, libraries, concurrency mechanisms). |
| **Stance** | Greenfield. Inherited concepts are kept only where still justified; renamed or dropped where the product moved on. |
| **Date** | 2026-06-29 |

---

## 1. The product, inferred (not from `ARCHITECTURE.md`)

Reading the RFC, the roadmap, and the implementation plans rather than the inherited blueprint, the product that is actually being built is:

> **A deterministic Runtime that executes declarative Pipelines of engineering work, where every unit of work is dispatched to a replaceable Executor, every output is verified before it is trusted, and the entire execution is recorded as durable, replayable, auditable history.**

Three load-bearing claims follow from the source documents, and they should drive the model:

1. **The Pipeline is the source of truth; the Runtime interprets it.** Control flow belongs to deterministic code, never to a model (RFC §6.2). The roadmap builds the Runtime *before* any AI and treats AI as a late-arriving plug-in.
2. **The model is one Executor among several.** The roadmap's explicit framing — "LLMs are only one possible executor," with shell and mock executors preceding any provider — means *model access is not a privileged execution path*. It is one implementation of a general execution abstraction.
3. **Durable value is process + knowledge + history; the executor is substrate.** RFC §6.3/V4 make the organization's process, knowledge, and the auditable record the assets that survive model generations. The architecture's *center of gravity* must be those, not the model integration.

This product is **executor-agnostic, graph-shaped, verification-gated, and history-first.** The inherited model is none of those cleanly — see §2.

---

## 2. Where the inherited architecture no longer matches the product

Diagnosis only; these are not preserved.

- **D1 — Model-centric leaf.** The inherited execution path is `Stage → Skill → Provider`, with **Provider as a top-level port**. This hard-wires "the thing that performs work is a model." The product says a model is *one* executor among shell/deterministic/human. The inherited model cannot express claim (2) without privileging the model.
- **D2 — Document-centric, not graph-centric.** The inherited core noun is **Workflow** with linear **Stages**. The product is a **Pipeline** that is a *graph* of units of work. "Workflow + Stage" undersells the graph and collides with the roadmap's pervasive "Pipeline/Node" vocabulary.
- **D3 — Selection is provider-specific, not general.** Routing lives inside the inherited "Provider System." The product needs to select *any* executor (including a deterministic tool or a human) by declared capability — a **general Router**, not a provider router.
- **D4 — "Where knowledge comes from" is not first-class.** The inherited model has "extractors" feeding a knowledge graph plus a separate "Context Engine." The product's quality hinges on *which sources a unit of work sees*; that origin set must be an open, first-class boundary — a **Context Source**.
- **D5 — History is conflated with cache.** The inherited storage view lumps the run record with disposable derived state. The product's trust/audit/replay promise (RFC V3, §6.5) requires the **Ledger** to be a durable, first-class concept, not a recomputable cache.
- **D6 — The determinism boundary is asserted, not drawn.** The inherited model claims validators are "deterministic functions" while running them through non-deterministic tools. The product needs an explicit **determinism boundary**: a deterministic core, non-deterministic work at the edge, and the Ledger recording across it.

---

## 3. First-class abstractions (the domain model)

The nouns below are the proposed ubiquitous language. Each is justified by the product, and its relationship to the others is stated. (Inherited terms it replaces are noted; full disposition in §6.)

### Durable, owned (the center of gravity)

- **Pipeline** *(replaces Workflow)* — a declarative, versioned **directed acyclic graph of Nodes** plus policy (gates, budgets, failure handling). It is data the Runtime interprets, not a program. The unit of *process* that compounds. First-class because the process is the product (RFC §6.2).
- **Node** *(replaces Stage)* — a single unit of work in the Pipeline graph. A Node declares *intent*, its required **Context**, the **Capabilities** its Executor must have, its expected **Artifacts**, the **Validators** that must pass, and its **Gate**. Edges express data/ordering dependencies. First-class because the graph vertex is the atom of execution and the anchor of provenance.
- **Skill** *(kept, redefined)* — a **reusable, versioned Node template**: a packaged capability contract (required context, required executor capabilities, expected artifacts, validators, gate) that Pipelines instantiate. It is the *reuse* mechanism for process, on a different axis from execution. Justified because shareable, improvable engineering capabilities are core to "process compounds" (RFC). A Pipeline can use raw Nodes; Skills are how Nodes become shared capital.
- **Knowledge** *(kept)* — the durable engineering capital: decisions, conventions, rationale, and a derived model of the codebase. Retains the **Authored** (source of truth, owned, portable) vs **Derived** (recomputable, disposable) split — strongly justified by RFC V4 and the durability thesis. Knowledge is *what Context Sources draw from*, not the per-Node selection itself.
- **Ledger** *(kept, elevated)* — the append-only, immutable, **durable** record of every Run: Node lifecycle, Context assembled, Executor calls (recorded), Artifacts produced, Validations, Gate verdicts, approvals. Audit, replay, resume, and observability are all *derived from* the Ledger. First-class and explicitly **not a cache** (resolves D5).

### Execution (the engine and how work is performed)

- **Runtime** *(replaces "kernel")* — the deterministic engine that interprets a Pipeline and drives execution: resolves the graph, sequences Nodes, requests Context, asks the Router to place each Node, evaluates Gates, enforces Budgets, runs bounded repair, and records to the Ledger. **The Runtime owns control flow; no Executor or model does.** The conservative, durable core.
- **Executor** *(new first-class; absorbs "Provider")* — the abstraction that **performs a Node's work**. A model is one Executor kind; a deterministic command/tool is another; a human approval step is another. Executors are the **replaceable substrate**, and their output is **untrusted until verified**. First-class because claim (2) is the whole point: making the model "one executor among many" requires the executor, not the model, to be the abstraction.
- **Router** *(generalized from "Provider Router")* — selects, per Node, which Executor (and which concrete backend, e.g. a specific model) satisfies the Node's required **Capabilities** under the active **policy** (cost / latency / quality / privacy), with failover. The Router makes *placement* decisions; it never makes *control-flow* decisions. First-class because executor-agnosticism is impossible without a general placement mechanism (resolves D3).
- **Capability** *(elevated from "capability descriptor")* — a declared, negotiable property an Executor advertises and a Node requires (e.g. "tool use," "context ≥ N," "runs locally"). The **matching currency** between Nodes and Executors. First-class because it is how provider-agnosticism works *without* collapsing to a lowest common denominator (RFC §6.3).
- **Budget** *(elevated to first-class)* — an enforceable ceiling (cost / time / iterations) on a Run or Node, enforced by the Runtime as a *constraint*, not reported as a metric. First-class because cost compounds alongside value (the RFC-review cost finding) and repair loops must be bounded.

### Verification and provenance (the trust machinery)

- **Validator** *(kept)* — a deterministic-first check over Artifacts producing structured **Findings**. Pure with respect to the workspace: it reads and reports, never mutates. First-class because verification is what manufactures trust (RFC §6.1).
- **Gate** *(kept)* — a deterministic decision over Findings (and signals) yielding **pass / fail / repair**. Where verification becomes control flow. First-class because the Runtime acts on Gate verdicts and their reproducibility is the cornerstone of process determinism.
- **Artifact** *(kept)* — any content-addressed output of a Node (a diff, a file, a report, a message). **Identity is its content.** The unit of provenance and replay. First-class because content-addressing is what makes history replayable and auditable.

### Context (what a unit of work is allowed to see)

- **Context Source** *(replaces "extractor")* — a first-class, **open/extensible** origin of knowledge: code structure, history, decisions, issues, external documents, the live world. First-class because the product's quality hinges on *which sources a Node sees*, and that set must be community-extensible (resolves D4).
- **Context** *(per-Node bundle; distinct from Knowledge)* — the assembled, attributed, budgeted set of knowledge handed to a Node for **one execution**. Immutable, content-addressed, fully provenanced (every element traces to a Context Source). Distinct from Knowledge (the store): Context is the *selection*, Knowledge is the *capital*.

### The human (accountability participant)

- **Approval / Human Authority** *(made explicit)* — the human is a first-class participant *at the boundary*: the source of intent and the holder of accountability (RFC §6.4). Outward or irreversible effects pass through an explicit approval the Runtime awaits. Not a UI detail — a domain concept, because "approval, not autonomy" is a non-negotiable value.

---

## 4. Execution model (conceptual lifecycle of a Run)

A **Run** is one execution of a Pipeline — a resumable, replayable state machine whose state is reconstructable from the Ledger. The Runtime drives each ready Node (respecting graph dependencies; independent Nodes may proceed concurrently):

1. **Context assembly.** The Runtime requests Context for the Node; Context Sources contribute; the result is an immutable, attributed Context fit to the Node's Budget.
2. **Placement.** The Router matches the Node's required Capabilities to an Executor and backend under the active policy; the Budget may refuse or downshift the placement.
3. **Execution.** The chosen Executor performs the work and produces Artifact(s). **Output is untrusted.**
4. **Verification.** Validators check the Artifact(s), producing Findings.
5. **Decision.** The Gate evaluates Findings to pass / fail / repair.
6. **Repair (bounded).** On *repair*, Findings feed back to a fixer Node; bounded by Budget and a must-make-progress invariant.
7. **Approval (where required).** Human authority gates outward/irreversible effects.

Every step is appended to the **Ledger** by content hash. Two invariants define the engine:

- **Control flow is the Runtime's.** Executors fill in node work; they never decide what runs next, whether a Gate passed, or whether work is done.
- **Replay re-derives the deterministic, replays the recorded.** Deterministic Nodes are re-executed and must reproduce identical Artifacts; non-deterministic Executor outputs are **replayed from the Ledger's record**, never re-derived for the identity guarantee. (This is the honest-determinism stance made structural; the *cross-version scope* of that guarantee is an open question — §7.)

---

## 5. System boundaries

The architecture is organized by four conceptual planes. Each is a *boundary*, not a layer.

### B1 — The determinism boundary
- **Inside (deterministic core):** Runtime control flow, Gate evaluation, Budget enforcement, Router *policy* (placement is a deterministic function of capabilities + policy), and the Ledger.
- **Outside (non-deterministic, recorded):** Executors (model/shell/human), Validators (which invoke real tools), Context Sources (which read the live world).
- **The rule:** everything crossing this boundary is recorded by content hash, so the non-deterministic edge is *reproducible by replay* even though it is not deterministic. This boundary is what lets the product promise reproducibility without promising deterministic model output.

### B2 — The trust boundary
- Executor output is **untrusted** the moment it is produced. It becomes trustworthy only after Validators + Gate, and consequential/outward effects only after Human Authority. The trust boundary is *internal to every Node*, and the Ledger records where on it each Artifact sits (produced → validated → gated → approved).

### B3 — The durability boundary
- **Durable & owned:** Pipelines, Skills, Authored Knowledge, the Ledger.
- **Disposable & recomputable:** Derived Knowledge, Context bundles (cacheable), Executor backends, and any index.
- This boundary *is* the "knowledge is capital, the model is substrate" thesis made structural: the durable set must remain meaningful if every Executor vanished. The Ledger sits on the durable side — explicitly resolving the inherited cache/audit contradiction (D5).

### B4 — The extension boundary
- **Open/replaceable (the substrate edge):** Executors, Validators, Context Sources, Router policies, Skills.
- **Closed/conservative (the durable core):** the Runtime, the Ledger semantics, Gate semantics, and the Pipeline/Node schema.
- Conceptually, *cleverness lives at the open edge; the closed core stays boring* (RFC §6.8). What an extender can add is precisely the substrate; what they cannot redefine is the engine, the history, or the meaning of a verdict.

The human spans B2 (as the trust authority) and is the origin of intent feeding B3 (authored knowledge and pipelines).

---

## 6. Disposition of inherited concepts

| Inherited concept | Disposition | Justification |
|---|---|---|
| Workflow / WorkflowDefinition / WorkflowRun | **Renamed → Pipeline / Pipeline Definition / Run** | Product is pipeline-graph-centric; resolves the duplicated core noun (D2) |
| Stage / StageRun | **Renamed → Node** | A Pipeline is a DAG of Nodes; graph-native vocabulary |
| Provider (top-level port) | **Demoted → an Executor backend, placed by the Router** | "LLMs are only one executor"; model access is not privileged (D1) |
| Provider Router | **Generalized → Router** | Placement applies to any Executor, not only models (D3) |
| Skill / SkillInvocation | **Kept, redefined → reusable Node template** | Reuse of process is core, but it is a Node spec, not an execution mechanism |
| Extractor | **Renamed → Context Source** | Elevates "where knowledge comes from" to a first-class, open boundary (D4) |
| Context Engine + Context Bundle | **Split → Context Sources (set) + Context (per-Node)** | Separate the open source-set from the immutable per-Node selection |
| Knowledge graph (Derived/Authored) | **Kept** | The durability thesis (RFC V4); the split is still correct |
| Run Ledger | **Kept, elevated to durable (not cache)** | Audit/replay/trust; resolves the cache/audit contradiction (D5) |
| Validator / Gate / Artifact | **Kept, first-class** | Already aligned with the trust machinery |
| Budget | **Elevated to first-class** | Cost compounds; repair must be bounded |
| Capability descriptor | **Elevated → Capability** | The matching currency for executor-agnosticism without LCD |
| Kernel | **Renamed → Runtime** | Product noun; "kernel" was implementation-flavored |
| (implicit human approval) | **Made explicit → Human Authority** | "Approval, not autonomy" is a non-negotiable value |

---

## 7. Open conceptual questions (model-level, not implementation)

1. **Is a Node's executor *kind* part of its identity or purely a Router outcome?** If a Node declares only Capabilities, the same Pipeline can run model-backed or deterministic-backed — powerful, but it blurs replay identity. Where the line sits between "Node requires capabilities" and "Node requires a specific executor kind" is unresolved.
2. **Cross-version replay.** Does the replay guarantee hold across Runtime versions, or only within one? The deterministic-re-derive path means an evolving Runtime can diverge on old Runs. This must be scoped at the model level before it is promised.
3. **Where does the boundary between Skill and Node actually fall?** If every reusable Node is a Skill, the two risk collapsing; if Skills carry behavior beyond a Node template, they re-introduce a second execution concept. The axis must be drawn cleanly.
4. **Is Human Authority a Gate, a Node, or a cross-cutting authority?** All three are defensible; the choice shapes how approval, accountability, and outward effects are modeled.
5. **Knowledge portability across the Runtime's own evolution.** Authored Knowledge is durable capital (RFC V4); the model must say what guarantees its readability as the Runtime changes — otherwise "durable" is aspirational.

---

_This is a conceptual model, not a design. It reorganizes the domain around what the product actually is — an executor-agnostic, graph-shaped, verification-gated, history-first Runtime — and keeps inherited concepts only where the product still justifies them. Implementation choices (language, storage, protocols, structure) are deliberately out of scope and belong to ADRs once this model is ratified._
