# Multi-Executor Router & Publish Policy — Component Design & Implementation Plan

> **Executable roadmap** for [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md)'s six sequenced pieces. This guide is the "how, in order"; RFC-0004 is the "why". Each commit below leaves the repository compiling with tests green, mirroring [M0-IMPLEMENTATION-BACKLOG.md](M0-IMPLEMENTATION-BACKLOG.md) and [interactive-session-implementation-plan.md](interactive-session-implementation-plan.md)'s discipline.
>
> **Detail level.** Piece 1 (Capability + Router + `feeds_forward`) is fully sequenced into commits below — it is next up, provable entirely with the existing Claude Code Executor, zero new vendor risk. Pieces 2–6 (RFC-0004 §6) are given component-level design only; each gets its own commit plan once its turn comes, so this guide does not lock in detail for work that is still gated on an ADR or an external decision.

---

## Piece 1 — Capability, Router (explicit pin), and `feeds_forward`

### 1. Audit — what's reused, what's new

| Piece | Today | Verdict |
|---|---|---|
| `engine.Step{ID, Kind}` | No Capability, no executor pin | **Additive fields**: `Capability map[string]string`, `Executor string`, `FeedsForward bool`. Every existing `Step` literal (Go tests, `DefaultPipeline()`) keeps compiling — new fields default to zero values that mean "today's exact behavior." |
| `engine.DecodePipelineDocument` / `PipelineDocument` (`engine/document.go`) | Decodes `id`/`kind` per Step | **Additive decoding**: `capability`, `executor`, `feeds_forward` become optional JSON keys. A document that omits them decodes identically to today. |
| `engine.Engine` — one `executor Executor` field | Every Generate Step calls the same `Executor` | **Not removed.** Becomes the Router's *default* — a Step with no `executor` pin resolves to exactly this, so `foundry do`'s existing single-Executor Pipelines are unaffected byte-for-byte. |
| `engine/strategy.go`'s `runSteps` | Calls `rc.executor.Execute(...)` unconditionally for a `StepKindGenerate` | **One call site changes**: resolves through the new `Router` first. Everything else in `runSteps` (checkpointing, `stopsShortOnFailure`, recording) is untouched. |
| `considered []string` threading | Fixed per attempt; only `repairContext` (repair-only, `attempt > 0`) appends to it | **New, opt-in path**: when a Step declares `feeds_forward: true`, the immediately-preceding Step's `Produced`/`Checked` is appended once, reusing `repairContext`'s own rendering helper. Default (`false`, every Pipeline shipped today) is unchanged. |

**Conclusion:** no existing Pipeline document, no existing test fixture, and no existing `Engine`/`Strategy` call site outside the two named above needs to change to make this additive.

### 2. Design decisions this plan is built on

