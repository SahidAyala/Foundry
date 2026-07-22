# RFC-0004 — Multi-Executor Router, Publish Policy, and Lightweight Knowledge Capture

| | |
|---|---|
| **Status** | Draft — Proposed (seeking ratification; a governance process now exists — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) — but this RFC has not itself been individually ratified through it. Two of the ADRs it proposed shapes for, [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) and [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md), are now separately Accepted — that does not ratify this RFC itself) |
| **Authors** | Principal architect review (AI-assisted), for Foundry Core |
| **Reviewers** | _(pending)_ |
| **Supersedes** | — |
| **Superseded by** | — |
| **Created** | 2026-07-14 |
| **Related** | [RFC-0002](RFC-0002-pipeline-execution-runtime.md) §4.4, §7, §9 Phase 6–7 (this RFC makes Phase 6 concrete; it does not reopen it), [RFC-0003](RFC-0003-interactive-assistant-and-multi-executor-pipelines.md) §3.3, §4.1, §4.2 (this RFC resolves the two gaps RFC-0003 deliberately left open), [execution.md](../02-architecture/execution.md), [extensibility.md](../02-architecture/extensibility.md), [terminology.md](../05-reference/terminology.md), [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md), [OQ-005](../06-open-questions/OQ-005-extension-isolation.md), [roadmap.md](../00-overview/roadmap.md) open decision 9 |

> **What this document is.** A concrete design for a specific worked example, requested directly: a seven-role Pipeline — plan → implement → check requirements → code review → resolve review → write tests → document (twice) → commit and open a PR — spanning **real, distinct model vendors**, not only distinct models from one vendor, with a **project-level, enforced policy** for whether publishing to a remote PR requires a human approval. This validates RFC-0002 §4.4/§7's Router design against that example, resolves RFC-0003 §4.1's deliberately-left-open publish-trust-boundary question with the specific policy shape requested, and names one gap neither prior RFC surfaced: today's Context-threading mechanism does not carry one Step's output forward into the next Step outside the repair loop.
>
> **Maturity discipline.** PROVISIONAL and non-canonical, exactly as RFC-0002 and RFC-0003 are treated. It does not resolve OQ-002 or OQ-005, and it does not silently ratify any ADR backlog item — see §5.

---

## 0. Executive summary

The request: a Pipeline where **A** plans, **B** implements, **C** checks the requirements are met, **D** code-reviews, **B** resolves the review's comments, **E** reviews and writes tests, **F** writes two documents (one as Foundry's own memory, one as project documentation), and **G** commits and opens a pull request — with **A** through **G** genuinely different model vendors (not, e.g., Opus/Sonnet/Haiku all via Claude Code), and a project-wide switch deciding whether **G**'s publish step always waits for a human or always goes straight to a PR.

**Confirmed still true (§1):** `engine.Step` carries only `{ID, Kind}` — no Capability, no executor pin. Exactly one Executor exists, hardcoded at the composition root. No Router. No VCS integration. No Knowledge capture of any kind. Nothing here has changed since RFC-0003 was written five days ago.

**What this RFC resolves that RFC-0002/RFC-0003 left open:**
1. RFC-0002 Phase 6 (§4.4/§7) already designed Capability declaration + explicit-pin Router. §2.1–§2.3 below make it concrete: the Pipeline-document schema addition, a new Executor-configuration document (sibling to a Pipeline document, not a Pipeline field), and what "one normalized contract across vendors" has to mean once a vendor is not a subprocess CLI the way Claude Code is.
2. RFC-0003 §4.1 named "commit and open a PR" as a new trust boundary and deliberately did not decide its accountability model. §2.5 below proposes the specific shape requested: a project-level policy, enforced by the Engine at Pipeline-registration time, not left to a Pipeline author's discretion.
3. **New finding, not previously named:** RFC-0003 §3.3 claimed "Sonnet resolves the code review... is mechanically identical to today's existing repair-context mechanism." That mechanism (`repairContext` in `engine/strategy.go`) only fires on a *repair* re-run (`attempt > 0`); a straight-line forward Pipeline — Step 3 reviews requirements, Step 4 code-reviews, Step 5 resolves — never threads Step 4's findings into Step 5's Context today. §2.4 below closes this gap.
4. §2.6 gives the worked example's documentation role (**F**) a minimal, honest shape — two writes, no retrieval, explicitly not M4's Knowledge model — so it does not have to wait for that milestone.

