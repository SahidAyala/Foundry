# Pipelines Guide

> How to read and author a Pipeline document. Concepts: [../05-reference/terminology.md](../05-reference/terminology.md) (**Pipeline**, **Step**). Mechanics: [../02-architecture/execution.md](../02-architecture/execution.md). Design reasoning: [../01-rfcs/RFC-0002-pipeline-execution-runtime.md](../01-rfcs/RFC-0002-pipeline-execution-runtime.md).
>
> **Maturity: PROVISIONAL.** The Pipeline-as-Strategy model is a working hypothesis ([../06-open-questions/OQ-002-pipeline-as-strategy.md](../06-open-questions/OQ-002-pipeline-as-strategy.md)), not a ratified decision.

A **Pipeline** is one **Strategy** for producing an **Act**: a predeclared sequence of **Steps**. It is authored as a small JSON document and decoded by `engine.DecodePipelineDocument` (`engine/document.go`) into the `engine.Pipeline` a `PipelineStrategy` walks.

## Schema

```json
{
  "name": "feature",
  "steps": [
    { "id": "implement", "kind": "generate" },
    { "id": "verify", "kind": "verify" }
  ],
  "repair": {
    "max_attempts": 1,
    "target": "implement"
  }
}
```

- **`steps`** — an ordered list. Each Step has an `id` (unique within the document, doubles as the human-readable name a `repair.target` can point back to) and a `kind`, one of RFC-0002 §4.2's closed five: `generate`, `verify`, `approve`, `apply`, `record`. A Step Kind PipelineStrategy does not recognize is a decode-time error, never a silently skipped Step. A Step may also carry `capability` (object, still unused by any document above — reserved for future capability-based routing, see [ADR-0006](../03-adrs/ADR-0006-routing-and-policy.md)), `executor` (string — a `generate` Step's pin to a named Executor from `.foundry/executors.json`, resolved by `engine.Router`; **used by `issue.json`'s `plan` Step below** to run its analysis on a different model than `implement`'s), `model` (string — ADR-0013, Proposed; a `generate` Step's pin to a named Model from the process's Model Registry instead of an Executor directly — see "Pinning a Step to a Model" below), `preferred` (array of strings — ADR-0013, Proposed; an ordered list of Model IDs, the first of which wins over `model` — see "Preferred model lists" below), `feeds_forward` (bool — used by `engine/strategy.go`'s `appendFeedsForward` to attach the immediately-preceding Step's own recorded output to a later Step's considered Context; not yet used by any document above), and `target` (string — used by `apply` Steps to name an apply Target, e.g. `issue.json`'s `remote-pr`).
- **`repair.max_attempts`** — how many times the Pipeline may re-run after a `verify` Step's Judgment is `fail`. `0` (or an omitted `repair` block) means no repair.
- **`repair.target`** — the Step ID a repair round jumps back to, re-running only from there onward, not the whole Pipeline. Omitted means "restart from the first Step."

A failing `verify` Step always stops the current attempt before any `approve`, `apply`, or `record` Step — a Pipeline never seeks approval for, applies, or records an Outcome its own verification just rejected, whether or not that attempt goes on to repair.

### Field evolution and unknown fields

Per [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md): the fields listed above are the complete schema — there is no document schema-version field yet, and any field not named here is a decode-time error, not a silently ignored key. If you see an error like `unknown field "capabilty"`, it means exactly that: the document has a field this schema doesn't recognize, most often a typo of one of the names above. Fix the field name (or remove it) and re-run. New optional fields, when added, are always additive and `omitempty` — a document written before a new field existed keeps decoding identically once the field is documented here.

### Pinning a Step to a Model instead of an Executor

Per [ADR-0013](../03-adrs/ADR-0013-model-registry.md) (Proposed): a `generate` Step can name `model` instead of (or alongside) `executor`:

```json
{ "id": "plan", "kind": "generate", "model": "claude-sonnet-5" }
```

