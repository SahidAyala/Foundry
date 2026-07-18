# RFC-0002 — From a Fixed Act Lifecycle to a Pipeline Execution Runtime

| | |
|---|---|
| **Status** | Draft — Proposed (seeking ratification; a governance process now exists — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) — but this RFC has not itself been individually ratified through it) |
| **Authors** | Principal architect review (AI-assisted), for Foundry Core |
| **Reviewers** | _(pending)_ |
| **Supersedes** | — |
| **Superseded by** | — |
| **Created** | 2026-07-07 |
| **Related** | [domain.md](../02-architecture/domain.md), [execution.md](../02-architecture/execution.md), [extensibility.md](../02-architecture/extensibility.md), [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md), [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md), [M0-IMPLEMENTATION-BACKLOG.md](../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md) |

> **What this document is.** A full architectural audit of the M0 implementation as it exists today, and a proposed migration from its single hardcoded Act lifecycle to a data-driven Pipeline execution runtime — while preserving the Act as the domain's invariant unit, the Engine's ownership of control flow (I1), and every other accepted invariant. This RFC does not redefine the domain ([domain.md](../02-architecture/domain.md)); it proposes how [execution.md](../02-architecture/execution.md)'s "Pipeline is one Strategy" idea gets a real, general implementation instead of remaining a single fixed procedure wearing Pipeline vocabulary.
>
> **Maturity discipline.** This RFC is PROVISIONAL and non-canonical until ratified through [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)'s process — that process now exists, but this RFC has not yet been individually walked through it. It does not silently upgrade OQ-002's open status — see §4's reconciliation note. No code accompanies this RFC; per the operating instructions for this audit, the deliverable is the plan, not the implementation.

---

## 0. Executive summary

Foundry's M0 implementation (`domain/`, `engine/`, `executor/`, `gatherer/`, `verify/`, `workspace/`, `record/`, `cli/`, `cmd/foundry/` — roughly 2,000 lines of Go) is a real, working, well-tested walking skeleton exactly as scoped by [m0-plan.md](../04-guides/m0-plan.md): one Intent, one Executor call, one verification pass, one bounded repair, one approval, one record. It is not a framework — deliberately. That was the right call for M0.

It is, however, the fixed lifecycle the user's request describes: the Engine knows exactly two steps and one repair round; approval is a single hardcoded call outside the Engine; there is one Executor per process; the `Act` struct has room for exactly the fields M0.0–M0.3 need and no more. None of this is a defect — it is M0 doing what M0 promised. But it is the wrong shape to extend toward "configurable AI pipelines composed of multiple models, verification steps, repair loops and human approval," and extending it by accretion (a flag here, a second hardcoded repair round there) would recreate, piece by piece, exactly the rigidity the project's own architecture docs already warn against.

The proposed target: the **Engine becomes a generic Strategy-execution runtime**, of which a genuinely data-driven **Pipeline** — an ordered sequence of typed **Steps** with named repair-jump edges, authored as a declarative document — is the first and best-supported Strategy, exactly as [execution.md](../02-architecture/execution.md) already claims but does not yet implement. Approval, repair, and recording all become Step-level concerns instead of hardcoded call sites. A **Router** matches each Step's declared **Capabilities** to a configured **Executor**, which is what makes multi-model pipelines (Planner→GPT, Implementer→Claude, Reviewer→Gemini) possible without new Engine code. Migration proceeds in eight independently shippable phases, each behavior-preserving until the phase whose entire point is to change behavior, so `foundry do "<intent>" --repo <path>` with no flags never breaks.

---

## 1. Current architecture

### 1.1 The lifecycle as actually implemented

The eight-step lifecycle in [execution.md](../02-architecture/execution.md) (open → gather → produce → verify → judge → repair → apply → record) is *conceptually* honored but *mechanically* compressed into two Go functions:

```
cli.CLI.Do(ctx, intent, repoPath)
  └─▶ engine.Engine.Run(ctx, intent)              // = RunBudgeted(ctx, intent, DefaultBudget())
        1. act := domain.NewAct(intent.Text)
        2. considered, _ := gatherer.Gather(ctx, intent)          // ONE gather, up front
        3. spent.charge(executeCostEstimateUSD)                    // budget check #1
        4. outcome, _ := executor.Execute(ctx, intent, considered) // ONE Execute
        5. judgment, _ := verifier.Verify(ctx, outcome, workspace) // ONE Verify
        6. if judgment.Verdict == "fail":
             repairOnce(...)                                       // AT MOST one repair round:
               spent.charge(...)                                   //   budget check #2
               executor.Execute(ctx, intent, considered+findings)  //   Execute again
               verifier.Verify(ctx, outcome2, workspace)           //   Verify again
        7. return act with final JudgmentVerdict + CheckedFindings
  └─▶ PromptForApproval(stdin, stdout, act)         // ONE approval, always after Run returns
  └─▶ if approved:
        workspace.NewWorkspace + Apply + Land        // apply the patch to the dev's branch
        recorder.Write(ctx, act)                     // ONE record, only on approval
```

