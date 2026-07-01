# Foundry — Architecture Planning Document

> **Status:** Draft v0 (foundational blueprint)
> **Audience:** Core maintainers, contributors, and architects
> **Nature:** Living document. Sections marked _(decision pending)_ require ratification via ADR before implementation.
> **One-line thesis:** Foundry is a **deterministic orchestration engine for software-engineering work** in which AI models are a replaceable compute substrate, and the durable assets are knowledge, workflows, validators, and an auditable run ledger.

---

## 0. How to read this document

This is not a feature list. It is an argument about where the durable value of an AI-native engineering platform actually lives, followed by an architecture that protects that value.

Throughout, I challenge assumptions from the brief where I believe a different framing produces a more maintainable system. Those challenges are marked **⚠ Assumption challenged** so they are easy to find and contest. Architecture is a series of trade-offs made explicit; every major one is recorded in §20.

---

## 1. Vision

Software development is currently being "automated" one prompt at a time. The unit of work is a conversation, the output is ephemeral, and the quality is a function of the operator's prompting skill on that particular day. This is a local maximum. It does not compound.

Foundry's vision is to make **engineering processes**, not prompts, the unit of work. A process is something you can name, version, share, test, replay, audit, and improve. When an organization improves how it does code review or how it writes RFCs, that improvement should be a versioned artifact that every future task inherits — not tribal knowledge re-typed into a chat box.

The long-term goal: **the entire engineering lifecycle expressed as composable, observable, replaceable stages**, where AI providers are interchangeable implementation details and the platform retains its value across model generations.

```
Idea → RFC → Architecture → Plan → Implement → Validate
     → Security → Performance → Docs → Commit → PR → Release
     → Deploy → Knowledge Update ─┐
        ▲                          │
        └──────── feedback ────────┘   (the loop is the point)
```

The lifecycle is drawn linearly for legibility, but the **Knowledge Update** stage closing back to the top is the most important arrow in the diagram. A platform that only executes work and forgets is a fancier prompt box. A platform that executes work *and gets better at the next task because of it* is a different category of thing.

---

## 2. Product Philosophy

### 2.1 Software Engineering > Prompt Engineering

The brief states this. I want to make it operational, because as a slogan it is ambiguous. It means:

1. **The control flow is owned by deterministic code, not by a model.** A model never decides "what stage runs next." A workflow definition does. Models fill in steps; they do not drive the machine.
2. **Every model output is an untrusted input until validated.** Validation is deterministic-first (§12). The model proposes; the engine verifies.
3. **The process is the artifact.** A Foundry run produces not just a diff, but a complete, replayable record of *how* that diff was produced and *why* it was accepted.

### 2.2 ⚠ Assumption challenged: "deterministic workflows"

> **Claim in brief:** Foundry produces "deterministic, reproducible engineering workflows."

LLMs are not deterministic and will not become so. Temperature 0 is not determinism (it is provider-, version-, and hardware-dependent). If we promise bit-identical model output we will ship a lie and erode trust. We must define determinism precisely or it becomes marketing vapor.

**Foundry's determinism is process determinism, not output determinism.** Concretely, Foundry guarantees:

- **Reproducible orchestration** — given the same inputs, the same stages run in the same order under the same gates. The *structure* of a run is deterministic.
- **Reproducible verification** — validators and gates are deterministic functions of artifacts. The same diff always yields the same pass/fail verdict.
- **Replayable runs** — every model call is recorded (request hash → response). A run can be re-executed from its ledger with cached responses, yielding identical artifacts. This is what makes workflows *testable* and CI-stable.
- **Provenance** — every artifact, context chunk, and decision is content-addressed and traceable to its source.

What Foundry explicitly does **not** promise: that two *live* runs against a live model produce identical text. We promise the process around the model is rigorous, observable, and replayable. This distinction is foundational; it shapes the entire architecture (record/replay, content-addressing, the run ledger) and it must be communicated honestly.

### 2.3 Models are replaceable; knowledge is not

The durable, defensible assets of Foundry — the things still valuable if every current LLM vanished tomorrow — are:

1. The **Knowledge Graph** of a project (decisions, structure, history, conventions).
2. The **Workflow and Skill definitions** (encoded engineering process).
3. The **Validators and Gates** (deterministic quality enforcement, mostly LLM-free).
4. The **Run Ledger** (auditable history of how the system reached every state).

Models are a swappable substrate behind the Provider port. The architecture must make the durable assets first-class and the model a detail. If a section of this document seems to over-invest in knowledge, context provenance, and validation relative to "AI features," that is intentional and correct.

### 2.4 Boring where it counts

Foundry orchestrates irreversible actions (commits, PRs, deploys) on people's real repositories. The core must be conservative, legible, and recoverable. Cleverness belongs in skills and providers (the replaceable edges), not in the kernel (the durable center).

---

## 3. Domain Model

Foundry is a domain-rich system; we model it with DDD because the brief explicitly values aggregates, bounded contexts, and a strong domain model. The model below is the *ubiquitous language* — these terms must mean exactly this everywhere in code, docs, and CLI.

### 3.1 Bounded contexts

```
┌──────────────────────────────────────────────────────────────┐
│                         FOUNDRY KERNEL                          │
│        (orchestration core — pure domain, no I/O)              │
└───────────────┬──────────────────────────────┬────────────────┘
                │ ports                          │ ports
   ┌────────────┴─────────┐        ┌────────────┴──────────────┐
   │  Knowledge Context    │        │  Capability  Verification  │
   │  (graph) (assembly)   │        │  (providers) (validators)  │
   └───────────────────────┘        └────────────────────────────┘
   ┌───────────────────────┐        ┌────────────────────────────┐
   │  Integration          │        │  Extension   Observability  │
   │  (git, VCS host, CI)  │        │  (plugins)   (audit/telem)  │
   └───────────────────────┘        └────────────────────────────┘
```

| Bounded context | Responsibility | Stable? |
|---|---|---|
| **Orchestration (Kernel)** | Workflow lifecycle, state machine, run ledger, budgets, gates | Very — changes slowly, breaking changes are expensive |
| **Knowledge** | Ingest, store, query the project knowledge graph | Stable schema, pluggable extractors |
| **Context** | Assemble task-scoped, budgeted, attributed context bundles | Algorithms evolve; the port is stable |
| **Capability (Provider)** | Normalize model access, capability negotiation, routing, failover | Adapters churn constantly; the port resists it |
| **Verification (Validator)** | Run validators, produce reports, evaluate gates | Stable port; many adapters |
| **Integration** | Git, VCS hosts, CI systems | Stable port; many adapters |
| **Extension** | Discover, load, sandbox, version-negotiate plugins | The contract IS the product surface |
| **Observability** | Traces, metrics, cost, audit | Stable; pluggable sinks |

