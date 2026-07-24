# ADR-0013 — Model Registry

| | |
|---|---|
| **Status** | **Proposed** — drafted 2026-07-24, not yet ratified. Per [ADR-0000](ADR-0000-governance-and-ratification-process.md), only the project's sole maintainer ratifies; this document does not self-ratify. |
| **Date** | Drafted 2026-07-24 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | A new, additive catalog abstraction — not an existing backlog row. |
| **Gates** | Nothing yet: this ADR introduces no new compatibility surface that anything else depends on. It exists so a *future*, separately-decided PR (e.g. a `foundry models` listing command, or capability-aware routing once [ADR-0006](ADR-0006-routing-and-policy.md)'s own named trigger fires) has somewhere to build from. |

---

## Context

**Today, "which model runs a Step" and "how that model is invoked" are the same concept.** A Pipeline Step's `executor` field (`engine.Router.Resolve`) names an *Executor* — `project.ExecutorConfig.Vendor` plus `Model` together decide both *how* (which vendor package: `executor/openai`, `executor/geminicli`, ...) and *what* (which model string that package is given) in one `.foundry/executors.json` entry. There is no separate place that answers "what models exist, independent of how they're invoked" — e.g. "Gemini 3.5 Flash is a Google model, reachable via the `gemini` Executor" as a fact on its own, distinct from a specific project's own configuration choice.

**This repository's own recent configuration work makes the gap concrete.** `.foundry/executors.json`/`.foundry/config.json` now name four vendors across four different models (Gemini for planning, Claude Code for implementation, GPT via GitHub Models for review, Ollama for bugfixes — see [getting-started.md](../04-guides/getting-started.md)), each chosen for a specific reason. That reasoning — which model is which vendor's, which provider it belongs to, what it's called for a human reading a config file — lives only in prose (this ADR, getting-started.md, code comments), not in any structure Foundry itself could query or display.

**The maintainer's ask, verbatim in spirit:** introduce a `ModelRegistry` abstraction — distinguishing *Executor* (how a model is invoked: `claude-code`, `gemini-cli`, `ollama`, ...) from *Model* (what is executed: `claude-sonnet`, `gemini-2.5-pro`, `gpt-5.5`, ...), with each Model belonging to exactly one Executor — **as an abstraction only, with explicit, repeated constraints that this PR change no existing behavior**: no pipeline-parsing changes, no execution-flow changes, no migration, existing tests and configuration must keep working exactly as today. The registry should "simply exist and be injectable."

**This is deliberately narrower than, and does not reopen, two already-declined decisions.** [ADR-0006](ADR-0006-routing-and-policy.md) declined capability-based routing/negotiation until a real trigger fires; [ADR-0008](ADR-0008-extension-isolation-and-contract-versioning.md) declined a third-party extension mechanism until a real third party asks. A catalog of "which model belongs to which Executor" is metadata, not a routing policy — `engine.Router.Resolve` still does exactly what it did before this ADR (a Step's `executor` pin, or the Engine's default, full stop). Nothing here lets a Step reference a *Model* instead of an *Executor*, or changes what a Step's `executor` field means. If a future PR wants to build capability-aware routing on top of this catalog, that remains its own decision, gated on its own trigger, exactly as ADR-0006 already requires.

## Decision

1. **New package `model`** (`model/registry.go`) defines `Info` (`ID`, `Executor`, `Provider`, `DisplayName`) and `Registry` (`NewRegistry`, `Register`, `Get`, `List`, `ByExecutor`) — a plain in-memory catalog, no persistence, no I/O, no dependency on any other Foundry package. `Register` refuses an empty ID, an empty Executor, or a duplicate ID (even under a different Executor) — each Model belongs to exactly one Executor, per the maintainer's own stated design.

2. **Executor packages that already have a natural, confident list of model IDs expose it via their own `SupportedModels() []model.Info`**: `executor/claude` (Anthropic's Claude models — informational only, since `executor/claude.Execute` has no model-selection parameter of its own today, unlike `executor/geminicli`/`executor/copilotcli`), `executor/geminicli` (Google's Gemini models, reusing `executor/gemini`'s own existing price-table model names — registered once, under the `"gemini"` Executor, per Decision 1's one-Executor-per-Model rule, not duplicated under `"gemini-api"`), `executor/openai` (both `"openai"`'s own hosted models and a couple of already-referenced `"openai-compatible"` examples: Ollama's `llama3`, GitHub Models' `openai/gpt-4.1` — this repository's own `.foundry/executors.json`/`.foundry/config.json`), and `executor/copilotcli` (a single informational `"copilot-default"` entry — no confirmed, documented list of selectable model IDs for the Copilot CLI's own `--model` flag exists to catalog honestly).

3. **`cmd/foundry`'s `buildModelRegistry()`** (`cmd/foundry/model_registry.go`) is the one place that calls every `SupportedModels()` and assembles one `*model.Registry` — mirroring why `cmd/foundry/main.go` is already the only place that imports every concrete Executor package (`project`/`cmd/foundry/commands` stay vendor-agnostic, per [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)'s existing seam). This function exists and is directly unit-tested (proving it is constructible and injectable), but **is not called from anywhere in the real startup path** — nothing consumes its result yet, satisfying "exists and is injectable" without any execution-flow change.

4. **No change to `engine.Router`, `project.ExecutorConfig`, `engine.DecodePipelineDocument`, or any Pipeline document's schema.** A Step's `executor` field still names an Executor exactly as before; nothing decodes or resolves a `model` field at the Step level. Every existing Pipeline document, `.foundry/executors.json`, and `.foundry/config.json` — including this repository's own, from the four-vendor configuration work immediately preceding this ADR — continues to work byte-for-byte unchanged.

## Consequences

- **Positive:** "what models exist, and which vendor runs them" is now a queryable fact independent of any one project's configuration, closing the prose-only gap this ADR's Context describes. A later PR (a `foundry models` listing command, richer `executors.json` validation against known IDs, or capability-aware routing once ADR-0006's trigger fires) has a real starting shape instead of inventing one from scratch.
- **Cost:** a new package and five new small files' worth of surface area to maintain; `SupportedModels()` lists are static and hand-curated (not fetched from any vendor's live model-listing API), so they will drift out of date as vendors ship new models — no different in kind from `executor/openai`'s/`executor/gemini`'s own existing hand-maintained price tables, which carry the identical staleness risk already.
- **Harder:** none identified — this is additive metadata with zero call sites in the execution path; reverting it entirely (deleting the `model` package and the five `SupportedModels()`/`model_registry.go` files) would have zero effect on any Pipeline, Step, Executor, or test outside this ADR's own new files.

## Migration Strategy

None — there is nothing to migrate. This PR adds new files only; it modifies no existing file's logic. `go build ./...`, `go vet ./...`, and `go test -race ./...` all pass unchanged for every pre-existing package.

## Review Checklist

- [ ] Maintainer confirms the `Info{ID, Executor, Provider, DisplayName}` shape and the "each Model belongs to exactly one Executor" rule match the intended abstraction.
- [ ] Maintainer confirms this should remain unratified/Proposed until a concrete consumer (the "later PR" named in Consequences) is actually decided, or confirms ratifying now regardless — consistent with how ADR-0012 was ratified the same day it was drafted, once the maintainer confirmed directly.
