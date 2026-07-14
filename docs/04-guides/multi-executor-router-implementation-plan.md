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

### Piece 2 — Write the "Executor contract & capability model" ADR
Documentation only, no code. Resolves, before Piece 3's adapter is written: how a vendor's invocation shape (subprocess CLI vs. pure API), cost/telemetry reporting, and capability-truthfulness are normalized across `Executor` implementations (RFC-0004 §2.3).

### Piece 3 — One additional real vendor `Executor`
New package, e.g. `executor/openai` (RFC-0004 §2.3's recommendation: a clean-API vendor before a literal Copilot adapter). Satisfies the unchanged `engine.Executor` interface; constructed from `LoadExecutorConfig`'s decoded entries (Piece 1, Commit 6); registered into an `ExecutorRegistry` at the composition root (`cmd/foundry/commands/do.go`'s `wireEngine`, and `session.Session`), the same place `claude.NewClaudeExecutor` is wired today.

### Piece 4 — Knowledge-lite capture (independent; can land any time after Piece 1)
Two new `apply` targets (`knowledge-note`, `project-doc`) alongside today's only target (`local`, implicit). A Step's `apply` Kind gains an optional `target` field (mirrors §2.1's `capability`/`executor` additions — additive, defaults to `local`). No new package required beyond a small write-helper beside `workspace.GitApplier`.

### Piece 5 — Per-Step Budget accounting
`engine/budget.go`'s `tracker.charge` moves from one flat estimate per attempt to a per-Step estimate, summed. Gated before Piece 1–4's mechanism is exercised on a real seven-Step Pipeline in production, not before it's built and tested (RFC-0004 §2.7).

### Piece 6 — VCS/PR publish policy
Gated on writing the "VCS/PR integration & Apply targets" ADR (RFC-0004 §2.5 proposes its shape). A new `apply` target (`remote-pr`), a new `.foundry/config.json` (`require_approval_before_remote_publish`), and a load-time `PipelineRegistry.Register` check that refuses a Pipeline declaring `target: remote-pr` without a preceding `approve` Step when the project's policy requires one. Sequenced last deliberately — the highest-risk, least-precedented piece, exactly as RFC-0003 §8 already recommended for its own §4.1.
