# RFC-0003 — From a Flag-Based CLI to an Interactive, Project-Configured, Multi-Executor Assistant

| | |
|---|---|
| **Status** | Draft — Proposed (seeking ratification; a governance process now exists — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) — but this RFC has not itself been individually ratified through it) |
| **Authors** | Principal architect review (AI-assisted), for Foundry Core |
| **Reviewers** | _(pending)_ |
| **Supersedes** | — |
| **Superseded by** | — |
| **Created** | 2026-07-09 |
| **Related** | [RFC-0002](RFC-0002-pipeline-execution-runtime.md) (§4.4, §7, §8 — this RFC elaborates, does not contradict, those sections), [execution.md](../02-architecture/execution.md), [extensibility.md](../02-architecture/extensibility.md), [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md), [OQ-005](../06-open-questions/OQ-005-extension-isolation.md), [terminology.md](../05-reference/terminology.md) |

> **What this document is.** A product-shape decision, requested directly: Foundry's primary interface should be an interactive, slash-command-driven assistant resident in a project (`/init`, then e.g. `/feature "<intent>"`) rather than a one-shot, flag-parsed shell command. This RFC validates today's actual behavior against that intent, separates what RFC-0002 already anticipated from what is genuinely new, and proposes a concrete design for the three pillars the request implies — an interactive session, project-scoped Pipeline authoring, and multi-Executor routing — without implementing any of it.
>
> **Maturity discipline.** PROVISIONAL and non-canonical, exactly as RFC-0002 is treated. It does not resolve OQ-002 or OQ-005, and it does not silently ratify any ADR backlog item — see §6.

---

## 0. Executive summary

The request was to validate, then decide: Foundry should not be `foundry do "<intent>" --repo <path>`, a flag-parsed one-shot subcommand. It should behave like Claude Code, opencode, or aider — a persistent interactive session in a project directory, where `/init` scaffolds project-local configuration and Pipeline customization, and a command like `/feature "implementa refresh tokens con JWT"` runs a project-defined Pipeline that routes different Steps to different named models (e.g. Opus/Fable plans, Sonnet implements, Haiku reviews and writes tests, Copilot reviews the code, Sonnet resolves the review, Haiku commits and opens a PR).

**Validated (§1):** today's `foundry` is exactly the flag-based, one-shot tool the request says it should not be. This was confirmed by running the built binary, not by reading source.