1. **Router has exactly one policy: explicit pin, or the Engine's default Executor.** No negotiation, no capability matching against advertised properties — RFC-0002 §7 layer 2 is explicitly out of scope until a real multi-Executor Pipeline in production motivates it.
2. **`ExecutorRegistry` is a new type, named to avoid "Provider."** [terminology.md](../05-reference/terminology.md) retires "Provider" for anything touching model access; this mirrors `PipelineRegistry`'s register-once/look-up-by-name shape without reusing that retired word.
3. **`.foundry/executors.json` is project-local data, read the same way `.foundry/pipelines/*.json` already is** — a new, small `project` package addition, not a new subsystem. Absence of the file means "only the process default Executor exists," so a project that never opts in sees no change.
4. **`feeds_forward` names no Step — only "the one immediately before."** Naming an arbitrary earlier Step (the way `RepairPolicy.Target` does) is deferred until a real Pipeline needs it (RFC-0004 §3).
5. **The Router only resolves Generate Steps in this piece.** A model-backed `Verify` (RFC-0003 §3.3's "Copilot does code review") is a new `verify.Validator` implementation that itself calls a `Router`-resolved Executor internally — a `verify`-package concern layered on the same `ExecutorRegistry`, not a change to `Verify`'s Engine-level handling. Out of scope for Piece 1's commits below; named here so Piece 1's `ExecutorRegistry`/`Router` types are shaped to be reusable by it later without rework.

### 3. Component design

| Component | Package (new/existing) | Responsibility | Interface (indicative) | Depends on | Reuses |
|---|---|---|---|---|---|
| **Step schema fields** | `engine` (existing file) | Carry a Step's optional Capability, executor pin, feeds-forward flag | `Capability map[string]string`; `Executor string`; `FeedsForward bool` | — | `Step`'s existing `{ID, Kind}` shape, unchanged |
| **PipelineDocument decoding** | `engine` (existing `document.go`) | Decode the three new optional JSON keys | extends existing `DecodePipelineDocument` | `Step`'s new fields | The exact decoder every Pipeline already goes through |
| **ExecutorRegistry** | `engine` (new file) | Register named Executors, look one up by name, refuse duplicate registration | `Register(name string, e Executor) error`; `Get(name string) (Executor, error)` | `Executor` (unchanged port) | `PipelineRegistry`'s pattern, not its code |
| **Router** | `engine` (new file) | Resolve a Step's Executor: its pin if set and registered, else the Engine's default | `Resolve(step Step) (Executor, error)` | `ExecutorRegistry`, a default `Executor` | Nothing — new, small, pure logic over existing types |
| **`feeds_forward` rendering** | `engine` (existing `strategy.go`/`steps.go`) | Append the immediately-preceding Step's output to a Step's Context, once | reuses `repairContext`'s rendering, generalized to take a `StepRecord` instead of only a `Judgment` | `domain.StepRecord` | `repairContext`'s existing string rendering |
| **`.foundry/executors.json` loading** | `project` (existing package, new file) | Read and decode the project-local Executor configuration into constructible `Executor`s | `LoadExecutorConfig(root string) (map[string]ExecutorConfig, error)` | Vendor-specific constructors (Piece 3) | `FilesystemPipelineProvider`'s existing "flat directory, no recursion" reading pattern |

### 4. Sequencing rationale

Bottom-up, same discipline as every prior migration in this codebase: pure schema/decoding first (no behavior change, provable in isolation), then the Router as new, small, unwired logic, then the one call-site change in `runSteps`, then `feeds_forward`, then the project-local config file last (it is the only piece that needs a real filesystem fixture).

### 5. Commit plan

Each commit compiles and passes `go vet`, `go test ./...`, `go test -race ./...` on its own.

**Commit 1 — `feat(engine): Add Capability, Executor pin, and FeedsForward to Step`**
`Step` gains the three new fields; `PipelineDocument`/`DecodePipelineDocument` gain matching optional JSON decoding. Tests: a document omitting all three decodes identically to today (golden-test regression guard); a document declaring all three decodes them correctly; an unset `executor` on `Step` is the empty string, never a nil-pointer risk.

**Commit 2 — `feat(engine): Add ExecutorRegistry`**
New file, `engine/executor_registry.go`. Register-by-name, duplicate-registration and unknown-name lookup both fail with named errors — the exact shape `PipelineRegistry` already established. Tests use fake `Executor`s; no real vendor code yet.

**Commit 3 — `feat(engine): Add Router with explicit-pin-only policy`**
New file, `engine/router.go`. `Resolve(step)` returns the pinned Executor if `step.Executor != ""` and registered, a clear named error if pinned but not registered (never a silent fallback), or the Engine's default Executor if unset. Tests cover all three paths.

**Commit 4 — `feat(engine): Wire Router into runSteps for generate Steps`**
The one call-site change: `runSteps`'s `StepKindGenerate` case resolves through `rc.router` instead of calling `rc.executor` directly. `Engine` gains a `SetRouter`-style wiring point defaulting to "every Step routes to today's single configured Executor" — so an `Engine` that never opts in behaves byte-for-byte as before. Tests: a Pipeline with two Generate Steps pinned to two different fake Executors proves both actually get called, in order, with the right one; a Pipeline with no pins behaves exactly like every existing engine test.

**Commit 5 — `feat(engine): Add feeds_forward Context threading`**
Generalizes `repairContext`'s rendering to take a `domain.StepRecord` (Produced or Checked, whichever is non-empty) instead of only a `*domain.Judgment`. When a Step declares `FeedsForward: true`, `runSteps` appends the immediately-preceding `StepRecord`'s rendered output to that Step's `considered` before calling Generate. Tests: a three-Step Pipeline (`Verify` → `Generate` with `feeds_forward: true`) proves the Verify Step's `Checked` findings actually reach the Generate call's `considered`; a Step without the flag sees no change, proving the existing behavior (and every existing test) is untouched.

**Commit 6 — `feat(project): Add LoadExecutorConfig for .foundry/executors.json`**
New file in the existing `project` package. Reads a flat JSON map of name → `{vendor, model, api_key_env}`; missing file is not an error (mirrors `FilesystemPipelineProvider`'s "missing directory → empty, not an error"). This commit only *decodes* the config — constructing real vendor `Executor`s from it is Piece 3, gated on the Executor-contract ADR (RFC-0004 §6 point 2). Tests: missing file → empty map; a valid file decodes; a malformed file surfaces a clear, named error.

### 6. Explicitly out of scope for Piece 1

- **A second real vendor Executor.** `LoadExecutorConfig` (Commit 6) decodes configuration; it does not construct an OpenAI- or Copilot-backed `Executor` — that's Piece 3, and RFC-0004 §6 puts writing the Executor-contract ADR (Piece 2) before it.
- **Model-backed Verify / qualitative code review.** Named in §2's design decision 5 so the types shaped here are reusable by it later, but no `verify.Validator` change happens in Piece 1.
- **Capability-based negotiation** (RFC-0002 §7 layer 2). Explicit pin only.
- **`feeds_forward` naming an arbitrary earlier Step.** Only "the one immediately before," per RFC-0004 §3.

---

## Pieces 2–6 — component-level design only (each gets its own commit plan when its turn comes)

### Piece 2 — Write the "Executor contract & capability model" ADR — **drafted**
Documentation only, no code. Resolves, before Piece 3's adapter is written: how a vendor's invocation shape (subprocess CLI vs. pure API), cost/telemetry reporting, and capability-truthfulness are normalized across `Executor` implementations (RFC-0004 §2.3).

Drafted as [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) — **Status: Proposed, not ratified** (no governance process exists yet, [OQ-006](../06-open-questions/OQ-006-governance-model.md)). It keeps `engine.Executor.Execute` unchanged, adds an optional `CostEstimator` interface Piece 5 can type-assert for, and rules that Capability truthfulness stays declared-not-verified while routing is explicit-pin-only. Piece 3 may proceed against this proposed shape; ratifying it is a separate, still-blocked step.

### Piece 3 — One additional real vendor `Executor` — **shipped**
New package, `executor/openai` (RFC-0004 §2.3's recommendation: a clean-API vendor before a literal Copilot adapter). Satisfies the unchanged `engine.Executor` interface; also implements the optional `engine.CostEstimator` (ADR-0005 Decision 3), the seam Piece 5 reads from. Constructed from `LoadExecutorConfig`'s decoded entries (Piece 1, Commit 6) via `project.BuildExecutorRegistry`/`ExecutorConstructor`; registered into an `ExecutorRegistry` at the composition root (`cmd/foundry/commands/do.go`'s `wireEngine`, and `session.Session`), the same place `claude.NewClaudeExecutor` is wired today.

### Piece 4 — Knowledge-lite capture — **shipped**
Two new `apply` targets, `knowledge-note` and `project-doc`, alongside today's only target (`local`, implicit) — RFC-0004 §2.6's explicitly minimal write, not a memory system: no indexing, retrieval, or provenance schema, and nothing here preempts the "Knowledge & semantic store" ADR backlog entry or roadmap.md's M4.

**What changed:**
- `engine/step.go`: `Step` gains `Target string`, meaningful only for a `domain.StepKindApply` Step (additive, zero value `""` — every Pipeline predating it keeps its exact behavior). New constants `ApplyTargetLocal`, `ApplyTargetKnowledgeNote`, `ApplyTargetProjectDoc`. `engine/document.go`'s `StepDocument` decodes an optional `target` JSON key the same way.
- `engine/applier_registry.go` (new): `ApplierRegistry`, mirroring `ExecutorRegistry`'s register-by-name/look-up-by-name shape. Unlike `ExecutorRegistry` paired with `Router`, there is no "default" resolution inside it — `ApplyTargetLocal` (or an unset Target) never touches the registry at all.
- `engine/strategy.go`: `runContext` gains `applierRegistry *ApplierRegistry` and a `resolveApplier(target)` method — `""`/`ApplyTargetLocal` returns `rc.applier` (the Engine's single configured Applier, byte-for-byte unchanged), any other Target resolves via the registry, erroring clearly (never a silent fallback) if unregistered. `runSteps`' `StepKindApply` case calls it before `Apply`.
- `engine/engine.go`: `Engine` gains an `applierRegistry` field, defaulting to an empty `ApplierRegistry` in `NewEngine`, plus a `SetApplierRegistry` setter mirroring `SetRouter`.
- `project/config.go` (new): `Config{DocsPath string}` and `LoadConfig(root)`, reading `.foundry/config.json`'s `docs_path` — missing file decodes to the zero `Config`, mirroring `LoadExecutorConfig`'s pattern. §2.5's `require_approval_before_remote_publish` is a field Piece 6 adds to this same file later; `LoadConfig`'s shape doesn't need to change for that.
- `workspace/knowledge_applier.go` (new): `KnowledgeNoteApplier` writes `act.Patch` (the prose a preceding Generate Step produced — Outcome's one content field, reused for prose exactly as `local` already reuses it for a git diff) to a new file per Act under the fixed `.foundry/knowledge/` directory, named `<act-id>-<slug>.md`; `ProjectDocApplier{DocsPath}` appends the same content, under a heading naming the Act, to one project-relative file multiple Acts write into over time — and refuses clearly if `DocsPath` is empty rather than silently no-op'ing.
- Both composition roots (`cmd/foundry/commands/do.go`'s `wireEngine`, `session.NewSession`/`Session.Engine`) gained a small, duplicated `buildApplierRegistry(root)` — registers `knowledge-note` unconditionally and `project-doc` only if `project.LoadConfig` reports a non-empty `DocsPath`. Duplicated rather than shared, matching this codebase's existing precedent of both composition roots independently building their own Gate/Verifier.

Tests: `engine/applier_registry_test.go` (register/duplicate/unregistered-lookup); `engine/applier_routing_test.go` (an apply Step with no Target still uses the configured Applier; a registered Target routes to the named Applier, never the default; an unregistered Target fails clearly, never falling back); `engine/document_test.go`'s decode tests extended for `target`; `project/config_test.go` (missing/valid/malformed); `workspace/knowledge_applier_test.go` and an internal `slugify` test (note-per-Act, append-across-Acts, empty-`DocsPath` failure, slug edge cases).

### Piece 5 — Per-Step Budget accounting — **shipped**
`engine/budget.go`'s `tracker.charge` moved from one flat estimate per attempt to a per-Step estimate, summed (RFC-0004 §2.7). Before this change, `PipelineStrategy.Produce` charged once per attempt, before any of that attempt's Steps ran — a Pipeline with more than one Generate Step per attempt (e.g. `feature.json`'s `plan` + `implement`) was charged as if it made one Executor call, regardless of how many it actually made.

**What changed:**
- `engine/cost_estimator.go` gained `estimateExecuteCostUSD(ctx, executor, intent, considered)`: type-asserts the Step's *resolved* Executor (post-`Router.Resolve`, so a per-Step pin is priced correctly, not the Pipeline's default) for `CostEstimator` and calls it; falls back to the flat `executeCostEstimateUSD` constant for an Executor that doesn't implement it (`executor/claude.ClaudeExecutor`, `executor.ScriptedExecutor` — no change required to either).
- `engine/strategy.go`'s `runSteps` `StepKindGenerate` case now resolves the Step's Executor, estimates its cost, and charges `rc.spent` *before* calling `Execute` — once per Generate Step, not once per attempt. `Produce`'s attempt loop no longer pre-charges; it distinguishes `ErrBudgetExceeded` from any other Step error returned by `runSteps` and applies the same halt-on-first-attempt / skip-repair-on-later-attempt policy as before, now correctly triggered mid-attempt if a later Generate Step (not only the first) exhausts the Budget.
- `engine.Reporter`'s `Executing`/`Verifying`/`Verified` now receive the Pipeline *attempt* number (`attempt + 1`) rather than the tracker's running charge count — the two diverge the moment a Pipeline has more than one Generate Step per attempt, and the Reporter's documented contract ("iteration is 1 for the first attempt, 2 for the bounded repair") is about attempts, not Executor calls.
- `DefaultBudget()`'s constants moved from `MaxIterations=2, MaxCostUSD=$1.00` to `MaxIterations=4, MaxCostUSD=$2.00` — sized to keep `feature.json`'s declared `repair.max_attempts: 2` reachable under the flat fallback rate (`plan` + `implement` + up to two repaired `implement` calls = 4 Executor calls). Retuning this alongside the accounting fix was a deliberate choice, not a RFC-0004 requirement: without it, the more accurate accounting alone would have silently made `feature.json`'s repair unreachable under the old default ceiling. Revisiting these constants with real per-vendor cost data is still the "Cost as a first-class constraint" ADR's job (`docs/03-adrs/README.md` backlog), not settled here.

Tests: `engine/cost_estimator_test.go` (the fallback and `CostEstimator` paths in isolation); `engine/cost_accounting_test.go` (an end-to-end Pipeline with two Generate Steps pinned to two differently-priced Executors, proving the charge sums correctly and that a Budget refusal can now happen mid-attempt, before the second Step's `Execute` is ever called); `engine/pipeline_golden_test.go`'s golden `feature` Pipeline run updated from `Iterations == 2` to `Iterations == 3` (`plan`, `implement`, `implement` repaired — three real Executor calls, now three real charges).

### Piece 6 — VCS/PR publish policy — **shipped**
Drafted as [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md) — **Status: Proposed, not ratified** (no governance process exists yet, [OQ-006](../06-open-questions/OQ-006-governance-model.md)) — and now implemented against that proposed shape, the same way Piece 3 proceeded against ADR-0005 before ratification. `Apply`'s meaning and `engine.Applier`'s contract are unchanged; `remote-pr` is Foundry's first apply target that leaves the developer's own machine.

**What changed:**
- `engine/step.go`: new constant `ApplyTargetRemotePR = "remote-pr"`, resolved through the same `ApplierRegistry` Piece 4 already built — no `runSteps` change was needed.
- `project/config.go`: `Config` gains `RequireApprovalBeforeRemotePublish bool` (`require_approval_before_remote_publish`) and `RemotePublishTokenEnv string` (`remote_publish_token_env`), mirroring `ExecutorConfig.APIKeyEnv`'s credential-by-reference pattern.
- `engine/registry.go`: `PipelineRegistry` gains `SetPublishPolicy(bool)` and a private `requireApprovalBeforeRemotePublish` flag. When set, `Register` (and therefore `RegisterMany`) refuses — leaving the registry unchanged, wrapping the new `ErrRemotePublishRequiresApproval` — any Pipeline declaring a `remote-pr` apply Step with no `approve` Step earlier in its `Steps` sequence. This is a load-time check, not a runtime one: a misconfigured Pipeline never reaches an Act at all.
- `project/project_loader.go`: `ProjectLoader.LoadRegistry` gained a `Config` parameter and calls `registry.SetPublishPolicy(cfg.RequireApprovalBeforeRemotePublish)` before registering either built-in or project-local Pipelines — the one place a project-authored Pipeline declaring `remote-pr` is ever registered, so the one place the policy can take effect.
- `workspace/workspace.go`: `Workspace` gains `Commit(ctx, message)` (stage + commit — unlike `Land`, which never commits and instead carries an uncommitted working-tree diff back onto the developer's own branch), `Push(ctx, remote)` (push with `-u`), and `BranchName()` (accessor) — the primitives a remote apply target needs that a purely local one never did.
- `vcs` (new package): `GitHubPRApplier{TokenEnv, Out}` implements `engine.Applier`. `Apply` builds a `workspace.Workspace`, applies + commits + pushes `act.Patch` to a throwaway `foundry/act-<id>` branch, shells out to `gh pr create` (subprocess, not an embedded API client — the same posture `executor/claude` and `git apply` already establish) with `GH_TOKEN` set only for that one subprocess call from the environment variable `TokenEnv` names, then `Clean`s the local throwaway branch — the pushed remote branch and opened PR are the durable, terminal result (ADR-0010 Decision 5). An injectable `ghRunner` seam (mirroring `executor/openai`'s `doer` pattern) means tests never need a real `gh` binary, network access, or credentials.
- Both composition roots (`cmd/foundry/commands/do.go`'s `wireEngine`, `session.NewSession`) load `project.Config` once and thread it through: to `buildApplierRegistry` (registers `vcs.GitHubPRApplier` under `ApplyTargetRemotePR` only if `RemotePublishTokenEnv` is set) and, in `session.NewSession`, to `ProjectLoader.LoadRegistry` for the publish-policy check. `session.Session` now stores `cfg` for the session's lifetime, reused by `ReloadPipelines` (no second file read) — the same session-lifetime treatment `executors`/`appliers` already get. `wireEngine` does not call `SetPublishPolicy`: it resolves Pipelines only from `engine.NewDefaultRegistry()` (built-ins, neither of which declares an apply Step), so the policy has nothing to enforce there yet — it becomes relevant only if `foundry do` ever gains project-local Pipeline support.

Tests: `engine/registry_test.go` (policy unset allows `remote-pr` without `approve`; policy required refuses/allows accordingly, leaving the registry unchanged on refusal; other targets are unaffected); `project/config_test.go` (decodes the two new fields); `workspace/commit_push_test.go` (`Commit`/`Push`/`BranchName` against a real local bare-repo remote fixture — including that `Clean` only removes the *local* throwaway branch, never the pushed remote one); `vcs/github_pr_applier_test.go` (missing/unset credential fails clearly; a successful run commits+pushes+calls `gh` with the right branch/env/output and cleans up locally; a `gh` failure propagates); `cmd/foundry/commands/do_test.go` and `session/publish_policy_test.go` (both composition roots wire `ApplyTargetRemotePR` correctly, and `NewSession` refuses/allows a project's `remote-pr` Pipeline exactly per its `require_approval_before_remote_publish` setting).