- `executor` still works exactly as it always did — `model` is entirely optional.
- **If both are set, `model` wins.** `engine.Router.Resolve` looks `model` up in the process's Model Registry to find which Executor it belongs to, and resolves *that* name instead of `executor`'s value.
- **An unknown `model` is a validation failure** — a name not in the Model Registry (or a Model Registry not configured at all) is a clear, named error at Resolve time, never a silent fallback to `executor` or the default Executor.
- **An unknown `executor` behaves exactly as before** — whether the name being resolved came from `model` or from `executor` directly, an unregistered name in `.foundry/executors.json` fails the same way it always did.
- A model's Executor value must match an entry's *name* in `.foundry/executors.json` (e.g. an entry literally named `"gemini"`), not merely share its `vendor`. A project that names its Gemini entry `"planner"` (as this repository's own `.foundry/executors.json` does) needs a `model` pinned Step to resolve against an entry actually named after the vendor, or against `"planner"` directly via `executor` instead — `model` does not search `.foundry/executors.json` by `vendor`. Models whose Executor is `"claude"` (Anthropic's Claude models) cannot resolve via `model` at all today: Claude Code is Foundry's implicit default and is never registered under a name, so this fails as "unknown executor" — correct, deterministic behavior, but a real, named limitation of this first increment (see the ADR's own Consequences).
- What models exist, and which Executor each belongs to, is a fixed catalog built once per process from every Executor package's own `SupportedModels()` (see [implementation-status.md](../00-overview/implementation-status.md)'s changelog) — there is no per-project way to add to it yet.
- The Model Registry (`model.Registry`) also carries a runtime `HealthManager` (`AVAILABLE`/`UNAVAILABLE`/`COOLDOWN`/`UNKNOWN`, plus a `reason` and `retryAt`), queryable via `Registry.Health` — ADR-0013 (Proposed), fifth increment. This is not a Pipeline/Step concept: no field on a Step reads or reports it, `Router.Resolve` never consults it, and a `model`/`preferred`-pinned Step's resolution is completely unaffected by any model's reported health today.

### Preferred model lists and automatic failover

Per [ADR-0013](../03-adrs/ADR-0013-model-registry.md) (Proposed, fourth and sixth increments): a `generate` Step can name `preferred` instead of (or alongside) `model` — an ordered list of Model IDs:

```json
{ "id": "plan", "kind": "generate", "preferred": ["claude-opus-4-8", "claude-sonnet-5", "gemini-3.1-pro"] }
```

- **`model`/`executor` still work exactly as before — `preferred` is entirely optional.** If set (non-empty), its first entry wins over `model`, which in turn still wins over `executor`.
- **Automatic failover is supported only when `preferred` names two or more models.** A Step with zero or one `preferred` entries behaves exactly as before this feature existed — a single resolve-and-execute attempt, no retry of any kind.
- **A retryable failure tries the next entry.** If the first entry's model call fails with a classified, retryable reason — rate limit, temporary unavailability, or timeout — Foundry resolves and tries the next `preferred` entry instead, logging the switch (e.g. "Claude Sonnet unavailable. Switching to Gemini 3.1 Pro."). This repeats down the list until one succeeds or the list is exhausted.
- **Some failures never fail over, by design** — authentication failures, an invalid/unrecognized model ID, and a model rejecting something it doesn't support (unsupported capability) always fail the Step immediately, even with more `preferred` entries left to try. A different model wouldn't fix a bad credential or a typo'd model name, and trying one anyway would hide the real problem.
- **There is still no availability *probing*** — nothing checks whether `claude-opus-4-8` is reachable before trying it; a failure is only ever detected by actually attempting the call and it failing. Nothing here consults a model's reported runtime health either (see the ADR's own Decision 9/10 split) — failover reacts only to a real, just-attempted failure.
- **This only actually helps once a real Executor's errors are classified.** `executor/openai` (and therefore the `"openai"`/`"openai-compatible"` vendors — Ollama, Groq, DeepSeek, GitHub Models, ...) now emits a classified `model.FailureError` for its documented 401/403 (authentication), 429 (rate limit), 404 (invalid model), 5xx (temporary unavailability), and context-deadline-exceeded (timeout) cases — confirmed against OpenAI's own documented error taxonomy before mapping it, not guessed. `executor/claude`, `executor/geminicli`, `executor/gemini`, and `executor/copilotcli` do not classify their own errors yet — failover remains dormant against those vendors until each gets its own equivalent mapping.
- **An empty `preferred: []` is treated as absent**, falling through to `model`/`executor` unaffected.

### Capability-aware model resolution

Per [ADR-0013](../03-adrs/ADR-0013-model-registry.md) (Proposed, seventh increment): a `generate` Step can name `require_capabilities` — a list of capability names every candidate selected from `model`/`preferred` must support:

```json
{
  "id": "implement",
  "kind": "generate",
  "preferred": ["claude-haiku-4-5", "claude-opus-4-8"],
  "require_capabilities": ["structured_output", "tool_use", "thinking"]
}
```