A declined Act is **not** recorded (deferred explicitly in code comments) — the immutable-history promise (I8) currently has a gap on the "no" path. This is a known, named gap in the code, not a silent one, but it is a real limitation worth carrying into any redesign.

### 1.2 Package responsibilities

| Package | Responsibility | Depends on |
|---|---|---|
| `domain` | `Act`, `Intent`, `Outcome`, `Budget`, `Judgment` — flat value types, pure, stdlib only | nothing |
| `engine` | `Engine.RunBudgeted` (the one Strategy, uncodified as such), budget tracker, the one repair round, `Reporter` port, and the `Executor`/`Verifier`/`Gatherer` port interfaces it declares | `domain` |
| `executor` / `executor/claude` | `ScriptedExecutor` (deterministic fixture), `ClaudeExecutor` (subprocess to Claude Code CLI, parses a unified diff out of stdout) | `domain`, `engine` (see §1.3) |
| `gatherer` | `NaiveGatherer`: regex file-name extraction from Intent text, an identifier-name fallback, README/directory supplementary context, all bounded to 100 KB | `domain`, `engine` |
| `verify` | `Validator` (shell command + timeout), `Gate` (single `"all-pass"` rule over Validators) | `domain`, `engine` |
| `workspace` | `Workspace` (git-branch isolation: `Apply`/`Land`/`Clean`), `StagedVerifier` (decorator: stages HEAD into a worktree, applies the Outcome's patch, delegates to a wrapped `Verifier`) | `domain`, `engine` |
| `record` | `FileStore`: one JSON file per Act at `<root>/<id>/act.json`, write-once (`O_EXCL`), `List` in creation order | `domain` |
| `cli` | `CLI.Do/Log/Show`, `ParseArgs`, `PromptForApproval`, ANSI diff/verdict rendering, `ProgressReporter` | `domain`, `engine`, `record`, `workspace` |
| `cmd/foundry` | Process entry point, dependency wiring (composition root), subcommand dispatch (`do`/`log`/`show`) | everything |

### 1.3 Coupling

The dependency direction is mostly what [m0-plan.md §6](../04-guides/m0-plan.md#6-dependency-graph--rules) prescribes — `domain` imports nothing, `engine` imports only `domain`, adapters import `domain`. One deviation from the plan's stated rule ("adapters never import `engine`"): `executor/claude`, `gatherer`, `verify/gate.go`, and `workspace/staged.go` all import `engine` solely to write a compile-time conformance assertion (`var _ engine.Executor = (*ClaudeExecutor)(nil)`). Because Go interfaces are structurally typed, this import is not required for the adapter to satisfy the port — it is a convenience check, not a real coupling need — but it is a real import edge today, and it is exactly the kind of edge that will need to disappear before a third-party, out-of-process Executor plugin can exist without importing Foundry's core module (relevant to [OQ-005](../06-open-questions/OQ-005-extension-isolation.md)).

Beyond that one soft edge, the packages are cleanly separable. `engine` genuinely does not know how a patch is produced, how the workspace is staged, or where an Act is stored.

### 1.4 Trust boundaries

- **Untrusted until verified (I4):** honored. `Gate.Verify` always runs before `PromptForApproval`; `StagedVerifier` guarantees the Gate checks the *proposed* patch (applied in an isolated worktree), never the developer's actual checkout.
- **Accountability (I5):** honored for the accept path. `Authority` is the OS user (`$USER` or `whoami`), captured only on explicit `y`/`yes` at a blocking terminal prompt. There is exactly one such prompt per Act, always at the very end, always synchronous, always human — the "explicitly delegated policy" half of the `Authority` definition in [terminology.md](../05-reference/terminology.md) has no implementation.
- **Deterministic-first verification (I6):** honored. `Validator` runs a shell command (`go build`, `go test`, or a `repo-sanity` fallback); there is no model-based check anywhere in the verification path.
- **Record durability (I8):** partially honored. `FileStore` enforces write-once immutability for what it does write, but only approved Acts are written — see §1.1's gap.
- **Model as substrate (I12):** honored in spirit and validated by the codebase: `Executor` already has two independent implementations (`Scripted`, `Claude`) behind one interface, proving the abstraction earns its place rather than being speculative.

### 1.5 Invariants as implemented vs. as promised

`I13 — No single execution Strategy is privileged` is the one invariant the code does not yet embody, because there is no `Strategy` type at all: `Engine.RunBudgeted` *is* the only strategy, compiled directly into the `Engine` struct. This is the precise gap this RFC addresses.

---

## 2. Current strengths — what should absolutely remain

1. **The package boundaries and inward dependency direction.** `domain` → `engine` (ports declared by consumer) → adapters is the right shape and should be preserved through the migration, not re-architected.
2. **The `Executor` port, proven with two real implementations from day one.** This is the one abstraction M0 built early and it paid off exactly as [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) and the archived freeze review's P1-2 finding (an `Executor` abstraction with no domain-model home) both anticipated it should. Keep the port; generalize its call sites, not its shape.
3. **`StagedVerifier` as a `Verifier`-decorating `Verifier`.** Wrapping a port with another implementation of the same port to add cross-cutting behavior (here: worktree staging) is exactly the composition idiom a Step-level interceptor/middleware model needs later (tracing, budget-checking, retry). This pattern should be named and reused deliberately, not treated as a one-off.
4. **The `Reporter` port.** A pure observer, structurally incapable of influencing control flow (I1 by construction, not by convention), is the correct template for how a Pipeline's step-level events get surfaced to a UI later.
5. **The Budget tracker's shape** (`charge` before spending, refuses without partially consuming, wraps a sentinel error) — small, enforced-not-reported, easy to generalize to per-Step accounting.
6. **`workspace.Workspace`'s isolation guarantee** (never mutate the developer's checkout; a throwaway branch; explicit `Land` vs. `Clean`) — this is the concrete, working instance of the "never mutate the user's working tree" rule the archived architecture called non-negotiable, and it already works correctly today.
7. **`record.FileStore`'s filesystem-first, human-readable, write-once JSON.** Matches [principles.md](../00-overview/principles.md)'s filesystem-first persistence rule and needs no storage-engine change to support a richer Act shape (§4.5) — only a schema change.
8. **The Validator/Gate error-vs-finding distinction.** `Validator.Run` returns a Go `error` only when the command could not run at all; a failing build or test is data (`Result{Passed:false}`), never an error. This is exactly the distinction §5 needs to generalize to arbitrary Steps and is already implemented correctly — it should be stated as an explicit rule going forward, not just an implicit convention.
9. **The CLI's "trivial path stays trivial" default.** One intent string, one `--repo` flag, a blocking `y/n`. Whatever richness gets added (§8), this must remain reachable with zero new ceremony for a user who wants exactly what M0 offers today.
10. **Golden/integration-test discipline.** The existing tests already assert on recorded Act shape and CLI output; the migration's safety net depends on this discipline continuing and, per §9 and §10, being audited for completeness before schema changes begin.

---

## 3. Architectural limitations — every place a fixed workflow is assumed

1. **`Engine.RunBudgeted` is a hardcoded Go function, not a data structure.** The lifecycle is imperative control flow in `engine.go`/`repair.go`. There is no `Step` type, no ordered list, no way to add, remove, or reorder a phase without a code change and a new binary.
2. **Exactly one repair attempt is wired in by name (`repairOnce`), with the "1" baked into `defaultMaxIterations = 2`.** There is no way to express two verify→repair rounds, or a different repair strategy (re-plan vs. patch-fix), without rewriting `repair.go`.
3. **Approval is a single hardcoded call (`PromptForApproval`) outside the Engine, always after `Run` returns, always synchronous, always human.** `Plan → Approval → Implementation → … → Approval → Apply` — the user's own first example — cannot be expressed: there is exactly one approval point and it can only ever be last.
4. **`Executor.Execute` has one fixed signature assuming "one Intent, one Outcome (a patch)."** A Planner Executor producing a plan Artifact for a different Implementer Executor to consume has nowhere to plug in; `domain.Outcome` is hardcoded to `struct{ Patch string }`.
5. **`Gatherer.Gather` runs exactly once, before execution, returning an untyped, unattributed `[]string`.** There is no per-Step Context assembly, and no attribution to source — the "considered" half of Evidence is far thinner than [terminology.md](../05-reference/terminology.md)'s Context definition promises.
6. **`Gate` supports exactly one rule (`"all-pass"`) over shell-command `Validator`s, evaluated at most twice per Act (initial + one repair).** There is no notion of multiple Gates at different pipeline points (a lint gate after Implementation, a security gate after Review) or of a non-boolean verdict rule.
7. **`Act` is a flat struct with singular "latest value" fields** (`ConsideredFiles`, `CheckedFindings`, `Patch`, `JudgmentVerdict`), not a Step-indexed trace. There is no way to answer "what happened at step 3 of 7" from a recorded Act.
8. **No `Strategy` type exists at all.** [execution.md](../02-architecture/execution.md) promises Pipeline is one Strategy among several — the code has zero Strategies as a named concept; `RunBudgeted` *is* the only one, compiled into `Engine`.
9. **`Recorder` is not an Engine port.** Recording happens in `cli.CLI.Do`, entirely outside the Engine, only after approval. A crash between Execute and Verify loses all progress silently — the opposite of "interrupt-driven and non-linear... resumable, replayable states" (RFC-0001 §8.1).
10. **Budget accounting is tied to "one Executor.Execute call = one charge,"** hardcoded at exactly two call sites. A pipeline with heterogeneous Step costs (cheap deterministic Steps, expensive model Steps, human-wait Steps) has no general per-Step accounting.
11. **`StagedVerifier` assumes exactly one cumulative patch, applied once, verified once.** A pipeline with several sequential non-patch Outcomes (a Plan, then an Implementation) has no generalized "stage the state so far and check it."
12. **The CLI is one flat command (`do`) for the one fixed shape.** There is no `--pipeline <name>`, no way to select a different Step sequence, and no interactive/ongoing-session concept — every invocation is a fresh, one-shot `Engine.Run`.
13. **No `Router`, no `Capability` model, no more than one Executor per Act exists anywhere in the tree** (confirmed by search). Multi-model pipelines (Planner→GPT, Implementer→Claude, Reviewer→Gemini) cannot be expressed until this exists — every Execute call in an Act, including its repair round, currently goes to the single Executor chosen once at the CLI composition root.
14. **Adapters importing `engine` purely for a compile-time assertion (§1.3)** is a soft coupling that will need to be broken before a language-agnostic, out-of-process extension boundary (OQ-005) can exist without forcing a plugin author to import Foundry's own module.
15. **No "reusable Act template" exists.** Every Act's shape is the Go source of `Engine`; there is no way to author, name, version, or share a different sequence without a code change — this directly blocks the M2 milestone ("author reusable Act templates") and is the central target of this RFC.

---

## 4. Target architecture

### 4.1 Reconciling the ask with OQ-002

The user's framing — "the Engine should become a generic pipeline runtime" — must be read carefully against [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md)'s still-open status: Pipeline is *one* Strategy, not the system's spine, and this RFC does not silently resolve that question. The correct target is therefore: **the Engine becomes a generic *Strategy*-execution runtime, whose first, best-supported Strategy is a genuinely data-driven Pipeline** — exactly OQ-002's current recommendation ("keep Pipeline as one Strategy... but treat the graph as the first, best-supported strategy"), just finally *implemented* as such instead of being the only strategy that happens to also be hardcoded. Adaptive, deterministic-procedure, and human-driven Strategies remain conceptual until a real use case demands them, unchanged from today's guidance (depth before breadth).

### 4.2 Shape

```
Engine.Run(ctx, intent, strategy Strategy) (*Act, error)

type Strategy interface {
    Produce(ctx, act *Act, ports Ports) error   // mutates act's Step trace as it runs
}

PipelineStrategy implements Strategy by interpreting a Pipeline definition:
    a restricted DAG — an ordered list of Steps plus named backward "repair" edges,
    NOT a general graph (see §5's tradeoff discussion).

Step kinds (closed set, extensible only by adding a new kind deliberately, not by
    the Pipeline document inventing arbitrary behavior — see §6):
    Generate  — a Router-selected Executor produces Artifact(s) from Context + Intent
    Verify    — Validators + a Gate produce a Judgment fragment
    Approve   — an Authority (human, or an explicitly delegated policy) accepts/rejects
    Apply     — an accepted Outcome is applied to Project State (code via Workspace, or Knowledge)
    Record    — checkpoint the Act's Step trace so far to the Record (safe to run after every Step)
```

### 4.3 Approval and repair become Step placement, not Engine code

Both of the user's example lifecycles fall out of this for free, with zero Engine changes:

```
Plan → Approval → Implementation → Verification → Repair → Approval → Apply → Record
  = [Generate(plan), Approve, Generate(implement), Verify, <repair edge back to Generate(implement)>,
     Approve, Apply, Record]

Implementation → Verification → Repair → Verification → Apply
  = [Generate(implement), Verify, <repair edge back to Generate(implement)>, Verify, Apply]
```

A `repair` verdict from a `Verify` Step is a jump to a named earlier Step with the failing findings injected as additional Context, bounded by Budget and a must-make-progress rule (findings count strictly decreasing, else abort) — this is exactly today's `repairOnce`, generalized from "always jump to the one Execute call" to "jump to whichever Step the Pipeline names," and it is worth explicitly reviving the retired-but-sound distinction from the archived pre-M0 design: **retry** (same Step, same inputs, transient infrastructure failure, invisible in the Judgment) is never the same construct as **repair** (a different/re-fed Step, triggered by a Gate verdict, always visible in the Act's Step trace).