**Not a contradiction, but a large gap (§2):** the interactive-session idea is already sketched, undeveloped, in [RFC-0002 §8](RFC-0002-pipeline-execution-runtime.md#8-human-interaction--interactive-terminal-ux) as Phase 8; multi-Executor routing is already sketched, undeveloped, in RFC-0002 §4.4/§7 as Phases 6–7. Nothing here overturns those sections. Two pieces of the request have **no existing proposal anywhere**: project-local Pipeline authoring via a `/init`-style command (§3.2), and committing + opening a pull request as an automated step (§4.1) — the latter is a genuine new trust-boundary question, not a mechanical extension.

**Recommendation (§7):** treat this as three separable, sequenced proposals layered on RFC-0002's existing phases, gated by two ADRs already in the backlog and one this RFC proposes adding.

---

## 1. Current state — validated by running the binary

```
$ foundry
Usage: foundry <command> [arguments]
Commands:
  do    Run the Act lifecycle for an Intent against a repository
  log   List recorded Acts for a repository
  show  Show one recorded Act in full

$ foundry /init
foundry: unknown command "/init"
```

- `cmd/foundry/main.go` dispatches on `args[0]` via a literal `switch` (`"do"`, `"log"`, `"show"`, `"help"`) and exits after one action. There is no loop reading further input, no REPL, no notion of "session."
- `cli.ParseArgs` (`cli/cli.go`) parses `--repo` and one positional intent; this is a flag CLI, not a command language.
- Exactly one Pipeline runs per invocation, selected by a hardcoded literal in `cmd/foundry/commands/do.go` (`const pipelineName = "default"`) — there is no `--pipeline` flag and, more relevantly to this RFC, no way today for a *project* to declare its own Pipeline at all. `BuiltinProvider` only ever reads documents embedded into the binary at compile time (`engine/pipelines/*.json`); nothing reads from a project directory at runtime.
- Exactly one `Executor` exists per process, injected once at the composition root (`claude.NewClaudeExecutor`). There is no per-Step Executor selection of any kind.

This matches [roadmap.md](../00-overview/roadmap.md)'s own milestone framing: the codebase is at M0/early-M2 ("reusable production + mock executor"); M3 ("real executors... behind capability routing") has not started.

---

## 2. What the request already has a home for, vs. what is genuinely new

| Piece of the request | Status |
|---|---|
| Persistent interactive session, natural-language input starts an Act | Sketched in [RFC-0002 §8](RFC-0002-pipeline-execution-runtime.md#8-human-interaction--interactive-terminal-ux) as **Phase 8** — not built. |
| Slash commands (`/status`, `/history`, `/pipeline`, `/approve`) | Same — RFC-0002 §8 names four; the request's `/init` and `/feature` are new instances of the same idea, not a new mechanism. |
| Different Steps routed to different named models (Opus/Fable → Sonnet → Haiku → Copilot → Sonnet → Haiku) | Sketched in RFC-0002 §4.4 (Capability declaration) and §7 (Router, three layers, explicit-pin-first) as **Phases 6–7** — not built. [roadmap.md](../00-overview/roadmap.md) names this M3. |
| `/init` scaffolding **project-local** Pipeline/config files | **No proposal exists anywhere.** RFC-0002 only says a document is "a `[]byte` a caller already has" and gestures at "a future filesystem or remote `PipelineProvider`" without specifying one. |
| A Step that commits and opens a pull request | **No proposal exists anywhere.** RFC-0002's `Apply` Step kind (§4.2) is "an accepted Outcome is applied to Project State (code via Workspace...)" — today's only implementation, `workspace.Land`, never leaves the developer's local git checkout. Pushing to a remote and opening a PR is a different, larger action. |

The first two rows are "build the thing already designed, in order." The last two need design work this RFC does (§3.2, §4.1).

---

## 3. Target architecture — three pillars

### 3.1 Interactive session & slash commands

No new design needed beyond what RFC-0002 §8 already states; this section only makes it concrete enough to sequence.

- `foundry` invoked with no subcommand starts a session in the current directory (today's `foundry do "<intent>" --repo <path>` remains available unchanged for scripting/CI — RFC-0002 §8's explicit requirement, and [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md)'s R2 non-Go extension boundary is unaffected either way).
- A line of input is either a slash command (`/init`, `/feature ...`, `/status`, `/pipeline`, `/approve`, `/reject`) or free text, which becomes an **Intent** for the project's default (or last-selected) Pipeline — exactly RFC-0002 §8's rule that the NL layer never makes control-flow decisions; it only starts an Act.
- `Reporter` (already a pure observer port, engine/reporter.go) needs `StepStarted`/`StepFinished` events in place of today's fixed `Executing`/`Verifying` pair — RFC-0002 §8 already names this exact change.
- The trust boundary does not get thinner: an `Approve` Step (RFC-0002 Phase 4, not yet built) still blocks on an explicit human decision whether triggered by `/approve` or free text — session ergonomics change, not I5.

### 3.2 Project-scoped Pipeline authoring (`/init`) — new design

Today's `PipelineProvider` interface (`engine/provider.go`) already generalizes over *where* a Pipeline document comes from — `BuiltinProvider` is one implementation; nothing about the interface assumes "compiled into the binary." This is confirmed structurally, not just claimed: two independent validation passes on this codebase already proved a second `PipelineProvider`-compatible document can be added with zero `Engine`/`Strategy`/`PipelineRegistry` changes. The same holds for a filesystem-backed provider.

**Proposed shape** (design only — not built):

- `/init` writes a project-local directory (e.g. `.foundry/pipelines/*.json`, mirroring `engine/pipelines/*.json`'s own document shape — `PipelineDocument` from `engine/document.go`, unchanged) — scaffolding, not a new schema. It also writes one starter Pipeline document a user is expected to edit, analogous to how `git init` writes a starter config, not a working policy.
- A new `FilesystemPipelineProvider` (a second, sibling implementation of `PipelineProvider`, living beside `BuiltinProvider` in `engine/`) reads `.foundry/pipelines/*.json` at session start and calls the exact same `DecodePipelineDocument` `BuiltinProvider` already calls. No new decoder, no new validation rules, no Engine change — this is the same seam the RFC-0002 Phase-3 validation work already exercised twice.
- Composition order: built-in Pipelines register first (as today), then project-local Pipelines register on top via the same `PipelineRegistry.RegisterMany`; a name collision between a built-in and a project-local Pipeline is a `Register` error today (by design — RFC-0002's registry already refuses silent overwrite) and should surface to the user as "this project's Pipeline shadows/conflicts with a built-in," a decision, not a silent resolution either way.
- What `/init` does **not** do: discover Pipelines by walking the filesystem, support a plugin format, or introduce a configuration framework beyond one directory of one already-defined document shape. This mirrors the constraints the two prior validation passes were explicitly built under, and there is no reason to relax them just because the reader is now a directory instead of `go:embed`.

This closes the one gap in the request that is *mechanically* simple: authoring location. The remaining gap (§3.3, §4.1) is not mechanically simple.

### 3.3 Multi-Executor routing — elaborating RFC-0002 §4.4/§7 against a concrete example

RFC-0002 already proposes the mechanism (Capability declaration + Router, explicit-pin-first). What it does not yet do is check that mechanism against a *real* six-role lifecycle. Doing that check surfaces a real gap RFC-0002's abstract description didn't:

| Requested role | Nearest existing Step kind (RFC-0002 §4.2) | Fit |
|---|---|---|
| Opus/Fable generates the architecture doc | `Generate` (Capability `role: plan`) | Clean fit. |
| Sonnet implements | `Generate` (Capability `role: implement`) | Clean fit — this is RFC-0002's own worked example. |
| Haiku reviews tests **and writes new tests** | ⚠️ Neither `Verify` nor `Generate` alone | **Gap** — see below. |
| Copilot does code review | `Verify` (Capability `role: review`), if review output is modeled as Validator-style findings | Fit, with a caveat: today's `Verify` (`verify.Gate`) yields a boolean-ish pass/fail/repair Judgment fragment over deterministic Validators; a model-backed qualitative code review producing prose findings is a different *kind* of Validator than any that exists today, but the port itself (`Verifier`) does not need to change — RFC-0002 §5's error-vs-finding rule already generalizes to this. |
| Sonnet resolves the code review | `Generate` (Capability `role: implement`), fed the review's findings as Context | Clean fit — mechanically identical to today's existing repair-context mechanism (`repairContext` in `engine/strategy.go`), which already injects a failed Verify's findings into the next Generate's Context. |
| Haiku commits and opens a PR | ⚠️ Not `Apply` as currently defined | **Gap** — see §4.1. |

**The "reviews tests and writes new tests" gap:** this is one Step description covering two different kinds of work — a `Verify`-shaped judgment over existing tests, and a `Generate`-shaped artifact (new test files) produced as a *consequence* of that judgment. RFC-0002's closed Step-kind vocabulary (§5: "a closed set, extensible only by adding a new kind deliberately") does not have a kind for this today. Two ways to resolve it, both fitting inside the existing restricted-list-plus-repair-edges shape (RFC-0002 §5's tradeoff discussion), neither requiring a general DAG:
1. **Two Steps, not one**: `Verify` (Haiku reviews existing tests → Judgment) immediately followed by `Generate` (Haiku, fed that Judgment's findings, writes new tests) — the same Context-injection mechanism §3.3 already relies on for "Sonnet resolves the code review." This requires no new Step kind at all.
2. **A new Step kind**, e.g. a review-that-also-produces-Artifacts kind — more expressive, but reopens RFC-0002 §10's named risk ("over-generalizing... before a real use case needs it").

This RFC recommends (1): it needs zero new Step-kind vocabulary and is provable with the exact testing pattern the last two validation passes already established (a Pipeline document is just a longer, still-linear list of `generate`/`verify` Steps).

---

## 4. What remains genuinely unresolved

### 4.1 Committing and opening a pull request is a new trust boundary, not a new Step kind

This is the request's highest-risk piece, and this RFC deliberately does not resolve it — it names the shape of the decision instead.

Today, `Apply` means exactly one thing: `workspace.Land` fast-forwards the developer's own branch with a patch that a human already approved via `PromptForApproval`, entirely on the developer's own machine, touching nothing remote. "Commit and open a PR" is a different action along three axes at once:
- **It leaves the machine.** Every prior Apply is local; pushing a branch and calling a VCS host's API is Foundry's first outbound write to shared infrastructure.
- **It changes what "Authority" means (I5).** Today, one human's `y`/`yes` at a blocking prompt is the entire accountability story for an Act. If a PR opens automatically after that same prompt, is the PR itself now *also* a review surface — meaning the human who approved the Act and the human(s) who review the PR are allowed to differ? RFC-0002 §4.2 already has an `Approve` Step kind (Phase 4, unbuilt) that could model "PR review" as a *second*, later `Approve` Step in the same Pipeline — but that is a design choice this RFC flags, not one it makes.
- **It needs credentials Foundry does not manage today.** Nothing in `domain`, `engine`, or `workspace` currently models an external service credential; this is new surface, adjacent to but distinct from [OQ-005](../06-open-questions/OQ-005-extension-isolation.md)'s "third-party extension" isolation question (a VCS host is not a Foundry extension, but the trust question — untrusted-until-verified, default-deny — rhymes).

This RFC's recommendation is narrow: **do not build this by extending `Apply`'s existing meaning.** Model it as a distinct `Apply` *target* (local branch vs. remote PR) decided per-Pipeline, exactly the way `Generate`/`Verify` Steps already declare Capabilities — but the actual accountability model (does opening a PR require its own `Approve` Step, does the Authority who approves the Act need to be the same Authority whose credentials open the PR) is a decision for the dedicated ADR this RFC proposes adding to the backlog (§6), not something to default into silently.

### 4.2 The slash-command-to-Pipeline binding contract

`/feature "<intent>"` implies a naming/binding convention: which slash commands map to which project-configured Pipeline, and whether a project can define its own slash commands (`/feature` is not one of RFC-0002 §8's four named examples). This is a real design surface but a small one — it composes directly from §3.1 (the session reads a line, decides slash-command vs. free text) and §3.2 (Pipelines are now named, project-local data) without needing new Engine concepts. Left for the implementation phase of §3.1/§3.2, not a blocker.

---

## 5. A pre-existing terminology note (found during this audit, not introduced by it)

[terminology.md](../05-reference/terminology.md) retires **Provider** as live vocabulary ("Replaced by Executor — model access is not a privileged execution path"). The already-shipped `PipelineProvider` (`engine/provider.go`, from RFC-0002 Phase 3+ work) uses "Provider" in an unrelated sense — Pipeline *document discovery*, never model access — so it is not the retired concept RFC-0002's own commit history predates this RFC. Still, per [AGENTS.md](../../AGENTS.md)'s historical-isolation guarantee #1, any live use of a retired word in an active document is worth a named decision, not a coincidence to leave unexamined. This RFC does not rename anything; it records the tension for whoever writes the "CLI & output contract" or terminology-governing ADR to close explicitly (rename `PipelineProvider`, or explicitly re-scope "Provider" as retired only in the model-access sense in `terminology.md`).

---

## 6. Backlog ADRs this RFC touches

Three ADRs already in the backlog ([ADR README](../03-adrs/README.md)) directly gate this RFC's pillars and must not be pre-decided here:
- **Reusable-Act template format & evolution policy** — now also owns the project-local (`/init`-authored) Pipeline document's schema and versioning, not only the built-in one.
- **Executor contract & capability model**, and **Routing & policy** — gate §3.3 exactly as RFC-0002 §7 already said.
- **CLI & output contract** — gates whether `foundry`'s primary interface is the interactive session or the flag CLI, and the slash-command surface's own stability promise.

This RFC proposes **one new backlog entry**, added to [ADR README](../03-adrs/README.md) alongside this RFC: **"VCS/PR integration & Apply targets"** (§4.1) — the accountability model for an automated commit + PR, and whether it requires crossing outside today's single-machine trust boundary. It is listed there as unresolved, not decided.

---

## 7. Risks

- **Cost.** A six-role Pipeline (Opus/Fable, Sonnet, Haiku, Copilot, Sonnet, Haiku) multiplies per-Act cost roughly sixfold over today's one-Executor lifecycle. [roadmap.md](../00-overview/roadmap.md) open decision #9 ("cost as a first-class constraint") is unresolved and directly load-bearing here — Budget (`engine/budget.go`) today charges one flat estimate per attempt regardless of Step count (RFC-0002 §3 limitation #10, still unresolved), which will underestimate a six-role Pipeline's real cost until per-Step accounting exists.
- **Sequencing.** RFC-0002 §10 already warns that building the Router (Phases 6–7) before Approve/Record-as-Steps (Phase 4) forces Capability plumbing through the flat `Act` shape twice. That risk applies unchanged here — §3.3 should not start before RFC-0002 Phase 4 lands.
- **Trust boundary creep.** §4.1's PR automation is the first Foundry action that is not reviewable-then-reversible entirely on the developer's own machine before anything external sees it. This must be treated as at least as strict as today's `Approve`, never a convenience shortcut around it.

---

## 8. Final recommendation

Adopt the product-shape decision — Foundry's primary interface becomes an interactive, slash-command-driven session (§3.1) with project-scoped Pipeline authoring via `/init` (§3.2) — as the target, sequenced strictly on top of RFC-0002's existing, already-numbered phases, not parallel to them:

1. **RFC-0002 Phase 4** (Approve/Record as declared Step kinds) — prerequisite for everything below; not yet started.
2. **§3.1 minimal interactive shell** — a REPL mapping to today's `do`/`log`/`show` primitives, no new slash semantics yet. Builds on Phase 4's CLI shrinkage (RFC-0002 §9 Phase 4: "`cli.CLI.Do` shrinks to parse args → resolve Pipeline → run Strategy → done").
3. **§3.2 `/init` + `FilesystemPipelineProvider`** — no Engine change; the exact seam already twice validated in this codebase.
4. **RFC-0002 Phase 6** (Router, explicit-pin-only policy) — the minimum needed to name Opus/Sonnet/Haiku/Copilot per Step; do not build Phase 7 (negotiation) until a real multi-Executor Pipeline in production motivates it, per RFC-0002's own depth-before-breadth discipline.
5. **§4.1 VCS/PR integration** — gated on its own dedicated ADR (§6); the highest-risk, least-specified piece, and should not be built opportunistically alongside 1–4.

A governance process now exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but this RFC has not yet been individually ratified through it. Until it is, treat it as RFC-0001 and RFC-0002 are treated: Draft, Proposed, argued with rather than deferred to.

---

_This RFC is meant to be argued with. Its falsifiable core claim: every piece of the requested interactive/multi-executor/PR-automation vision is expressible as (a) work already scheduled in RFC-0002 Phases 4–8, (b) one new `PipelineProvider` implementation requiring zero Engine changes, or (c) one explicitly-flagged new trust-boundary decision (§4.1) — with no other new Engine or domain concept required. If a piece of the vision is found that doesn't fit one of those three buckets, that is the trigger to extend this RFC, not to build around it silently._