- **Only a candidate whose catalogued `Capabilities` support every named requirement may be selected — checked once, up front, before any execution.** In the example above, if `claude-haiku-4-5` doesn't support `thinking`, it is never even attempted; `claude-opus-4-8` (assuming it supports all three) is selected directly.
- **If no candidate qualifies, this fails immediately with a validation error — before any cost estimate, Budget charge, or model call happens at all.** Nothing is ever executed in that case.
- **Capability names match `model.Capabilities`' own field names**: `tool_use`, `thinking`, `streaming`, `multimodal`, `structured_output`. An unrecognized name is always treated as unsatisfied — never silently ignored — so a typo'd requirement fails clearly rather than passing vacuously.
- **Automatic failover (above) only ever moves between capability-verified candidates.** A `preferred` entry excluded for lacking a required capability is never tried, not even as a failover target.
- **`require_capabilities` has no effect on a Step naming only `executor` (or nothing at all).** Capabilities live on `model.Info`, keyed by Model ID — a plain Executor pin has no associated Model ID to check them against, so resolution proceeds exactly as it always did for that kind of Step.
- **This is a narrow, static form of capability matching** — a fixed, hand-curated catalog checked once against a fixed list, not a dynamic negotiation protocol, not a scoring/ranking policy, and not connected to `HealthManager`'s runtime health state. See the ADR's own Context for how this relates to [ADR-0006](../03-adrs/ADR-0006-routing-and-policy.md)'s previously-deferred capability-based routing.

## What's shipped, and why each is shaped the way it is

Two Pipelines are built into the Engine itself (`engine/pipelines/`, embedded by `engine.BuiltinPipelineSource`); four more are this repository's own project-level Pipelines (`.foundry/pipelines/`, loaded by `project.FilesystemPipelineSource` alongside the built-ins). All six are real, decodable, tested documents — not illustrations.

- **`default`** (built-in) — `generate → verify`, one bounded repair, no `target` (there is only one Step to restart from). This is the Engine's original hardcoded lifecycle, preserved byte-for-byte as the trivial path a caller who never asks for a different Pipeline still gets (RFC-0002 §9 Phase 3's compatibility requirement).
- **`review`** (built-in) — `generate → verify → verify-again`, no repair. Two independent verify Steps checking different things against the same Outcome (e.g. lint, then security); the second's verdict is what counts, and neither retries.
- **`feature`** (`.foundry/pipelines/feature.json`) — the full lifecycle: `plan → approve-plan → implement → verify → approve-outcome → apply → record`, repair bounded at 2 attempts, targeting `implement`. A feature is the case RFC-0002 §4.3 built this vocabulary for: agreeing on a plan before spending implementation effort, then a second approval gate over the verified diff before it ever touches the repository.
- **`bugfix`** (`.foundry/pipelines/bugfix.json`) — `implement → verify → approve → apply → record`, repair bounded at 1 attempt, targeting `implement`. No separate plan/approve-plan stage: a bugfix's scope is normally already known, so the ceremony a feature needs to agree on direction first would just be friction here. Its `implement` Step pins `"executor": "local-llama"` (`.foundry/executors.json` maps that name to a local Ollama instance via the `openai-compatible` vendor) — a deliberate choice to trade some capability for a fully free, private path on lower-stakes work, unlike `feature`/`release`/`issue` below, which all keep the default (Claude Code) for their own implementation Step.
- **`release`** (`.foundry/pipelines/release.json`) — `prepare → verify → verify-checklist → approve → apply → record`, **no repair**. Two verify Steps (e.g. the build/test suite, then a release-checklist-style check — changelog, version bump), mirroring `review`'s independent-verify pattern. Repair is deliberately disabled: a release failing its checklist should stop and get a human's attention, not retry itself automatically.
- **`issue`** (`.foundry/pipelines/issue.json`) — `plan → approve-plan → implement → verify → approve-outcome → apply (target: remote-pr) → record`, repair bounded at 2 attempts, targeting `implement`. Backs `/issue <id>`, this repository's own richer starter (see [getting-started.md](getting-started.md)'s "one model per part of the loop" section). Its `plan` Step pins `"executor": "planner"` (`.foundry/executors.json` maps that name to the Gemini CLI vendor) — a cheaper, faster model for the initial analysis a human still approves, while `implement` keeps the default (Claude Code) for the higher-stakes code-generation work itself.

Between `bugfix.json` and `issue.json`, three of `executor`'s four named vendors in this repository's own `.foundry/executors.json`/`.foundry/config.json` are actually exercised by a real Pipeline document (Gemini for planning, Ollama for a bugfix's implementation); the fourth, GitHub Models' GPT free tier, reviews every Act's diff as the supplementary AI-review Verifier layer (`ai_review_model`, not a Step pin — see [getting-started.md](getting-started.md)) rather than through the `executor` field at all, since it's a Verifier, not an Executor.

## Authoring your own

Add a `*.json` document to `.foundry/pipelines/` (create one with `/init` if the directory doesn't exist yet — it scaffolds simple starters you're free to edit). `project.FilesystemPipelineSource` loads every `*.json` file in that directory alongside the built-ins; a name collision with a built-in is a registration error, never silently resolved.
