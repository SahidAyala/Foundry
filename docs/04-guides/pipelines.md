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

- **`steps`** — an ordered list. Each Step has an `id` (unique within the document, doubles as the human-readable name a `repair.target` can point back to) and a `kind`, one of RFC-0002 §4.2's closed five: `generate`, `verify`, `approve`, `apply`, `record`. A Step Kind PipelineStrategy does not recognize is a decode-time error, never a silently skipped Step. A Step may also carry `capability` (object), `executor` (string), `feeds_forward` (bool), and `target` (string) — all optional, reserved for the Router (see [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md)) and unused by any document above.
- **`repair.max_attempts`** — how many times the Pipeline may re-run after a `verify` Step's Judgment is `fail`. `0` (or an omitted `repair` block) means no repair.
- **`repair.target`** — the Step ID a repair round jumps back to, re-running only from there onward, not the whole Pipeline. Omitted means "restart from the first Step."

A failing `verify` Step always stops the current attempt before any `approve`, `apply`, or `record` Step — a Pipeline never seeks approval for, applies, or records an Outcome its own verification just rejected, whether or not that attempt goes on to repair.

### Field evolution and unknown fields

Per [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md): the fields listed above are the complete schema — there is no document schema-version field yet, and any field not named here is a decode-time error, not a silently ignored key. If you see an error like `unknown field "capabilty"`, it means exactly that: the document has a field this schema doesn't recognize, most often a typo of one of the names above. Fix the field name (or remove it) and re-run. New optional fields, when added, are always additive and `omitempty` — a document written before a new field existed keeps decoding identically once the field is documented here.

## What's shipped, and why each is shaped the way it is

Two Pipelines are built into the Engine itself (`engine/pipelines/`, embedded by `engine.BuiltinPipelineSource`); three more are this repository's own project-level Pipelines (`.foundry/pipelines/`, loaded by `project.FilesystemPipelineSource` alongside the built-ins). All five are real, decodable, tested documents — not illustrations.

- **`default`** (built-in) — `generate → verify`, one bounded repair, no `target` (there is only one Step to restart from). This is the Engine's original hardcoded lifecycle, preserved byte-for-byte as the trivial path a caller who never asks for a different Pipeline still gets (RFC-0002 §9 Phase 3's compatibility requirement).
- **`review`** (built-in) — `generate → verify → verify-again`, no repair. Two independent verify Steps checking different things against the same Outcome (e.g. lint, then security); the second's verdict is what counts, and neither retries.
- **`feature`** (`.foundry/pipelines/feature.json`) — the full lifecycle: `plan → approve-plan → implement → verify → approve-outcome → apply → record`, repair bounded at 2 attempts, targeting `implement`. A feature is the case RFC-0002 §4.3 built this vocabulary for: agreeing on a plan before spending implementation effort, then a second approval gate over the verified diff before it ever touches the repository.
- **`bugfix`** (`.foundry/pipelines/bugfix.json`) — `implement → verify → approve → apply → record`, repair bounded at 1 attempt, targeting `implement`. No separate plan/approve-plan stage: a bugfix's scope is normally already known, so the ceremony a feature needs to agree on direction first would just be friction here.
- **`release`** (`.foundry/pipelines/release.json`) — `prepare → verify → verify-checklist → approve → apply → record`, **no repair**. Two verify Steps (e.g. the build/test suite, then a release-checklist-style check — changelog, version bump), mirroring `review`'s independent-verify pattern. Repair is deliberately disabled: a release failing its checklist should stop and get a human's attention, not retry itself automatically.

## Authoring your own

Add a `*.json` document to `.foundry/pipelines/` (create one with `/init` if the directory doesn't exist yet — it scaffolds simple starters you're free to edit). `project.FilesystemPipelineSource` loads every `*.json` file in that directory alongside the built-ins; a name collision with a built-in is a registration error, never silently resolved.