### 4.4 Multi-model routing

A `Generate` Step declares required **Capabilities** (e.g. `role: plan`, `role: implement, tool_use: true`, `role: review`). A **Router** matches declared Capabilities to configured Executors under a policy (explicit pin first; cost/latency/quality/privacy negotiation only once ≥2 real Executors need it — see §7). This is what makes "Planner→GPT, Implementation→Claude, Review→Gemini" expressible without new Engine code: it is three `Generate` Steps with three different Capability declarations, resolved by the Router to three different configured Executors.

### 4.5 Act becomes a Step trace

```go
type Act struct {
    ID        string
    Intent    string
    CreatedAt time.Time
    Steps     []StepRecord   // NEW: ordered trace, one entry per Step attempt (including repairs)
    // existing flat fields (Patch, JudgmentVerdict, ApprovedBy, ...) become
    // derived views over the final relevant StepRecord, kept for compatibility
}

type StepRecord struct {
    StepID          string
    Kind            string    // "generate" | "verify" | "approve" | "apply" | "record"
    Considered      []string  // this step's attributed Context
    Produced        []string  // Artifacts this step yielded (content-addressed)
    Checked         []string  // findings, if this was a Verify step
    JudgmentVerdict string    // if applicable
    Authority       string    // if this was an Approve step
    StartedAt, FinishedAt time.Time
}
```

