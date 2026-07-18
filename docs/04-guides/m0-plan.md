# M0 Implementation Plan

> **Status: M0 is complete**, and the codebase has since shipped well past it. This document is kept as the design rationale behind *why* M0 was scoped and sequenced the way it was — not as a current plan. For current status per milestone, see [../00-overview/roadmap.md](../00-overview/roadmap.md); to install and run Foundry today, see [getting-started.md](getting-started.md). "Atlas" below was always a placeholder name for whichever real codebase M0 was first pointed at, not a specific repository.
>
> **Goal of M0:** the first *usable* Foundry — capable of producing real, verified, human-approved, recorded code changes on the **Atlas** codebase. Documentation is done; a working system is the deliverable. This plan supersedes the M0 detail in the archived roadmap and uses the provisional vocabulary from [terminology.md](../05-reference/terminology.md). Build approach: [development.md](development.md).

## 1. Strategy & philosophy

**Validate the architecture with the thinnest possible end-to-end Act, then make it useful.** One vertical slice — Intent → Strategy → Execution → Evidence → Validation → Judgment → Record — running on the real Atlas repo, before any breadth.

Three rules govern every M0 decision:
1. **One concrete path, not a framework.** Build exactly one Strategy, one Executor kind at a time, one Gate rule. No Router, no plurality, no config language. The only abstraction M0 builds is the **Executor interface** — and only because it has *two real users from day one* (a scripted impl for deterministic tests, one real impl for developing Atlas).
2. **Fake or hardcode everything that isn't on the critical path to proving the Act lifecycle.** Context is a naive heuristic. Knowledge is the Record. Budget is two constants. Authority is the terminal user pressing `y`.
3. **Delete scope aggressively.** The deferred list (§9) must dwarf the included list (§8). When unsure, defer.

**M0 is also an experiment.** Building the Act lifecycle is the cheapest way to gather evidence for the open questions it depends on — especially [OQ-001 (Act vs Knowledge center)](../06-open-questions/OQ-001-domain-center.md) and [OQ-002 (Pipeline as Strategy)](../06-open-questions/OQ-002-pipeline-as-strategy.md). We are not waiting on those; we are informing them.

## 2. What the docs already give us (analysis)

- **Implementation-ready:** Act lifecycle (domain.md), the trust gate (validate → judge), deterministic-first verification, human-as-Authority, the immutable Record, worktree isolation (concept survives from archived ARCHITECTURE §7.3). These map directly to code.
- **Postpone (concept exists, not needed for M0):** Knowledge graph & Authored/Derived split, Context *engine* (ranking/compaction/provenance scoring), Router & Capability negotiation, multiple Strategies, Skills/reusable templates, replay.
- **Open questions that DO NOT block M0:** all of them. OQ-001/002 are *answered provisionally* and M0 tests them. OQ-003 (replay) and OQ-005 (extensions) are deferred features. OQ-004 (validator-determinism) is sidestepped — M0 just runs validators and records results. OQ-006 (governance) is resolved ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)); while it was open it blocked *ratifying* decisions, not *writing code*, which is why M0 work was never gated on it. OQ-007 (terminology) only affects names, which are cheap to change pre-1.0.
- **The only real prerequisite:** the language decision ([ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md), Go, interim) — sufficient. Plus knowing Atlas's build/test commands (config).

**Conclusion: nothing blocks starting M0.**

## 3. The walking skeleton

The absolute minimum that proves the architecture:

```
foundry do "<intent>" --repo <atlas>
  → Engine opens an Act
  → Context (naive: files named in intent)            [considered Evidence]
  → Executor (SCRIPTED: returns a fixed patch)         [Outcome]
  → Workspace applies patch to a throwaway git branch
  → Verify: run Atlas `build` + `test`                 [checked Evidence]
  → Gate: all pass? → human sees diff + results → y/n  [Judgment, Authority]
  → Record: write the immutable Act to disk
```

With a **scripted executor**, this entire pipeline is deterministic and golden-testable with **zero network and zero model**. That is the point: prove the Act lifecycle works before introducing LLM nondeterminism. Swapping the scripted executor for a real one (same interface) is what turns the skeleton into a tool that develops Atlas.

## 4. M0 milestones (each independently usable)