**Recommendation (§6):** six sequenced pieces, each independently shippable, none requiring the others to exist first except where noted.

---

## 1. Current state — what §0's claims rest on

- `engine/step.go`: `type Step struct { ID string; Kind string }`. Its own doc comment already names the gap: *"Capabilities, model hints, or routing metadata... belong to the Router, which does not exist until Phase 6."*
- `cmd/foundry/main.go` / `cmd/foundry/commands/do.go` (`wireEngine`): exactly one `Executor` is constructed per invocation, `claude.NewClaudeExecutor`, injected at the composition root. Nothing in `engine`, `cli`, `session`, or `project` selects between Executors.
- `engine/strategy.go`'s `runSteps`: a Generate Step's Context is `considered`, set once per attempt from the Gatherer's output and mutated only by `repairContext` when `attempt > 0` (a repair re-run). A Verify Step's `Checked` findings are recorded onto the Act's trace (`recordStep`) but never fed into a later Step's `considered` in the same forward pass.
- `workspace.GitApplier` / `workspace.Land`: `Apply` fast-forwards the developer's own local branch. Nothing pushes a remote, calls a VCS host API, or manages a host credential.
- No package resembling "Knowledge" exists (confirmed by `find . -iname '*knowledge*'` returning nothing outside docs). `gatherer.NaiveGatherer`'s own doc comment: *"Knowledge-based context... is a distinct, unbuilt concern — M4."*

---

## 2. Target design

### 2.1 Capability on a Step (Pipeline-document schema, additive)

A `Step` in a `PipelineDocument` gains two optional fields, mirroring RFC-0002 §4.4 exactly:

```json
{ "id": "implement", "kind": "generate", "capability": { "role": "implement" }, "executor": "sonnet-main" }
```

- `capability` — free-form role tag (`plan`, `implement`, `review`, `test`, `document`, ...). Advisory metadata only in Phase 6; nothing negotiates on it yet (that is Phase 7, RFC-0002 §7 layer 2, explicitly not built here).
- `executor` — the **only** routing policy this RFC builds: an explicit pin to a name resolved against the Executor configuration below. Absent `executor` means "use the Pipeline's (or process's) default Executor" — every existing Pipeline document (`default.json`, `review.json`, `feature.json`, `bugfix.json`, `release.json`) decodes and runs identically with zero edits, because the field is optional and today's single-Executor behavior is exactly what "no pin" already means.

`engine.Step` gains matching optional fields; `DecodePipelineDocument` gains this as new, backward-compatible decoding — no existing document's meaning changes.

### 2.2 Executor configuration — a new document, not a Pipeline field

A Pipeline names *which role* runs a Step (`executor: "sonnet-main"`); something else has to say *what `sonnet-main` actually is* (vendor, model, credential source). This is new project-local data, analogous to how `.foundry/pipelines/*.json` holds Pipeline documents:

- `.foundry/executors.json` — a flat map: `{"sonnet-main": {"vendor": "claude", "model": "claude-sonnet-5"}, "gpt-implementer": {"vendor": "openai", "model": "gpt-5.1", "api_key_env": "OPENAI_API_KEY"}}`. Credentials are named by *environment variable*, never stored in the document itself — the same posture the project already takes (Claude Code today reads its own credentials from its own environment, unmanaged by Foundry).
- A new `engine.ExecutorRegistry` (naming deliberately avoids "Provider" — [terminology.md](../05-reference/terminology.md) retires that word for anything touching model access; see RFC-0003 §5's note on the pre-existing `PipelineProvider` tension, which this RFC does not add to) resolves a pinned name to a constructed `engine.Executor`, mirroring `PipelineRegistry`'s register-once, look-up-by-name shape exactly.
- A new `engine.Router` (the Phase 6 piece RFC-0002 named): given a Step's `executor` pin (or the process default when absent), returns the `Executor` to call. Its *entire* policy in this RFC is "explicit pin, or default" — RFC-0002 §7's layer 1, nothing more.

### 2.3 The Executor contract across vendors — what actually needs deciding

`engine.Executor.Execute(ctx, intent, considered) (*domain.Outcome, error)` **does not change**. That port is already vendor-agnostic; adding a vendor is "write an adapter," not "redesign the Engine" — RFC-0003 §3.3 already established this for the port itself, and this RFC does not reopen it.

What genuinely differs across vendors, and is not decided anywhere today:
- **Invocation shape.** `executor/claude.ClaudeExecutor` shells out to the Claude Code CLI (a subprocess, matching ADR-0001's non-Go extension boundary). A GPT-backed Executor is a pure API call — no subprocess, no local binary dependency, different failure modes (HTTP errors/rate limits vs. process exit codes).
- **Cost/telemetry.** `engine.budget.go`'s flat per-attempt estimate assumes roughly one shape of cost; a real multi-vendor Pipeline needs each Executor to report (or a config to state) its own per-call cost estimate, not one constant reused everywhere — feeding §2.7 below.
- **Capability truthfulness.** Nothing today stops an Executor config from claiming a `role` it cannot actually perform; Phase 6's explicit-pin-only policy makes this the human's mistake to catch, not the Router's — deliberately, so no premature negotiation logic has to exist before it is needed (RFC-0002 §7 layer 2).

**On "Copilot" specifically:** GitHub Copilot does not expose a clean, headless, arbitrary-code-generation agent surface the way Claude Code or a model API does today — its most accessible non-IDE surface is narrower (PR review comments, `gh copilot suggest`). This RFC recommends proving the multi-vendor mechanism with Claude (multiple model configs) plus **one** additional vendor reachable by a clean API (e.g. an OpenAI-backed `Executor`) before attempting a literal Copilot adapter, and treating "Copilot does code review" in the worked example as satisfied by *whichever* Executor is configured for that `role: review` pin — a literal GitHub Copilot adapter, if wanted later, is its own smaller follow-up once its actual available surface is scoped, not a blocker to the mechanism this RFC builds. This is flagged as a decision for whoever reviews this RFC, not settled unilaterally.

This whole section feeds, and must not preempt, the ADR backlog's still-unwritten **"Executor contract & capability model"** entry.

### 2.4 Step-to-Step Context threading — the gap neither prior RFC named

Today, a Generate Step's Context (`considered`) is fixed for the whole attempt; only a *repair* re-run appends anything to it (`repairContext`, gated on `attempt > 0`). A straight-line Pipeline — `check-requirements` (Verify) → `code-review` (Verify) → `resolve-review` (Generate) — has no mechanism today to feed `code-review`'s findings into `resolve-review`'s Context. RFC-0003 §3.3's claim that this "is mechanically identical to today's existing repair-context mechanism" is not accurate for a forward pass; it only holds for an actual bounded-repair jump.

**Proposed fix, additive and opt-in (so no existing Pipeline's behavior changes):** a Step gains an optional `feeds_forward: true` flag. When set, the immediately-preceding Step's output — a Verify Step's `Checked` findings, or a Generate Step's `Produced` patch — is appended to the following Step's Context exactly once, using the same rendering `repairContext` already uses for findings. Absent the flag (every Pipeline shipped today), behavior is byte-for-byte unchanged. This is the smallest change that makes **D → B** ("code review feeds the fix") and **C → E** ("the review of existing tests feeds the writing of new ones," RFC-0003 §3.3's own recommended two-Step pattern) actually work, without touching the repair loop, the Budget tracker, or any existing Pipeline document.

### 2.5 Publish policy — the trust-boundary decision RFC-0003 §4.1 deliberately left open

The user's own answer to this RFC's prerequisite question: a **project-level, enforced** switch, not a per-Pipeline courtesy.

- `Apply`'s meaning is not extended (per RFC-0003 §4.1's explicit recommendation). A Step declares an `apply` Step's **target**: `"local"` (today's only behavior, the default — every existing Pipeline is unaffected) or `"remote-pr"`.
- A new project-level file, `.foundry/config.json`, holds one relevant field for this RFC: `{"require_approval_before_remote_publish": true}`.
- **Enforcement point:** when a Pipeline is registered (loaded via `PipelineRegistry.Register`/`RegisterMany`), if it declares an `apply` Step with `target: "remote-pr"` and the project's config requires approval, the Pipeline must also declare an `approve` Step somewhere before it — checked at registration, not at run time, so a misconfigured Pipeline is a load-time error a human sees immediately, never a silent bypass discovered after the fact. `false` (or the field's absence) means a Pipeline may publish directly if its author chose not to declare an `approve` Step — the project has explicitly opted out of requiring one.
- **Mechanism:** a new `apply`-target implementation (not `workspace.GitApplier`) pushes a branch and opens a PR by shelling out to the `gh` CLI — the same "subprocess, not an embedded API client" posture `ClaudeExecutor` and `git apply` already establish, keeping this consistent with ADR-0001's existing extension-boundary reasoning rather than introducing a new integration style. Host credentials are read from an environment variable named in `.foundry/config.json`, mirroring §2.2's Executor credential pattern — never stored by Foundry.
- This still needs, and this RFC does not substitute for, the ADR backlog's **"VCS/PR integration & Apply targets"** entry (added to the backlog by RFC-0003 §6). This RFC proposes a concrete shape for that ADR to ratify or reject; it is not itself a ratification.