The rule that makes this maintainable: **the kernel depends only on ports (interfaces). Everything that touches the outside world is an adapter.** This is hexagonal architecture, and it is the single most important structural decision in the platform (§5).

### 3.2 Core aggregates

Each aggregate has a clear consistency boundary and a single root.

- **Project** — root. The repository plus its resolved **Profile** (detected ecosystem, toolchains, conventions). Owns nothing transient; it is the stable context all work happens within. A monorepo is one Project with multiple scoped Profiles.
- **KnowledgeBase** — root over **KnowledgeItem**s and their edges. Two strict sub-layers: *Derived* (recomputable from sources — code structure, git history; a cache) and *Authored* (ADRs, RFCs, conventions, roadmap; source of truth). Never conflate them.
- **WorkflowDefinition** — declarative, versioned template of **Stage**s, gates, and policies. Immutable once published (new version = new identity).
- **WorkflowRun** — root over **StageRun**s. The live execution. A state machine with a durable, event-sourced history. Owns the **Budget** for the run.
- **SkillDefinition** — a reusable capability contract (required context, providers, outputs, validators, gates, retry policy). Invoked by stages.
- **SkillInvocation** — one execution of a skill within a StageRun.
- **ContextBundle** — an immutable, content-addressed set of attributed chunks fitting a token budget, produced by the Context Engine for a specific task.
- **Artifact** — any content-addressed output: a diff, a file, a report, an RFC, a commit message. Identity = hash of content. This is how provenance and replay work.
- **ValidationReport** — output of a Validator: a set of **Finding**s (severity, location, rule id, fixability).
- **Gate** — a policy that evaluates ValidationReports (and other signals) to a verdict: `pass | fail | repair`.
- **Provider** + **CapabilityDescriptor** — a model access point and its negotiated capabilities/costs.
- **RunLedger** — the append-only event log spanning a run. Not really an aggregate so much as the substrate; see §7.4.

### 3.3 Key relationships

```
Project ──has──▶ Profile(s) ──bind──▶ Validators, Toolchains, Extractors
Project ──has──▶ KnowledgeBase ──contains──▶ KnowledgeItems (Derived | Authored)
WorkflowDefinition ──composed of──▶ Stages ──invoke──▶ Skills
WorkflowRun ──executes──▶ StageRuns ──contain──▶ SkillInvocations
SkillInvocation ──requests──▶ ContextBundle (from Context Engine ← KnowledgeBase + code)
SkillInvocation ──calls──▶ Provider (via Router) ──produces──▶ Artifacts
Artifacts ──checked by──▶ Validators ──produce──▶ ValidationReports ──evaluated by──▶ Gates
Everything ──appended to──▶ RunLedger
```

---

## 4. Core Concepts

A concise glossary of the nouns a user and contributor must internalize. (Verbs/CLI in §16.)

- **Skill** — the atomic reusable engineering capability (e.g. `implement-feature`). Declarative contract + optional code behavior. §10.
- **Workflow** — an ordered, gated composition of stages that mostly invoke skills. Declarative. §7.
- **Stage** — one node in a workflow: invokes a skill (or a built-in op like `commit`), then evaluates a gate.
- **Gate** — a deterministic decision point that decides pass/fail/repair based on validation reports and budgets.
- **Context Bundle** — the budgeted, attributed knowledge handed to a model for one task. §9.
- **Knowledge Graph** — the durable, queryable model of the project. §8.
- **Provider / Router** — model access + selection/failover. §11.
- **Validator** — a (preferably LLM-free) checker producing structured findings. §12.
- **Profile** — an ecosystem binding (Node/Go/Rust/…); composes capabilities, toolchains, validators. §13-adjacent.
- **Artifact** — content-addressed output; the unit of provenance and replay.
- **Run Ledger** — append-only event log; the spine of audit, replay, resume, and rollback.
- **Budget** — enforced ceiling on tokens/cost/time/iterations for a run or stage. A first-class constraint, not a report.

---

## 5. System Architecture

### 5.1 Shape: a thin kernel, ports, and adapters

```
                    ┌─────────────────────────────┐
                    │           Surfaces           │
                    │  CLI · daemon/API · CI mode   │   (thin; no business logic)
                    └───────────────┬──────────────┘
                                    │
                    ┌───────────────▼──────────────┐
                    │           KERNEL              │
                    │  Workflow engine (state       │
                    │  machine) · Gate evaluation · │   (pure domain; deterministic;
                    │  Budget enforcement · Run      │    no network, no disk except
                    │  ledger · Saga/rollback        │    via ports)
                    └───────────────┬──────────────┘
                                    │  PORTS (stable interfaces)
   ┌──────────┬──────────┬─────────┼─────────┬──────────┬──────────────┐
   ▼          ▼          ▼         ▼         ▼          ▼              ▼
ProviderPort KnowledgePort ContextPort ValidatorPort VcsPort  ExtractorPort  ObservabilityPort
   │          │          │         │         │          │              │
 ADAPTERS (replaceable; many per port; built-in / trusted / community)
 Anthropic  GraphStore  Resolver  ESLint    git        ASTExtractor   OTel
 OpenAI     (SQLite+vec) impls    gosec     GitHub     GitExtractor   stdout
 Ollama     ...         ...       tsc       GitLab     IssueExtractor ...
 ...                              ...       ...        ...
```

**Why this shape and not the linear pipeline in the brief.**

> ⚠ **Assumption challenged:** The brief sketches `CLI → Workflow Engine → Context Engine → Knowledge Engine → Skill Engine → Provider Router → Providers` as a pipeline.

That ordering implies data flows through each "engine" once, in sequence. It does not. Context and Knowledge are **services consulted repeatedly** during execution (every skill invocation may request fresh context). The Provider Router is called dozens of times within a single workflow. Modeling these as pipeline stages would force the wrong dependencies and couple the kernel to the order of consultation.

The correct model is **layered hexagonal**: a deterministic kernel surrounded by ports, with knowledge/context/providers/validators as *services behind ports* that the kernel and skills call on demand. This keeps the kernel pure and testable (you can run a whole workflow against fake adapters), and it is what makes provider-agnosticism and extensibility fall out for free rather than being bolted on.

### 5.2 Process model

Three execution surfaces, **one engine**:

1. **CLI** — primary local experience. A single static binary.
2. **Daemon / local API** — long-running mode for editor integrations, watch mode, warm caches, and the local dashboard. Same engine, exposed over a local socket.
3. **CI mode** — the same binary invoked headlessly in a pipeline (§14).

The kernel is a library. Surfaces are thin wrappers. This is non-negotiable: business logic in a CLI command handler is how platforms rot.

### 5.3 State & storage