| # | Goal | Executable result | Deterministic? |
|---|---|---|---|
| **M0.0** | Walking skeleton | `foundry do` runs the full Act lifecycle with a scripted executor on a sample repo; diff review + approve; Act recorded | **Yes** (no model) |
| **M0.1** | Real work | Swap in one real Executor + iteration/cost caps → `foundry do` produces real diffs on Atlas for simple changes | Partly (cassette-tested engine; live executor) |
| **M0.2** | Repair | On validator failure, feed findings back to the executor once (bounded) → survives test failures | — |
| **M0.3** | Usable | Better naive context + `foundry log` / `foundry show <act>` to inspect history → good enough to use on Atlas daily | — |

**M0 done =** at least one real Atlas change, produced by Foundry, passes Atlas's tests, is approved by a human, is recorded, and is merged.

No milestone is infrastructure-only; M0.0 already ships a usable (if dumb) end-to-end tool.

## 5. Repository / package structure

One responsibility per package; dependencies point inward to `domain`.

```
cmd/foundry/        main: CLI bootstrap + dependency wiring (the only place impls are chosen)
internal/
  domain/           Act, Intent, Evidence, Outcome, Judgment, Authority, Budget — PURE, imports nothing
  engine/           Engine.Run(Act); the single concrete Strategy; DECLARES the ports it needs
  executor/         scripted.go, <provider>.go — satisfy engine's Executor port
  verify/           Validator (run a command), Gate (all-pass) — satisfy engine's Verifier port
  context/          naive gatherer — satisfies engine's Gatherer port   (arrives M0.1)
  workspace/        git branch/worktree + apply patch (isolation)
  record/           filesystem-backed immutable Act store — satisfies engine's Recorder port
  cli/              commands: do, show, log; renders diff; prompts the human Authority
```

## 6. Dependency graph & rules

```
        cmd/foundry ──wires──▶ engine, cli, executor, verify, context, workspace, record
              │
   cli ──▶ engine ──▶ domain
              │  └─(ports: Executor, Verifier, Gatherer, Recorder — declared HERE, in the consumer)
   executor ─┤
   verify ───┤──▶ domain        (adapters import domain only; satisfy engine's ports structurally)
   context ──┤
   workspace ┤
   record ───┘
```

**Forbidden (enforced by review before code exists):**
- `domain` imports nothing but stdlib.
- `engine` never imports an adapter (`executor`, `verify`, …) — it knows only its own port interfaces + `domain`.
- Adapters never import `engine` or each other (avoids cycles; Go structural typing makes this work).
- Nothing imports `cli` except `cmd`.

**Interface ownership:** ports are declared by their *consumer* (`engine`), per Go idiom — this is why no adapter needs to import `engine`, and why there are no cycles. The Executor port is the only one with two implementations in M0; the rest have one and could even be concrete (kept as tiny interfaces only for test seams).

## 7. Concept → code map (mapping only; not implementation)

| Concept | Code form | Package | Lifecycle | Owner |
|---|---|---|---|---|
| Act | `struct Act` | domain | created by engine → mutated through phases → frozen → persisted | domain (type) / engine (lifecycle) |
| Intent | `struct Intent` | domain | captured at CLI | cli |
| Strategy | a concrete method `engine.run()` — **no interface in M0** | engine | — | engine |
| Evidence | `struct Evidence{Considered, Checked}` | domain | accumulated during run | engine |
| Executor | `interface Executor` + `Scripted`, `<Provider>` | engine (port) / executor (impls) | called once per attempt | engine / executor |
| Outcome | `struct Outcome{Patch}` | domain | produced by executor | executor → engine |
| Validator / Gate | `Validator{Cmd}`, `Gate.Evaluate()` | verify | run after Outcome | verify |
| Judgment | `struct Judgment{Verdict, Authority}` | domain | gate verdict + human approve | engine + cli |
| Authority | `struct Authority` (local user) | domain | identifies approver | cli |
| Budget | `struct Budget{MaxIters, MaxCostUSD}` — constants | domain | checked each iteration | engine |
| Record | `Recorder` port + filesystem store | engine (port) / record | write-once per Act | record |
| Knowledge | **deferred** — the Record *is* M0 knowledge | — | — | — |
| Workspace | `Workspace.Apply(patch)` on a branch | workspace | per Act | workspace |