### 2.6 Lightweight Knowledge capture — explicitly not M4

The worked example's **F** role ("document, in two places") is given the smallest honest shape that satisfies it today:

- Two ordinary `Generate` Steps whose `Produced` Artifact is prose, not a code patch, each with a new `apply` target: `"knowledge-note"` (writes to `.foundry/knowledge/<act-id>-<slug>.md`) and `"project-doc"` (writes to a path named in `.foundry/config.json`, e.g. `docs_path`).
- **No indexing, no retrieval, no provenance schema, no reuse by a later Act's Gatherer.** This is a write, not a memory system — deliberately, so this RFC does not have to design M4 to unblock the worked example. It is named here as feeding, and explicitly not preempting, the ADR backlog's **"Knowledge & semantic store"** entry and [roadmap.md](../00-overview/roadmap.md)'s M4.

### 2.7 Budget: per-Step, not per-attempt (prerequisite before running this in anger)

`engine/budget.go`'s `tracker.charge` is called once per attempt at a flat `executeCostEstimateUSD`, regardless of how many Steps (and now, how many *vendors*) that attempt actually runs. A seven-role Pipeline under today's Budget model is charged as if it were a one-Executor Pipeline — [roadmap.md](../00-overview/roadmap.md) open decision 9 ("cost as a first-class constraint") and RFC-0002 §10's own named risk, both still unresolved. This RFC does not propose a full cost-accounting redesign; it recommends per-Step charging (a Step's own estimate, summed) as the minimum fix, gating it before §2.1–§2.6 are exercised end-to-end on a real seven-Step Pipeline, not before the mechanism is built and tested.

---

## 3. What remains genuinely unresolved

- **Whether a literal GitHub Copilot adapter is in scope at all**, versus proving multi-vendor with Claude + one clean-API vendor first (§2.3). Left as an explicit open call, not decided here.
- **Credential storage remains environment-variable-only.** No secrets manager exists or is proposed; this is a real, named limitation inherited from how `ClaudeExecutor` already works, not a new gap this RFC introduces.
- **Whether `require_approval_before_remote_publish` should ever be overridable per-Pipeline** (e.g. a trusted, signed-off Pipeline exempted from a project-wide `true`). This RFC recommends: no, not in v1 — the policy is project-global only, the simplest shape that satisfies the request as given. Escalating to a per-Pipeline override is a future decision, not a default to build in now.
- **Whether `feeds_forward` should eventually generalize to naming *which* prior Step feeds a later one** (not only "the immediately preceding one"), the same way `RepairPolicy.Target` names a specific earlier Step. Deferred until a real Pipeline needs it — the same depth-before-breadth discipline RFC-0002 §5 already applies to the Step-DAG-vs-list tradeoff.

---

## 4. Backlog ADRs this RFC touches

None of the following are decided by this RFC itself — it proposed concrete shapes for each, per the [ADR backlog](../03-adrs/README.md); two have since been ratified as their own ADRs (governance now exists — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), the other two remain backlog:

- **Executor contract & capability model** — §2.3. Ratified as [ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md).
- **Routing & policy** — §2.1, §2.2 (explicit-pin only; negotiation, RFC-0002 §7 layer 2, remains out of scope). Still backlog (proposed ADR-0006).
- **VCS/PR integration & Apply targets** — §2.5 (added to the backlog by RFC-0003 §6; this RFC is the first concrete proposal against it). Ratified as [ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md).
- **Knowledge & semantic store** — §2.6, named as fed-but-not-preempted. Ratified as [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md); RFC-0005 shipped the narrow retrieval slice this RFC's §2.6 fed into, and ADR-0007 has since ratified the note format, `.foundry/knowledge/`'s durability classification, and the decision to decline Derived-Knowledge indexing and semantic retrieval until each has a named trigger.

---

## 5. Risks

- **Cost.** Unchanged from RFC-0002 §10 and RFC-0003 §7's own risk sections, now with a concrete mitigation path named (§2.7) rather than only flagged.
- **Trust-boundary creep.** §2.5's publish step remains Foundry's first outbound write to shared infrastructure (RFC-0003 §4.1's own framing, unchanged). The enforced, load-time-checked project policy is this RFC's answer to "never a convenience shortcut around it" — but it is still a design proposal, not yet exercised in production.
- **Schema surface growth.** §2.1, §2.5, §2.6 together add four new optional fields to the Pipeline-document schema (`capability`, `executor`, `feeds_forward`, `target`) and one new document kind (`.foundry/executors.json`, `.foundry/config.json`). Each is additive and optional, but the *cumulative* schema is now meaningfully larger than RFC-0002 Phase 3 shipped — exactly the kind of compatibility-surface growth the unwritten "Reusable-Act template format & evolution policy" ADR is supposed to own. This RFC's proposals should be read as informing that ADR, not as a substitute for writing it.
- **Vendor-adapter maintenance.** Each new real Executor (a second, a third vendor) is ongoing surface to keep working as that vendor's API/CLI changes — a cost this RFC's "prove with two vendors first" sequencing (§6) is meant to bound, not eliminate.

---

## 6. Final recommendation — sequenced, none blocking the others except where noted

1. **Capability schema + Router (explicit pin only) + `feeds_forward`** (§2.1, §2.2, §2.4) — provable entirely with Claude Code, multiple model configs (`opus-plan`, `sonnet-implement`, `haiku-review`), zero new vendor-adapter risk. This alone already lets **A** plan, **B** implement, **C**/**D** review, **B** resolve, **E** review-then-write-tests.
2. **Write the "Executor contract & capability model" ADR** (§2.3) — before, not after, the second real vendor Executor is written, so it is not reverse-engineered from one adapter's accidental shape.
3. **One additional real vendor Executor** (§2.3's recommendation: a clean-API vendor first; Copilot's fit is an open question for whoever reviews this RFC).
4. **Knowledge-lite capture** (§2.6) — independent of 1–3; can land in parallel.
5. **Per-Step Budget accounting** (§2.7) — required before running the full seven-role Pipeline in production; not required to build/test 1–4 in isolation.
6. **VCS/PR publish policy** (§2.5) — gated on writing the "VCS/PR integration & Apply targets" ADR; the highest-risk, least-precedented piece, sequenced last deliberately, exactly as RFC-0003 §8 already recommended for its own §4.1.

A governance process now exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but this RFC has not yet been individually ratified through it. Until it is, treat it as RFC-0001 through RFC-0003 are treated: Draft, Proposed, argued with rather than deferred to.

---

_This RFC is meant to be argued with. Its falsifiable core claims: (a) every mechanical piece of the worked example is either Router/Capability plumbing already designed in RFC-0002 §4.4/§7, made concrete here, or the one genuinely new forward-context-threading gap named in §2.4; (b) the publish-policy shape in §2.5 is the smallest mechanism that gives a project an enforced (not advisory) switch between "always ask a human" and "always publish"; (c) §2.6's Knowledge capture is honestly a write, not a memory system, and says so. If a piece of the worked example does not fit one of those three buckets, that is the trigger to extend this RFC, not to build around it silently._