- **`.foundry/` in the repo** — portable, diffable, versioned. Holds *authored* knowledge (ADRs, conventions, workflow/skill definitions), project profile, and the durable parts of run history. Plain-text-friendly so it lives in git review.
- **Derived index (local cache)** — SQLite + a vector extension for embeddings and the derived knowledge graph. Rebuildable from sources; never the source of truth; gitignored.
- **Run ledger** — append-only event store (embedded; SQLite or an embedded log). Local by default; optionally backed by a remote store for teams (§14).
- **Secrets** — never in `.foundry`. OS keychain or external secret manager via the Secrets port (§17).

> ⚠ **Assumption challenged:** "single binary" vs "services." For v0.1–v1.0 Foundry should be a **local-first single binary**, not a distributed service. A server/SaaS is a *later* deployment of the same kernel, not a different architecture. Premature service decomposition would kill the project under operational weight before it has users. The hexagonal kernel makes the eventual split cheap; do it when teams demand shared state, not before.

### 5.4 ⚠ Language choice (decision pending — recommend Go)

This is architectural, not incidental, so it belongs here. Recommendation: **Go for the kernel and CLI.** Rationale: single static cross-platform binary (critical for a CLI + CI tool), excellent concurrency for fan-out orchestration, strong stdlib for processes/IO, a proven out-of-process plugin story (gRPC/`go-plugin`), fast cold start. Trade-off: less expressive type system than Rust, GC pauses irrelevant at this workload. The **plugin boundary must be language-agnostic** regardless (§15), so the community is never locked to Go. Ratify via ADR-0001.

---

## 6. Module Responsibilities

Mapping bounded contexts to concrete modules and their hard boundaries. The dependency rule is strict: **Surfaces → Kernel → Ports ← Adapters.** Nothing imports "upward"; adapters never import the kernel's internals, only the port interfaces and shared domain types.