## 8. M0 scope — INCLUDED (small)

`foundry do` (+ `show`, `log`) · Engine with one hardcoded Strategy · domain types · **Executor interface + scripted impl + one real provider** · validators (shell) + all-pass Gate · naive context · immutable Record (filesystem) · git-branch isolation · human approval · iteration + cost caps.

## 9. M0 scope — DEFERRED (large, on purpose)

Router & capability negotiation · multiple providers · multiple/adaptive/agentic Strategies · Knowledge graph, extractors, Authored/Derived store, semantic retrieval · Context *engine* (ranking, compaction, provenance scoring) · Skills / reusable Act templates · replay & replay-across-versions · cassette/mock infra beyond the scripted executor · VCS host integration (GitHub PRs) · CI mode · daemon / TUI / dashboard · observability/OTel · secrets manager (env var only in M0) · plugin/extension SDK & isolation · policy/expression language · budgets beyond two constants · multi-user, RBAC/SSO, audit export · remote/distributed execution · OCI distribution · conformance suites · port versioning · knowledge-update workflow · governance tooling.

## 10. Implementation order (each step unlocks the next)

1. `domain` types (pure) — everything needs them.
2. `record` (write/read one Act) — persistence from the start.
3. `verify` (validators + gate) — deterministic, testable alone.
4. `workspace` (branch + apply patch).
5. `engine` + **scripted** `executor` + `cli do` + approval → **M0.0 walking skeleton** (first executable; golden-tested).
6. `context` (naive) + real `executor` + budget caps → **M0.1** (develops Atlas).
7. repair loop in `engine` → **M0.2**.
8. context tweaks + `cli show`/`log` → **M0.3** (usable).

No abstraction is built before it has a user: the Executor interface appears at step 5 with the scripted impl and gains its second user (real) at step 6.

## 11. Simplification challenge (applied)

- *Can a milestone disappear?* M0.2 (repair) is the smallest; kept because surviving test failures is what makes it actually usable on Atlas, and it's a discrete testable Engine change.
- *Can milestones merge?* Diff-review + approval were pulled **into M0.0** (they are the Judgment surface, not polish). Cost guard folded **into M0.1** (only matters with real $). History inspection is the only thing left in M0.3.
- *Can an abstraction wait?* Strategy, Router, Gatherer-as-interface, Knowledge — all deferred to concrete or nothing.
- *Can a package disappear?* `context` is not created until M0.1 (the scripted skeleton needs none). Considered folding `workspace` into `engine`; kept separate because git is a distinct responsibility likely to grow.
- *Can an interface be concrete?* Yes — only `Executor` stays an interface; `Verifier`/`Gatherer`/`Recorder` are tiny interfaces kept solely as test seams and may start concrete.

## 12. Risks

| Risk | Mitigation |
|---|---|
| LLM nondeterminism makes the engine untestable | Scripted executor → M0.0 is fully deterministic; record real runs as cassettes for M0.1 engine tests |
| Codegen quality too low to "develop Atlas" | Keep intents small; repair loop (M0.2); accept M0 handles small/medium changes only |
| Cost / loop runaway with a real model | Hardcoded iteration + cost caps in M0.1 |
| Scope creep into Router/Knowledge/Strategy | This plan's §9; one concrete path is the rule |
| Git apply/conflict edge cases | Isolate on a throwaway branch; escalate conflicts to the human, never auto-resolve |
| Provisional model (Act center) proves wrong | Acceptable — M0 is how we find out; the cost of learning is one milestone, and renames are cheap pre-1.0 |

## 13. Recommendations before the first line of Go

1. **Pin the Atlas build/test commands** (and a tiny sample repo for M0.0 tests) — these are the validators.
2. **Confirm ADR-0001 is sufficient to proceed** (it is, interim) and pick the single M0 provider + how its key is read (env var).
3. **Write the M0.0 acceptance test first** (scripted executor → recorded Act on the sample repo) — it is the architecture's proof and the regression guard for everything after.
4. **Do not create a package until step 10 reaches it.** No empty scaffolding.
5. **Treat the Executor interface as the only contract worth getting right now;** keep all other interfaces minimal and changeable.