This is additive (§9 Phase 1), not a breaking rewrite: today's flat fields keep working, `foundry show`/`log` keep rendering, and the Record's on-disk JSON gains a field rather than losing one — deliberately, because the Record's on-disk shape is a compatibility surface with no owning ADR yet (§10).

---

## 5. Step abstraction

**Inputs:** the Intent (or the Act's accumulated state so far), plus a Step-scoped Context assembled by a Gatherer/Context-Source bound to that specific Step — not one Gather-everything-up-front call. Bounded by a Step-scoped slice of the Act's overall Budget.

**Outputs:** zero or more content-addressed **Artifacts** — a patch, a plan document, a set of review findings, a Knowledge proposal — never hardcoded to "a patch," generalizing `domain.Outcome`.

**State:** `Pending → Running → {Succeeded, Failed, AwaitingApproval, Repairing, Skipped} → Terminal`. This is the archived pre-M0 design's `WorkflowRun` state machine, scoped down to per-Step granularity and composed upward into an Act-level status derived from its Steps — reviving a design that was already carefully thought through and never contradicted by anything ratified since.

**Transitions:** Engine-driven per Step kind — `Generate` auto-advances to the next declared Step; `Verify`'s Gate verdict decides pass (advance) / fail (advance to a declared terminal-fail or a declared repair target) / repair (jump back, bounded); `Approve` blocks until an Authority decides, then advances or terminates as rejected; `Apply` is terminal-success for that Outcome's scope; `Record` is an idempotent checkpoint, safe after every Step.

**Errors vs. findings — a rule, not a convention:** an **infrastructure error** (Executor crashed, network failure) is a Go `error`, retryable at the Engine level, never visible as a Judgment. A **Step-produced finding** (a failing Validator, a `fail`/`repair` Gate verdict) is data flowing through the pipeline — this distinction already exists correctly in `verify/validator.go` (a non-zero exit is a `Result{Passed:false}`, never an `error`) and must be stated explicitly and enforced for every future Step kind, not left as an implicit convention only one package happens to follow.

**Retry semantics — two independent axes, never conflated:**
- **Retry** = same Step, same inputs, transient infrastructure failure. Bounded by count + backoff. May trigger Executor failover (§7). Invisible in the Judgment.
- **Repair** = a different Step, or the same Step re-fed with prior findings as Context, triggered by a Gate verdict. Bounded by Budget + a must-make-progress rule (findings strictly decreasing, else abort rather than loop). Always a distinct, visible `StepRecord` in the Act's trace.

**Tradeoff — list-with-repair-edges vs. full DAG:** a general Step DAG (arbitrary branching, parallel Steps) is strictly more expressive, but every example lifecycle in this RFC's brief — and every one this audit could construct from the current architecture docs — is satisfied by a much simpler shape: **an ordered list of Steps plus named backward edges for repair only.** This keeps replay and audit narration simple (a run is "steps 1..k, plus these repair loops," never an arbitrary graph traversal to reconstruct) at the cost of not supporting parallel or conditionally-branching Steps. Recommend starting here and escalating to a full DAG only when a real Pipeline needs branching or parallelism that a repair edge cannot express — the same depth-before-breadth discipline that kept M0 small.

---

## 6. Pipeline abstraction

| Option | For | Against |
|---|---|---|
| **Code (Go)** — today's `Engine.RunBudgeted` | Fastest to build; type-checked | Not inspectable/diffable/shareable by non-Go authors; violates ADR-0001 R2 (the extension boundary is explicitly not Go); every new shape needs a new binary |
| **Declarative document (YAML authored, JSON canonical)** | Inspectable, diffable, signable, version-controlled; language-agnostic (satisfies R2); matches the archived pre-M0 design's already-argued decision | Needs an owning schema-versioning/evolution ADR (already flagged in the [ADR backlog](../03-adrs/README.md) as unwritten) |
| **Bespoke DSL** (a small expression/control-flow language embedded in the document) | More expressive (conditionals, loops as "code") | Reintroduces control flow into a non-reviewable text format outside the Engine — directly in tension with I1 and principles 6.2/8.3; the archived design explicitly rejected this ("a workflow that needs real control flow is a smell that the logic belongs in a skill") and nothing since has overturned that argument |
| **General graph** | Maximum expressiveness | Complicates replay/audit narration (§5's tradeoff); no current use case needs it |
| **State machine** | Precise runtime semantics | Not itself an authoring format — this is what the Engine *interprets* a document into, not a competing choice |

**Recommendation:** a declarative document (YAML source, canonical JSON) as the authored form, describing the restricted Step-DAG from §5, interpreted by the Engine at runtime as the state machine from §5. This is not a new idea for this codebase — it is the archived pre-M0 design's §7.1/§7.2 decision, unretired in substance (only "Workflow"→"Pipeline", "Stage"→"Step" in name), and it has not been contradicted by anything ratified since it was shelved.

Scored against RFC-0001 §13's rubric:
1. *Durable or disposable?* A versioned, diffable Pipeline definition is durable capital (a project's authored process), unlike control flow buried in a Go binary.
2. *Provenance & audit?* A run's Steps are exactly its declared Steps — fully auditable from the document plus the Act's Step trace.
3. *Control flow?* Stays with the deterministic Engine interpreter; never the document itself, never a model.
4. *Accountability?* Unaffected — `Approve` Steps still require an Authority regardless of how the Pipeline is authored.
5. *The simple path?* Preserved by construction: the built-in default Pipeline is authored to reproduce today's exact fixed shape, invisible to a user who never asks for a different one (§9 Phase 3).
6. *Vendor capture?* No Go required to author or read a Pipeline; satisfies ADR-0001 R2 directly.
7. *Honesty?* No new determinism claim is made — the document's *structure* is what replays, per I2/I3, unchanged.
8. *The loop?* Neutral — this RFC does not touch Knowledge; a future Knowledge-authoring Pipeline benefits from the same mechanism.
9. *Reversibility?* Cheap while pre-1.0 (schema has no external consumers yet); becomes a real compatibility surface the moment a second Foundry instance reads a Pipeline definition — hence Phase 0's prerequisite ADR in §9.

---

## 7. Multi-model support

- **Capability declaration, not vendor pinning, as the default.** A `Generate` Step names *what it needs* (`role: plan`, `role: implement, tool_use: true`, `role: review`), not *which vendor* — this is the concrete mechanism that keeps I12 ("the model is substrate, never a domain concept") true under multi-model pipelines instead of merely aspirational.
- **Three layers, cheapest-first:**
  1. **Explicit pin** in the Pipeline definition (`step: plan, executor: gpt-5-config`) — deterministic, fully auditable, the only layer needed until a second Executor genuinely competes for the same Capability. This is the *only* layer built in Phase 6 (§9).
  2. **Capability-based negotiation** (the Router picks the best match under a cost/latency/quality/privacy policy) — built only once ≥2 real Executors advertise the same Capability in production (Phase 7).
  3. **Failover chain** for availability — recorded as Evidence when it fires, so an audit can see "GPT was requested but Claude actually executed this Step because of a failover," never silently.
- **No new trust path.** Every Executor's output, regardless of which model backs it, is exactly as untrusted as any other until the Step's declared `Verify` Gate checks it and an `Approve` Step's Authority accepts it (I4 unchanged). A Reviewer-by-Gemini finding is not "more trusted" than an Implementer-by-Claude patch merely because review sounds more authoritative — both are Outcomes, both pass the identical trust gate.

---

## 8. Human interaction — interactive terminal UX

- **A persistent interactive session** (invoking `foundry` with no subcommand) fronts the same Engine, replacing (or sitting in front of) today's one-shot `foundry do "<intent>" --repo <path>`. Natural-language input becomes the **Intent** for a new Act using the default (or last-selected) Pipeline — the NL layer is a thin CLI-side router to "start an Act with this Intent," never a place where control-flow decisions happen. I1 still holds: the Engine decides what runs, not the chat surface.
- **Minimal slash commands**, mapped onto existing or newly-Step-aware primitives:
  - `/status` — what Act/Step is currently running or last ran (reads the Act's `Steps` trace from §4.5).
  - `/history` — an alias for today's `log`.
  - `/pipeline` — list, select, or inspect available Pipeline definitions (new, since Pipelines now exist as named, swappable documents).
  - `/approve`, `/reject` — an explicit alternative to the blocking `y/n` prompt, useful once `Approve` is a mid-pipeline Step the session can surface as "Approval needed for Step 4/7," not only a final gate.
- **The trust boundary does not get thinner.** An `Approve` Step still blocks and requires an explicit accept whether triggered by free text ("yes, ship it") or `/approve` — the interactive shell is a richer front-end onto the same `PromptForApproval`-equivalent port, never a new or weaker mechanism. This is the concrete way "avoid CLI flags" is achieved without eroding I5: the *ceremony of asking* gets friendlier; *whether* a human is asked never becomes optional by default.
- **Progress narration reuses `Reporter` almost unchanged** — a Pipeline-aware Reporter needs `StepStarted(step)`/`StepFinished(step, judgment)` in place of today's fixed `Executing`/`Verifying`/`Repairing`, which are already special cases of "a step started/finished."
- **The scriptable, non-interactive surface stays available and unchanged in kind**: `foundry do "<intent>" --repo <path> --pipeline <name> --yes` remains the CI entry point (system-context.md's "same behavior locally and in CI"); the interactive shell is additive, not a replacement.

---

## 9. Migration strategy

Each phase compiles, keeps `go test -race ./...` green, and leaves `foundry do "<intent>" --repo <path>` with no new flags behaving identically to today, until the phase whose entire point is to change behavior (Phase 4) — and even then, the *default* Pipeline is authored to reproduce today's exact shape.

**Phase 0 — Prerequisite ADRs (no code).** Write the two backlog ADRs this migration needs before its compatibility surfaces harden: (a) reusable-Act-template / Pipeline-definition format & evolution policy, (b) Executor contract & capability model. Both are already identified as required in the [ADR backlog](../03-adrs/README.md); this migration is the forcing function to write them now, not a new discovery.

**Phase 1 — Add the Step trace to `Act`, additively.** Introduce `Act.Steps []StepRecord` (§4.5) alongside every existing flat field; populate it from *inside* today's unchanged `RunBudgeted`/`repairOnce` (Step 1 = gather+execute+verify, Step 2 = repair, if it ran). No behavior change; existing golden tests pass unmodified; new tests assert the trace shape. Payoff: the schema is proven before anything depends on it, and per-Step Record checkpointing (write a `StepRecord` as each step finishes, not only at the very end) can land here — the resumability win arrives before Pipeline exists at all.

**Phase 2 — Extract a `Strategy` interface with exactly one implementation.** `FixedStrategy` is today's `RunBudgeted`/`repairOnce` logic, moved behind `Strategy` unchanged. `Engine.Run(ctx, intent, strategy)` replaces `Engine.Run(ctx, intent)`; `cmd/foundry` wires `FixedStrategy` everywhere. Byte-for-byte identical behavior; this is the seam limitation #8 identifies, and introducing it costs nothing.

**Phase 3 — Introduce the declarative Pipeline format and `PipelineStrategy`.** Ship alongside `FixedStrategy`, not replacing it. The built-in default Pipeline definition is authored to be *exactly* today's fixed shape, so `foundry do` with no `--pipeline` flag is identical before and after, verified by the same golden tests as Phase 1. A second, different Pipeline (e.g., the Plan→Approval→Implementation example) can now be authored and tested with zero Engine code changes — proving the abstraction before `FixedStrategy` is ever deleted.

**Phase 4 — Move Approval and Record into declared Step kinds.** `cli.CLI.Do` shrinks to parse args → resolve Pipeline → run Strategy → done; approval and recording already happened as Steps during the run. This is the phase that removes hardcoded-single-approval and hardcoded-single-recording-point (limitations #3, #9), and where end-to-end resumability becomes real (a crash mid-pipeline leaves a partial Act the next invocation can detect).

**Phase 5 — Retire `FixedStrategy`.** After `PipelineStrategy` with the default definition has matched `FixedStrategy`'s golden-test output in production/CI for a burn-in period, delete the now-dead direct-call logic in `engine.go`/`repair.go`. Purely a cleanup phase; nothing user-visible changes.

**Phase 6 — Router with exactly one policy: explicit pin.** Add a `Router` mapping a Step's declared Capabilities to a configured Executor, with only "this Step uses this Executor" as policy — matching today's single-Executor-per-process reality but now expressible per-Step. This is what first makes "Planner→GPT, Implementer→Claude" nameable in one Pipeline definition, without building any negotiation logic yet.

**Phase 7 — Capability-based negotiation policy.** Cost/latency/quality/privacy weighting and failover, built only once a real multi-Executor Pipeline in production motivates it — the same depth-before-breadth discipline every prior milestone used.

**Phase 8 — Interactive terminal shell.** Built entirely on the Pipeline-aware Engine and CLI primitives from Phases 1–4; no further Engine changes required, since `/status`, `/history`, `/pipeline`, and an approval front-end all read or drive ports established earlier.

---

## 10. Risks

**Architectural:**
- Over-generalizing the Step graph into a full DAG before a real use case needs branching — mitigated by capping at "ordered list + repair edges" (§5) until proven insufficient.
- A declarative Pipeline interpreter becomes a second place control flow can hide if it is not held to engine.go's own rigor (schema validation, versioning, tests) — left unchecked, this quietly reopens I1.
- The `Act`-as-flat-struct → `Act`-as-Step-trace change (§4.5) is the highest-blast-radius schema change in this migration; Phase 1's additive-only approach is the mitigation, not an afterthought.

**Migration:**
- Phase ordering matters: building the Router (Phases 6–7) before the Step-trace/Pipeline phases (1–4) would force Capability plumbing through the old flat `Act` shape twice.
- Golden-test coverage must be audited *before* Phase 1 begins — "identical behavior" claims through Phases 1–5 are only as strong as the tests actually pinning today's full Act JSON shape, not just spot fields.

**Backward compatibility:**
- `.foundry/acts/<id>/act.json` becomes a real compatibility surface the moment a second reader exists (a teammate, CI, three years later) — hence additive-only schema evolution until the persistence/on-disk-layout ADR (Phase 0) exists, echoing the archived freeze review's core lesson (don't harden a compatibility surface with no owning decision).
- The CLI surface (`foundry do "<intent>" --repo <path>`) must keep working with zero new required flags through every phase — the "simple path stays simple" principle applies to the migration itself, not only the end state.

**Performance:**
- Per-Step Context gathering could multiply gatherer calls versus today's single up-front Gather; needs a shared per-Act Context cache so re-gathering isn't paid at every Step.
- Router negotiation (Phase 7) must not add network round-trips to "ask" an Executor its Capabilities at call time — capability advertisement should be a registration-time fact, or it reintroduces the exact cold-start latency concern [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) R4 protects against.

**Complexity:**
- The honest cost: today's ~2,000-line M0 codebase is genuinely simple, with one obvious code path. Every phase above trades some of that simplicity for generality. The default single Pipeline is designed to stay invisible to a user who never asks for more (so the *user-facing* trivial path is protected), but the Engine and CLI *source* will unavoidably be harder to read once Phase 3+ lands — a real, named cost, not a free lunch.

---

## 11. Final recommendation

Adopt the target architecture in §4: the Engine as a generic Strategy runtime whose first, best-supported Strategy is a genuinely data-driven Pipeline (a declarative document, interpreted as a restricted Step-DAG with repair edges — §5, §6); approval, repair, and recording as Step-level concerns rather than hardcoded call sites; a Router matching Capabilities to Executors as the mechanism for multi-model pipelines (§7); an interactive terminal shell built on top of the same ports (§8). Sequence the work as the eight phases in §9, starting with the two prerequisite ADRs in Phase 0 — they gate real compatibility surfaces and should not wait for the rest of this RFC to be settled.

This is deliberately **not** a re-architecture: every element above is either already present in the codebase in miniature (the `Executor` port, `StagedVerifier`'s decorator pattern, the `Reporter` observer, the Validator/Gate error-vs-finding split) or already designed and reasoned through in the archived pre-M0 architecture document, shelved only because M0 correctly chose not to build a framework before it had one real use case. What changes now is that the "one real use case" condition this RFC is written to satisfy — configurable multi-model, multi-verification, multi-approval pipelines — did not exist when M0 was scoped, and does now.

A governance process now exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but this RFC has not yet been individually ratified through it; until it is, it should be treated exactly as [RFC-0001](RFC-0001-vision-and-product-philosophy.md) is treated — Draft, Proposed, argued with rather than deferred to.

---

_This RFC is meant to be argued with. Disagreement is contribution — open an amendment or a counter-RFC citing the section number. Its central falsifiable claim is narrow and checkable: every example lifecycle in this document's brief, and every one in the current architecture docs, is expressible as an ordered Step list with named repair back-edges, with no new Engine code once §4–§6 land. If a real pipeline is found that this shape cannot express, that is the trigger to escalate to a full Step DAG (§5) — not a reason to have built one up front._