| Module | Owns | Must NOT do |
|---|---|---|
| `kernel/orchestrator` | Workflow state machine, stage sequencing, gate evaluation, retries, saga/rollback | Talk to network/disk directly; know any provider name |
| `kernel/ledger` | Append-only event log, replay, resume | Interpret domain semantics beyond events |
| `kernel/budget` | Token/cost/time/iteration accounting & enforcement | Estimate costs (that's the provider adapter) |
| `domain/*` | Pure types: Skill, Workflow, Artifact, ContextBundle, Finding, Gate | Any I/O |
| `knowledge/graph` | Graph schema, queries, derived/authored separation | Decide *what* is relevant (that's context) |
| `knowledge/extractors/*` | Adapters: AST, git, issues, CI logs → graph | Persist outside the graph store |
| `context/resolver` | Task→bundle assembly, ranking, compaction, budget fit, provenance | Long-term storage (uses knowledge + cache) |
| `provider/router` | Capability match, routing policy, failover, health, budget gate | Translate to any specific API (adapters do) |
| `provider/adapters/*` | Anthropic/OpenAI/Gemini/Ollama translation, cost descriptors | Make routing decisions |
| `verification/runner` | Execute validators, aggregate reports | Implement specific checks |
| `verification/validators/*` | tsc, eslint, gosec, test runners, arch rules | Mutate code |
| `integration/git`, `integration/vcs/*`, `integration/ci/*` | Worktrees, commits, PRs, checks | Decide *when* to commit (orchestrator does) |
| `extension/runtime` | Discover, verify, sandbox, version-negotiate plugins | Trust plugins implicitly |
| `observability/*` | Traces, metrics, cost rollups, audit export | Be on the critical path (best-effort, async) |
| `surfaces/cli`, `surfaces/daemon` | Argument parsing, rendering, human interaction | Contain business logic |

---

## 7. Workflow Lifecycle

### 7.1 Workflows are declarative data, not code

> **Decision:** Workflow definitions are **declarative documents** (YAML authored, JSON canonical), version-controlled in `.foundry/workflows/`. They are *data the kernel interprets*, not programs. This guarantees they are inspectable, diffable, signable, and replayable. Behavior lives in the skills they invoke, not in the workflow.

A workflow declares: stages (each = a skill or built-in op + inputs), gates between/after stages, retry/repair policy, budget, and `on_failure` strategy. No loops or arbitrary logic in the document itself beyond bounded **repair loops** (a structured, first-class construct — see §7.5). A workflow that needs real control flow is a smell that the logic belongs in a skill.

```yaml
# illustrative shape, not a spec
workflow: implement-feature
version: 1
budget: { max_cost_usd: 5, max_iterations: 3 }
stages:
  - id: plan          ; skill: plan-feature
  - id: implement     ; skill: implement-feature
    gate: { all: [build.pass, lint.pass, tests.pass], on_fail: repair }
  - id: review        ; skill: review-changes
    gate: { all: [review.no_blocking], on_fail: repair }
  - id: commit        ; op: git.commit       ; gate: { all: [commit.conventional] }
  - id: pr            ; op: vcs.open_pr
on_failure: rollback
```

### 7.2 Lifecycle states

A `WorkflowRun` is a durable state machine:

```
PENDING → PLANNING → RUNNING ⇄ AWAITING_INPUT (human gate)
   │                    │  └────────────────────┐
   │                    ▼                         ▼
   │              REPAIRING ──(bounded)──▶ RUNNING
   │                    │
   ▼                    ▼
CANCELLED          COMPLETED | FAILED | ROLLED_BACK
```

Every transition is an event in the ledger. A run is **resumable** from any point because state is reconstructed by replaying events — a crashed or interrupted run can be continued, not restarted.

### 7.3 Isolation: never mutate the user's working tree

> **Decision:** All file-mutating work happens in an **isolated git worktree/branch** managed by Foundry, never in the user's checked-out working directory. The user always reviews a *diff*; they are never surprised by in-place edits. This single rule makes rollback trivial and trust possible.

### 7.4 The Run Ledger (event sourcing)

The ledger is the spine of the platform. Every meaningful occurrence — stage started, context assembled (with bundle hash), provider called (with request/response hash), artifact produced (with content hash), validation report, gate verdict, human approval — is an immutable, ordered event.

This one decision buys us, simultaneously:

- **Audit** (§17) — the ledger *is* the audit log.
- **Replay** (§2.2) — re-run with recorded provider responses.
- **Resume** — reconstruct state after interruption.
- **Observability** (§18) — traces and cost rollups are projections over events.
- **Rollback** (§7.6) — compensating actions are derived from the event history.

We do not build five subsystems; we build one event log and project five views from it.

### 7.5 Retries vs. repair loops (distinct concepts)

- **Retry** — same operation, transient failure (provider 429, network). Exponential backoff; may trigger provider failover (§11). Bounded by budget.
- **Repair loop** — a *first-class structured construct*: a gate fails, the structured findings are fed back as context to the same (or a "fixer") skill, which regenerates. Bounded by `max_iterations` and budget. This is how `implement → test fails → fix → test` works without unbounded model thrash. Repair is the workhorse of quality; retries are mere plumbing.

The danger is infinite repair loops burning money. Therefore repair is **always** bounded by both an iteration count *and* a cost budget, and it must show *measurable progress* (e.g., finding count strictly decreasing) or it aborts.

### 7.6 Rollback strategy (saga)

Stages register **compensating actions**. Because most work is in an isolated worktree, rollback is usually "discard branch." But outward-facing stages (opened PR, pushed tag, triggered deploy) need explicit compensation (close PR, delete tag, …). The orchestrator runs compensations in reverse order on failure when `on_failure: rollback`. Irreversible actions (a deploy to prod) are gated behind explicit human approval and are *not* auto-rolled-back — they escalate to a human with full context. **Foundry never silently undoes something it cannot cleanly reverse.**

---

## 8. Knowledge Architecture

> ⚠ **Assumption reframed:** The brief lists ~20 knowledge sources ("ADRs, RFCs, git history, CI failures, open PRs…") and says "think beyond files." Correct. But the deeper insight is that these sources are **not equal in kind.** Treating them uniformly is the mistake most "AI codebase" tools make.

### 8.1 The Derived / Authored split (the central idea)

```
                 KNOWLEDGE BASE
        ┌───────────────────┬───────────────────┐
        │     DERIVED        │     AUTHORED       │
        │  (a cache)         │  (source of truth) │
        ├───────────────────┼───────────────────┤
        │ code structure     │ ADRs               │
        │ import/symbol graph│ RFCs               │
        │ git history/blame  │ conventions        │
        │ CI failures        │ roadmap/milestones │
        │ open PRs/issues*   │ domain glossary     │
        │ test coverage      │ "state of impl"     │
        └───────────────────┴───────────────────┘
         recomputable, disposable    versioned in repo, durable
```

- **Derived knowledge** is *recomputable* from primary sources (the repo, git, the issue tracker, CI). It is a cache. It can be deleted and rebuilt. It never needs human review. It is gitignored.
- **Authored knowledge** is *created* by humans or by Foundry's own knowledge-update stage (then human-approved). It is the source of truth for *decisions and intent*. It lives in `.foundry/` in the repo, is code-reviewed, and survives independent of any model.

This split is what makes the "valuable even if LLMs disappear" promise real. The authored knowledge — *why* this architecture, *what* the conventions are, *what* was decided and superseded — is durable engineering capital. The derived layer is just an accelerator.

(* Open PRs/issues straddle the line: their *existence* is derived from the tracker, but a *decision recorded in* an issue may be promoted to authored knowledge. Promotion is explicit, not automatic.)

### 8.2 It's a graph, not a pile of files

Knowledge is modeled as a **typed graph** because the valuable queries are relational:

- Nodes: `Module`, `File`, `Symbol`, `Decision(ADR/RFC)`, `Convention`, `Milestone`, `Issue`, `PR`, `Person`, `TestSuite`, `Incident`.
- Edges: `depends_on`, `owns` (CODEOWNERS), `decided_by`, `supersedes`, `relates_to`, `caused_by`, `implements`, `tested_by`.

This lets the Context Engine answer "which ADRs affect the module this task touches, and which of them have been superseded?" — a graph traversal, not a keyword search. The graph is the substrate; embeddings are *one index over it*, not the whole thing.

### 8.3 Extractors (pluggable ingestion)

Each source has an **Extractor** adapter (behind `ExtractorPort`) that maps it into graph nodes/edges: `ASTExtractor`, `GitExtractor`, `MarkdownDocExtractor`, `IssueTrackerExtractor`, `CILogExtractor`. Extractors are plugins (§15) so the community can teach Foundry about new sources without touching the core. Extraction is **incremental** (driven by git diffs and source webhooks/polls) — never full re-scan when avoidable.

### 8.4 Knowledge evolution

The lifecycle's closing arrow lands here. After a workflow, a **Knowledge Update** stage proposes authored-knowledge changes: a new ADR for a decision made during the run, an update to "state of implementation," a new convention observed. These are *proposals* — they enter the repo as a reviewable diff and require human (or policy) approval before becoming authoritative. **Foundry never silently rewrites the project's source of truth.** Knowledge evolution is itself a gated workflow.

---

## 9. Context Engine Design

The brief calls this "one of the most important parts." Agreed — it is the make-or-break subsystem, and it is genuinely hard. It deserves the most careful design and the most humility (§19 lists it as the top product risk).

### 9.1 What the Context Engine is

A function: `(Task, KnowledgeBase, Codebase, Budget) → ContextBundle`. The ContextBundle is **immutable, content-addressed, budgeted, and fully attributed** (every chunk traces to a source + hash). It is not "a prompt"; it is a structured, auditable answer to "what knowledge does this task actually require?"

### 9.2 Pipeline

```
Task intent
   │
   ▼ 1. INTENT ANALYSIS        classify task type; extract entities
   │                           (modules, symbols, features) — deterministic
   │                           parsing first; model only if needed
   ▼ 2. CANDIDATE RETRIEVAL    multi-source, deterministic-first (see below)
   │
   ▼ 3. RANKING & SELECTION    score by relevance × recency × authority;
   │                           greedily fill the token budget
   ▼ 4. COMPACTION             signatures over bodies; hierarchical
   │                           summaries; "context cards"
   ▼ 5. ASSEMBLY               ordered, attributed bundle + provenance
   │
   ▼ ContextBundle (hashed, cached)
```

**Retrieval is deterministic-first, semantic-last** — a deliberate ordering, because deterministic signals are cheaper, explainable, and don't hallucinate:

1. **Structural** — symbol/import graph, ownership (CODEOWNERS), "files that change together" from git. Deterministic, high-precision.
2. **Lexical** — exact symbol/string search (ripgrep-class).
3. **Decision** — ADRs/RFCs linked (via the graph) to the touched modules; filter superseded.
4. **Historical** — git blame/log on target files; past PRs touching them; related CI failures.
5. **Semantic** — embedding similarity over code + docs. Used to *broaden* recall when deterministic signals are thin, never as the primary signal. Embeddings are a backstop, not the brain.

### 9.3 Budget, compaction, and provenance

- **Budget is a hard constraint, not a hope.** The bundle is assembled to *fit* a token budget that is itself a function of the chosen provider's context window and the run's cost budget. Selection is a greedy knapsack over (relevance, cost).
- **Compaction tiers**, applied as budget tightens: full file → relevant ranges → signatures/skeletons → summary card. Code we want the model *aware of* but not *editing* is compacted hardest.
- **Provenance is mandatory.** Each chunk carries `{source, hash, why_selected, score}`. This makes context auditable ("why did the model see this file?") and is essential for the feedback loop and for debugging bad outputs.

### 9.4 Caching & invalidation

- ContextBundles are content-addressed: `hash(task + selected_chunk_hashes + resolver_version)`. Identical inputs → cached bundle → replayable.
- Invalidation is **diff-driven**: a file's chunks invalidate when its content hash changes (cheap to detect via git). The derived knowledge index updates incrementally on the same signal. Authored-knowledge changes invalidate dependent bundles via graph edges.
- Embeddings recompute **incrementally** per changed file, never wholesale.

### 9.5 The relevance feedback loop (what makes it improve)

After implementation, Foundry knows which loaded files were *actually edited* vs. *merely loaded and ignored*. This signal feeds back to tune future selection (e.g., down-weight sources that are loaded-but-never-used for this task type). Over time the engine learns each project's relevance shape. This is recorded in the ledger and is itself observable (§18). **This loop is the difference between a retrieval tool and a context *engine*.**

### 9.6 ⚠ Honest limitation

There is no known technique that makes context selection reliably correct. The engine will sometimes load the wrong things or miss the right things. The architecture's defense is not "be perfect"; it is: (a) provenance so failures are *diagnosable*, (b) deterministic-first so behavior is *explainable*, (c) the feedback loop so it *improves*, and (d) validation downstream so a bad context that produces a bad diff is *caught by gates*, not merged. We design for graceful failure, not for an oracle.

---

## 10. Skill System

### 10.1 ⚠ The format question, answered: declarative manifest + code behavior

> **Brief asks:** "Should Skills be YAML? JSON? Go code? A DSL?"

This is a trap with a well-known wrong answer (invent a DSL) and a well-known right answer:

> **Decision:** A Skill is a **declarative manifest** (its *contract*) plus **optional code behavior** (its *implementation*, as a plugin behind the skill port). **Never a bespoke DSL.**

Reasoning:
- A **manifest** (YAML authored / JSON canonical) is right for the *contract*: declared inputs, required context, required provider capabilities, declared outputs, validators, quality gates, retry/repair policy. Contracts must be inspectable, diffable, signable, and statically analyzable. YAML/JSON delivers that; a DSL does not without enormous cost.
- **Code** (a real language, as a plugin) is right for the *behavior* that the manifest can't express declaratively: custom prompt assembly, multi-step internal logic, post-processing. Real languages have debuggers, tests, types, and ecosystems. A homegrown DSL has none of these and becomes an unmaintainable second-class programming language — the single most common way platforms like this die.

Many skills (especially early ones) are **manifest-only**: declare context + provider + prompt template + validators + gates, and the kernel's generic skill executor runs them. Code is the escape hatch, not the default.

```yaml
# illustrative skill manifest shape
skill: implement-feature
version: 2
inputs:    { task: string, target_module?: string }
context:   { resolver: default, max_tokens: 60000,
             require: [structural, decisions], prefer: [historical] }
provider:  { require_capabilities: [tool_calling, structured_output],
             quality_tier: high }
outputs:   [ diff ]
validators: [ build, lint, tests, arch-rules ]
gates:     { all: [build.pass, lint.pass, tests.pass, arch.no_violations] }
repair:    { max_iterations: 3, must_make_progress: true }
# behavior: ./behavior.go   # optional; omit for manifest-only skills
```

### 10.2 Skills are composable and versioned

Skills are versioned independently and distributed in **Workflow/Skill packs** (§15). A workflow pins skill versions (replayability). A skill declares its required port/SDK version for compatibility negotiation (§15.3).

### 10.3 The standard library of skills

Foundry ships a curated set (the brief's list: `plan-feature`, `implement-feature`, `review-pr`, `security-review`, `performance-review`, `generate-tests`, `generate-rfc`, `generate-adr`, `write-docs`, `prepare-release`, `create-migration`, `generate-changelog`). These are *reference implementations* and the proving ground for the skill API — if the standard skills are awkward to express, the API is wrong.

---

## 11. Provider System

### 11.1 ⚠ No lowest-common-denominator interface

> The brief says "providers must expose a common interface." True, but the naive reading — one interface that only exposes what *all* providers share — throws away exactly the capabilities that make each provider valuable (caching, parallel tool calls, structured output, huge context, reasoning modes).

> **Decision:** A **normalized request/response** *plus* **capability negotiation.** The interface is common; the *capabilities are advertised and negotiated*, not flattened.

### 11.2 Capability descriptor

Every provider/model advertises a `CapabilityDescriptor`:

```
{ context_window, max_output,
  supports: { streaming, tool_calling, parallel_tools,
              structured_output, vision, prompt_caching, reasoning },
  cost: { input_per_mtok, output_per_mtok, cache_read, cache_write },
  limits: { rpm, tpm }, latency_profile, locality: local|cloud }
```

Skills declare *required* and *preferred* capabilities. The Router selects a provider that satisfies the requirements, preferring those that satisfy preferences. Advanced features degrade gracefully when absent (e.g., emulate structured output via tool-calling, or via constrained re-prompting + validation).

### 11.3 The Router

Routing is policy-driven, configurable per project/workflow/skill:

- **Policies:** cost-optimized, latency-optimized, quality-tiered, **privacy-constrained** (e.g., `must_be_local` for sensitive repos → only Ollama-class providers).
- **Failover chains:** ordered fallbacks; on retryable failure, fail over to the next provider that satisfies required capabilities.
- **Health:** circuit breakers per provider; passive (observed errors) + active (lightweight probes) health.
- **Budget gate:** the router refuses a call that would exceed the run budget; it can downshift to a cheaper model if policy allows.
- **Determinism aid:** the exact `{provider, model, version, params}` and the request/response hashes are recorded in the ledger for replay.

### 11.4 Adapters

`provider/adapters/{anthropic,openai,gemini,ollama,…}` translate normalized ↔ native. Adapters are plugins; a new provider is a new adapter, zero kernel changes. Cost descriptors live with the adapter (the adapter knows its own pricing/capabilities), versioned so pricing changes are tracked.

---

## 12. Validator System

### 12.1 Deterministic-first is a hard rule, not a preference

> **Decision:** A Validator is LLM-free by default. An LLM is used in a validator **only** when deterministic checking is genuinely impossible, and even then with a structured rubric and (ideally) cross-checking. This is the technical embodiment of "Software Engineering > Prompt Engineering."

```
Validator tiers (prefer the top):
  1. DETERMINISTIC TOOLS   compilers, type-checkers (tsc), linters (eslint,
                           clippy), test runners, coverage, security scanners
                           (gosec, semgrep), AST-based architecture rules,
                           conventional-commit checkers
  2. DETERMINISTIC POLICY  structural assertions over the artifact/graph
                           ("no import from layer X to layer Y")
  3. LLM-JUDGE (last)      only for the irreducibly subjective: "does this RFC
                           cohere?", "does the PR description match the diff?"
                           — rubric-driven, logged, never a silent gatekeeper
```

### 12.2 Validator contract

`Validator: (Artifacts, Profile) → ValidationReport`. A report is a set of `Finding{ severity, location, rule_id, message, fixable }`. Validators are **pure with respect to the codebase** — they read and report, they never mutate. (Auto-fix is a *skill* that consumes findings, not a validator side effect — keeping validation idempotent and trustworthy.)

### 12.3 Gates compose validators

A **Gate** is a policy over reports + signals: `all/any/none` of named conditions, with `on_fail: repair | fail | warn`. Gates are where validation becomes decision. Because gates are deterministic functions of reports, **a gate verdict is reproducible** — the cornerstone of process determinism (§2.2). Validators and gates run **identically locally and in CI** (§14), so "passes on my machine" and "passes in CI" converge by construction.

---

## 13. Project Types → Profiles + Capabilities

> ⚠ **Assumption challenged:** "Plugins? Profiles? Capabilities?" — the brief offers these as alternatives. They are not alternatives; they are layers. Modeling project types as a rigid taxonomy (an enum of "Node | Go | Rust…") would be brittle: real projects are monorepos mixing ecosystems, or Next.js (which is React + Node + a bundler + its own conventions).

> **Decision:** Prefer **composition over taxonomy.**
> - A **Capability** is an atomic, detectable trait: "has TypeScript", "uses pnpm workspaces", "has a Dockerfile", "is a NestJS app". Each capability binds concrete toolchain commands, validators, and context extractors.
> - A **Profile** is a *composition* of capabilities resolved for a project (or a path within a monorepo). Profiles are detected by heuristics and overridable by explicit config in `.foundry/`.
> - Capabilities and profiles are **plugins** (§15) — the community adds ecosystem support without core changes.

A monorepo is one Project with **path-scoped profiles** (e.g., `/services/api` → Go profile, `/apps/web` → Next.js profile). The Context Engine and validators resolve the right profile per path. This composition model is the only one that survives contact with real, messy repositories.

---

## 14. Git & CI Integration

### 14.1 Git is native, via a VCS port

- **`VcsPort`** with adapters: `git` (local plumbing), and host adapters `github`, `gitlab`, `azure`, `bitbucket`.
- **Worktree isolation** (§7.3) is the foundation: branch creation, commits, and conflict handling happen in Foundry-managed worktrees.
- **Commits and PRs are generated from the ledger, not re-derived.** The run already knows what changed and *why* (the plan, the decisions, the validations). A semantic commit message and a PR body are *projections* of that history — high quality because they're grounded in the actual process, not a post-hoc summary of a diff.
- **Conflicts/rebase** are bounded operations; when resolution is ambiguous, Foundry escalates to a human with full context rather than guessing. Versioning, tagging, changelogs, and release branches are built-in ops or standard skills (`prepare-release`, `generate-changelog`).

### 14.2 CI is an execution environment, not a separate product

> ⚠ **Assumption answered:** "Should workflows run locally? In CI? Both? How is state synchronized?"

> **Decision:** **The same binary runs in both.** CI is just (a) a *trigger*, (b) an *execution environment*, and (c) a *VCS adapter* emitting native annotations (GitHub Checks, GitLab reports). There is no separate "CI engine." Workflows authored locally run unchanged in CI.

State synchronization:
- **Authored knowledge & definitions** travel *in the repo* (`.foundry/`) — already synchronized by git, no extra mechanism.
- **The derived index** is rebuilt in CI from sources (it's a cache), optionally warmed from a shared remote cache for speed.
- **Run ledgers** are local by default; teams opt into a **remote ledger backend** (the same event store behind a network port) for shared history and dashboards. Local-first remains the default; remote is additive.
- **Record/replay** makes CI runs cheap and stable: a verification job can replay a developer's run against recorded provider responses to deterministically re-check gates without paying for fresh model calls.

---

## 15. Extension System

The extension surface *is* the product for the open-source community. Everything replaceable is an extension point behind a versioned port.

### 15.1 The complete extension taxonomy

```
Extension points (all behind versioned ports):
  · Providers         (new model backends)
  · Validators        (new checks)
  · Skills            (new capabilities; manifest ± code)
  · Workflow packs    (curated workflow + skill bundles)
  · Knowledge extractors (new sources → graph)
  · Context resolvers (alternative selection strategies)
  · Project profiles / capabilities (new ecosystems)
  · VCS adapters      (new hosts)
  · CI adapters       (new pipelines)
  · Observability sinks (new telemetry targets)
```

### 15.2 ⚠ Plugin runtime: out-of-process, sandboxed, capability-gated

> ⚠ **Assumption challenged:** In-process plugins (dynamic linking, language-native plugin APIs) are the obvious choice and the wrong one. They lock the community to the kernel's language, crash the host on a plugin panic, and — critically — run untrusted community code with full host privileges. For a tool that holds repo access and provider credentials, that is unacceptable.

> **Decision: a tiered plugin trust model with out-of-process isolation.**
> - **Built-in** — in-tree, maintained by core. Full trust.
> - **Trusted** — signed, distributed, run as **out-of-process subprocesses over a stable RPC** (gRPC-class). Language-agnostic. A crash is contained.
> - **Community / untrusted** — run in a **sandbox (WASM component model preferred; subprocess with seccomp/namespacing as fallback)** under a **capability-based permission model**: a plugin gets *only* the capabilities it declares and the user grants (e.g., "network: anthropic.com only", "fs: read `.foundry/` only", "no secrets"). Default-deny.

This is more work than dlopen, but it is the difference between a platform people can safely `foundry plugin install some-random-repo` and one they can't. Sandboxing also future-proofs the security story (§17) for enterprise.

### 15.3 Versioning & compatibility

- **Ports are versioned with semver; the SDK is the contract.** Breaking a port is a major SDK bump and is rare and loud.
- Plugins declare the SDK/port version they target; the runtime negotiates compatibility at load and refuses incompatible plugins with a clear message rather than failing mysteriously at call time.
- **API stability is a public promise.** Pre-1.0, ports may break between minors (documented in a migration guide). Post-1.0, port stability is a core guarantee — this is what lets an ecosystem form.

### 15.4 Discovery & distribution

A **registry** of extensions. Start minimal: plugins are git repos with a manifest; discovery is an index (a simple, possibly community-hosted catalog) — *not* a bespoke package manager on day one. Distribution as OCI artifacts is a strong later option (reuses container infra, signing, provenance). Resist building a package manager before there are packages.

---

## 16. User Experience

The whole architecture earns its keep only if the developer experience is calm and trustworthy. The north star: **the developer is always reviewing and approving, never surprised.**

### 16.1 End-to-end walkthrough (illustrative CLI)

```bash
# 1. Install — single binary, no runtime deps
$ brew install foundry            # or: curl … | sh

# 2. Connect providers — credentials go to the OS keychain, never to disk
$ foundry provider add anthropic     # prompts for key → keychain
$ foundry provider add ollama --endpoint http://localhost:11434
$ foundry provider list
  anthropic  ✓ healthy   caps: tools,structured,caching,reasoning
  ollama     ✓ healthy   caps: tools                local

# 3. Initialize a project — detects profile(s), scaffolds .foundry/
$ cd ~/code/my-app && foundry init
  detected: Next.js (apps/web), Go (services/api)  → 2 profiles
  created .foundry/ (workflows, skills, knowledge)

# 4. Build initial knowledge — extractors populate the graph (incremental after)
$ foundry knowledge index
  structural ✓  git-history ✓  docs ✓  → 1,284 nodes, 3,901 edges
  (no ADRs found — consider `foundry adr new` to capture decisions)

# 5. Run the first workflow — work happens in an isolated worktree
$ foundry run implement-feature --task "add rate limiting to the public API"
  plan        ✓  (3 steps; touches services/api/middleware)
  context     ✓  bundle 0x9af… 42k tok  [12 chunks: 7 structural, 3 decisions, 2 historical]
  implement   ✓  3 files changed (+187 −12)
  gate: build ✓  lint ✓  tests ✓ (2 new)  arch ✓
  review      ⚠  1 suggestion (non-blocking): add metric for throttled reqs
  → run paused at human gate. review: `foundry review last`

# 6. Review the diff (never auto-applied to your working tree)
$ foundry review last
  [renders diff + the WHY: plan, decisions used, validation reports, cost]
  cost: $0.31  ·  2 provider calls  ·  bundle + reports recorded in ledger

# 7. Approve → commit + PR generated from the ledger (grounded, not guessed)
$ foundry approve last
  commit  ✓  feat(api): add token-bucket rate limiting  (conventional ✓)
  pr      ✓  https://github.com/me/my-app/pull/482

# 8. Update project knowledge — proposed as a reviewable diff, you approve
$ foundry knowledge update last
  proposed: ADR-0007 "Rate limiting strategy: token bucket per API key"
  proposed: state-of-impl: services/api now has rate limiting
  apply? [y/N]
```

### 16.2 UX principles

- **Diff-first, approval-gated.** Foundry proposes; the human disposes. Outward actions (PR, deploy) always behind explicit approval unless a policy grants standing authorization.
- **The "why" is always one command away.** Any output traces to its context, its validations, and its cost — because the ledger makes that cheap.
- **Cost is visible before and after.** Budgets are shown; overruns are refused, not silently incurred.
- **Resumable, interruptible.** `Ctrl-C` never corrupts state; `foundry resume last` continues.
- **Editor/daemon parity.** The eventual IDE integration is a thin client over the same daemon; no second engine, no behavioral drift.

---

## 17. Security

Foundry holds repo access and provider credentials and runs community code. Security is foundational, not a feature.

- **Secrets** — provider credentials in the OS keychain or an external secret manager (Vault, cloud KMS) via the `SecretsPort`. **Never** in `.foundry/` or any committed file. Secrets are redacted from logs, telemetry, and the ledger by a mandatory redaction pass.
- **Plugin sandboxing & capability permissions** (§15.2) — community code is default-deny, runs sandboxed, gets only granted capabilities. Plugins are signed; signature/trust policy is per-repo and per-org.
- **Air-gapped / offline mode** — a first-class mode: forces local providers (Ollama-class), disables all telemetry and network egress, refuses cloud adapters. Required for sensitive repos and enterprise.
- **Privacy-constrained routing** — repos can be marked sensitive; the router then refuses non-local providers (`must_be_local`). Per-path policies for monorepos.
- **PII / secret scanning** — a deterministic validator scans artifacts (and context bundles before they leave the machine) for secrets/PII; a gate can block egress.
- **Audit log = the run ledger** (§7.4) — every action, approval, provider call, and knowledge change is recorded immutably. Exportable for compliance.
- **Permission model** — actions are classified by blast radius (read / local-mutate / outward / irreversible). Each class has a configurable authorization policy (auto / approve-once / approve-always-prompt). Irreversible actions (prod deploy) always require explicit human approval and are never auto-rolled-back (§7.6).
- **Enterprise installation** — the hexagonal kernel + remote ledger/secret ports make on-prem, SSO-fronted, audited deployments a configuration, not a fork.

---

## 18. Observability

Observability is not bolted on; it is *projected from the ledger* (§7.4), so it is complete and consistent by construction.

- **Tracing** — OpenTelemetry spans nest naturally: `WorkflowRun → StageRun → SkillInvocation → ProviderCall / ValidatorRun`. The trace tree mirrors the domain.
- **Metrics** — cost & tokens per run/stage/skill/provider; validator pass rates and finding counts; retry/repair counts; gate verdicts; provider latency and error rates; context bundle sizes and cache hit rates.
- **Cost is enforceable, not just observed** — budgets (§ kernel/budget) gate execution. "$X per workflow" is a control, not a report.
- **Execution history** — every run is replayable and inspectable; `foundry runs` / `foundry inspect <run>`.
- **Provider performance & routing analytics** — informs router policy tuning.
- **Knowledge evolution metrics** — graph growth, ADR coverage, stale-knowledge detection, relevance-feedback effectiveness (§9.5).
- **Sinks are pluggable** (`ObservabilityPort`) — stdout/TUI locally; OTLP/Prometheus/Datadog/etc. for teams. Telemetry is **opt-in, redacted, and fully disabled in air-gapped mode.**
- **Local dashboard** — the daemon serves a local TUI/web view of runs, costs, and knowledge health. No cloud required to see your own data.

---

## 19. Risks

Honest enumeration, highest first. (Mitigations in-line; trade-offs in §20.)

1. **Context Engine quality is the make-or-break and is genuinely unsolved.** Bad context → bad outputs → lost trust. *Mitigation:* deterministic-first retrieval, mandatory provenance (diagnosability), the feedback loop (improvement), downstream validation (containment). We design for graceful, debuggable failure, not for an oracle (§9.6).
2. **Boiling the ocean.** The full lifecycle (idea→deploy) is enormous; building it all before shipping value would kill the project. *Mitigation:* aggressive phasing (§22); a walking skeleton that does *one* loop end-to-end before breadth.
3. **The determinism promise is partial and easy to over-sell.** *Mitigation:* the §2.2 reframe — promise *process* determinism, communicate it relentlessly, never imply identical model output.
4. **Over-engineering / architecture astronautics.** This very document describes a lot of machinery; a 12-port hexagon with a plugin sandbox could collapse under its own weight before it has users. *Mitigation:* the kernel stays thin; ports are added when a second adapter actually exists, not speculatively; ship vertical slices.
5. **Provider abstraction lags provider innovation.** New model features (new tool modes, new caching) require adapter + capability work; the abstraction can feel a step behind. *Mitigation:* capability negotiation (not LCD), graceful degradation, fast adapter iteration as a community surface.
6. **LLM-judge validators are unreliable.** *Mitigation:* keep them minimal, rubric-driven, logged, and never the sole gate where a deterministic check is possible.
7. **Community plugin security.** Untrusted code near credentials and repos. *Mitigation:* the sandbox + capability model (§15.2); signing; default-deny.
8. **Repair-loop cost runaway.** *Mitigation:* hard iteration + cost bounds and a must-make-progress invariant (§7.5).
9. **Competition & differentiation.** Claude Code, Aider, Cursor, Devin, et al. move fast. If Foundry is "another wrapper," it loses. *Mitigation:* the moat is *not* code-gen — it's determinism, reproducibility, durable knowledge, and auditable process. Lean into what wrappers structurally can't do.
10. **Adoption friction.** A platform that demands process discipline may feel heavier than a chat box for quick tasks. *Mitigation:* make the simple path simple (manifest-only skills, `foundry run <skill>` one-liners); let value compound for teams without taxing individuals.

---

## 20. Trade-offs

Every major decision and its conscious cost. These are the seeds of the first ADRs.

| Decision | Chosen | Rejected alternative | Cost we accept |
|---|---|---|---|
| Determinism scope | Process determinism + replay | Output determinism | Can't promise identical model text; must educate users |
| Core shape | Thin hexagonal kernel + ports | Linear "engine pipeline" | More upfront abstraction; pays off in testability/extensibility |
| Deployment | Local-first single binary | SaaS/services from day 1 | No shared state until remote ledger added later |
| Language | Go (pending ADR) | Rust / TS / Python | Less type expressiveness than Rust; mitigated by language-agnostic plugins |
| Workflows | Declarative data | Imperative code/DSL | Less raw flexibility; bounded by design (logic goes in skills) |
| Skills | Manifest + optional code | Bespoke DSL | Two formats to learn; avoids an unmaintainable mini-language |
| Providers | Capability negotiation | Lowest-common-denominator interface | More complex router; preserves provider power |
| Validation | Deterministic-first | LLM-judge everywhere | Less "flexible"; vastly more trustworthy/cheaper |
| Project types | Composed capabilities/profiles | Rigid type enum | More moving parts; survives monorepos & hybrids |
| Plugins | Out-of-process, sandboxed | In-process dynamic linking | Performance overhead; gains safety + language freedom |
| Knowledge | Derived/Authored split + graph | Uniform "everything is a file" | More modeling; enables the durability promise |
| Mutation | Isolated worktree, diff-review | Edit working tree in place | Extra git mechanics; makes rollback & trust trivial |
| State backbone | Single event-sourced ledger | Separate audit/replay/resume subsystems | Event-sourcing learning curve; one substrate, five free features |

---

## 21. Future Roadmap (beyond v1.0)

- **Full lifecycle breadth** — RFC/architecture/security/performance/release stages matured into first-class workflow packs.
- **Team intelligence** — shared remote ledger enables cross-developer learning: relevance feedback and convention learning pooled across a team/org.
- **Multi-repo & platform-of-platforms** — orchestrate changes spanning services; dependency-aware cross-repo workflows.
- **Marketplace** — curated, rated, signed extension registry; org-private registries.
- **Adaptive routing** — router learns cost/quality trade-offs per task type from historical outcomes.
- **Knowledge-graph reasoning** — richer queries ("what decisions would this change violate?") as a pre-implementation gate.
- **Enterprise** — SSO, RBAC, on-prem, compliance exports, air-gapped reference deployment.
- **IDE clients** — thin clients over the daemon for VS Code/JetBrains, no engine fork.

---

## 22. Suggested Implementation Phases (v0.1 → v1.0)

Guiding principle: **depth before breadth.** Ship one complete loop, then widen. Each phase is independently useful and proves an architectural seam before the next leans on it.

### v0.1 — Walking skeleton (prove the spine)
- Hexagonal kernel + run ledger + **record/replay** (this *must* be in v0.1 — it's the determinism story and the test harness).
- **One** provider adapter, **one** project profile, **one** hardcoded linear workflow (`implement-feature`).
- Isolated-worktree mutation + diff review + approve.
- Deterministic validators: build, lint, tests + gates.
- **Exit criterion:** a real feature implemented, validated, committed, and *replayed identically from the ledger*.

### v0.2 — Declarative core
- Declarative workflow definitions + skill manifests (manifest-only skills).
- Multiple provider adapters + the Router (capability negotiation, failover, budget gate).
- Retries + first-class repair loops (bounded).

### v0.3 — Knowledge & Context (the differentiator)
- Knowledge graph + Derived/Authored split + core extractors (AST, git, docs).
- ADR/decision model + `knowledge update` as a gated, reviewable workflow.
- Context Engine v1: deterministic-first retrieval, budget, compaction, **provenance**, content-addressed caching + diff-driven invalidation.

### v0.4 — Integration & visibility
- VCS host adapters (GitHub first) + grounded commit/PR generation.
- CI execution mode (same binary) + native annotations + replay-based verification.
- Observability: OTel traces, cost/token metrics, enforceable budgets, local dashboard.

### v0.5 — Extensibility (open the gates)
- Extension SDK + versioned ports + out-of-process/sandboxed plugin runtime + capability permissions.
- Plugin discovery/registry (minimal).
- Community-authored providers/validators/profiles begin.

### v0.6–v0.9 — Lifecycle breadth & intelligence
- More skills/workflow packs: RFC, ADR, security/performance review, release, changelog, migrations.
- Relevance feedback loop (§9.5) live; knowledge-evolution metrics.
- Monorepo path-scoped profiles; semantic retrieval tier matured.
- Hardening: air-gapped mode, secrets ports, audit export.

### v1.0 — Stability promise
- **Port contracts frozen under semver** (the ecosystem guarantee).
- Security hardening + enterprise readiness (on-prem reference, RBAC/SSO).
- Complete contributor & extension docs; documented release process; conformance test suites for each port (so third-party adapters can self-certify).

---

## Appendix A — Open decisions requiring ADRs

1. **ADR-0001** — Kernel/CLI language (recommend Go). _(§5.4)_
2. **ADR-0002** — Plugin isolation mechanism: WASM component model vs. gRPC subprocess as the *primary* path. _(§15.2)_
3. **ADR-0003** — Embedding/vector store choice and on-device embedding strategy for air-gapped mode. _(§9)_
4. **ADR-0004** — Ledger storage engine (embedded log vs. SQLite) and the remote-ledger protocol. _(§7.4, §14.2)_
5. **ADR-0005** — Workflow/skill manifest schema and versioning policy. _(§7.1, §10)_
6. **ADR-0006** — Registry/distribution format (git+manifest vs. OCI artifacts). _(§15.4)_
7. **ADR-0007** — Capability descriptor schema and the negotiation/degradation rules. _(§11.2)_

## Appendix B — Glossary

See §4. The terms there are the project's *ubiquitous language*; code, CLI, and docs must use them consistently. Changing a term is a documentation event, not a casual rename.

---

_End of document. Challenge any section by opening an ADR or an issue that references the section number. This blueprint is meant to be argued with — that is how it stays correct._
