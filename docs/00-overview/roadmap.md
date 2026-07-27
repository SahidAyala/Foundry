# Roadmap

> The strategic build sequence and the open decisions that gate it. The detailed build plan (PR-level steps, testing, CI, repository layout) lives in [../04-guides/development.md](../04-guides/development.md). Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Guiding rule

**Depth before breadth.** Build one complete, trustworthy loop before widening the lifecycle. Build the deterministic core before any model is integrated — a model is one **Executor**, added late.

## Milestones

| Milestone | Theme | "Usable system" state |
|---|---|---|
| **M0** | Walking skeleton | The engine produces a trivial, fully deterministic Act end-to-end |
| **M1** | Deterministic core | Verified, judged, budgeted Acts; immutable Record; replay & resume — **no model involved** |
| **M2** | Reusable production + mock executor | Author reusable Act templates; produce Acts against a fixture Executor, deterministically; bounded repair |
| **M3** | Real executors | Model-backed Executors behind capability routing; record/replay against real models |
| **M4** | Knowledge & evidence | Authored/Derived Knowledge; evidence assembly with provenance; knowledge-update as a reviewed Act |
| **M5** | Integration & visibility | Worktree isolation, change review/approval, version-control integration, observability |
| **M6** | Extensibility | Open substrate edge: third-party Strategies, Executors, Validators, Context Sources |
| **M7** | Stability | Frozen core contracts, hardening, multi-user / enterprise readiness |

Each milestone is independently useful. A project could stop at M1 and have a rigorous deterministic engine with audit and replay.

## Parallel track: Interactive terminal UX

Not part of the M0–M7 depth sequence above — that table orders *the trust/knowledge lifecycle*, each milestone deliberately gating the next (RFC-0002 §9's Phase 0 pattern). This track is orthogonal: it makes the *existing* interactive session (already [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md)'s ratified primary interface) more pleasant to use — autocomplete over slash commands, live suggestions, styled output, closer to Claude Code/Codex/OpenCode's own terminal experience — without changing what an Act is, how it's judged, or any trust-model concept. It neither blocks nor is blocked by M6/M7.

Ratified via [ADR-0012](../03-adrs/ADR-0012-interactive-terminal-ux-and-first-dependency.md) (Accepted 2026-07-22) — Foundry's first-ever third-party dependency, confirmed by the maintainer personally. Implementation follows.

## Parallel track: Ticket-driven Acts (`/issue`)

Also orthogonal to the M0–M7 sequence: `/issue <id>` fetches an external ticket's content (an issue tracker — GitHub, Jira, GitLab, or Asana, per the maintainer's own stated priority order) and uses it as an Act's Intent, instead of a human typing one. This is deliberately **not** a new Step kind — RFC-0002 §4.2's closed five (generate, verify, approve, apply, record) are unchanged; fetching happens before a Pipeline starts, the same way a slash command's own typed argument text already does. No new architectural decision was needed: this is a new implementation of the already-named "Context Source" extension point ([extensibility.md](../02-architecture/extensibility.md)), the same way adding another Executor vendor needs no new ADR.

All four providers the maintainer named are shipped: GitHub (`ticket/github`, reusing the same `gh` CLI session `vcs.GitHubPRApplier`'s own PR-opening already requires), Jira (`ticket/jira`, a pure HTTP call — no equivalent already-authenticated CLI to piggyback on, so it needs its own Basic Auth credential), GitLab (`ticket/gitlab`, mirroring GitHub's own approach via the `glab` CLI's own session instead of a raw token), and Asana (`ticket/asana`, a pure HTTP call like Jira's — no CLI convention, no separate base URL either, since Asana's API has one fixed global endpoint). A fifth provider, `ticket/backlog`, was added later: a local, project-committed JSON file (no external API/CLI/credential at all), validated against an external "harness engineering" example project before adopting — see [implementation-status.md](implementation-status.md)'s own changelog for that validation and what was (and wasn't) judged portable.

## Current implementation status

The table above states the *plan*. For what has actually shipped per milestone, per RFC, and per architecture document — kept current as a living index rather than duplicated here — see [implementation-status.md](implementation-status.md). Getting-started instructions for what's usable today (install, dependencies, first run) live in [../04-guides/getting-started.md](../04-guides/getting-started.md).

## Open decisions that require a human (architectural / governance)

These are unresolved and **must not be settled silently in implementation**. They are the prerequisites to any architecture "freeze". The ones that are architectural (not pure governance) are treated in depth, with alternatives and a current recommendation, in [../06-open-questions/](../06-open-questions/) — that tier is the home for this deliberation so it never leaks into canonical docs.

1. ~~**Governance & ratification process.**~~ **RESOLVED 2026-07-16** — see [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md): a lightweight, sole-maintainer-led process, with no enforced comment period while the project has one maintainer, and a named trigger (a second contributor) that forces a real comment period to be defined. The founding RFC (RFC-0001) itself remains unratified — resolving *how* decisions are ratified does not itself ratify any pending RFC.
2. **Principle priority ordering.** **PARTIALLY RESOLVED 2026-07-26** — see [ADR-0014](../03-adrs/ADR-0014-principle-priority-ordering.md): when a numbered Principle and a Value conflict, the Value always wins (ratified with a concrete worked example — approval must never be skipped, however trivial the Act). **Still open:** any ranking among the 8 numbered Principles themselves, or among the 6 Values themselves, when two from the same tier conflict (see [principles.md](principles.md)).
3. ~~**The center of the domain — Act vs Knowledge.**~~ **RESOLVED 2026-07-26** — see [ADR-0015](../03-adrs/ADR-0015-domain-center-act-and-knowledge.md): the maintainer confirmed ratifying the already-adopted working model as-is — Act as the dynamic center, Knowledge as the durable medium (see [../02-architecture/domain.md](../02-architecture/domain.md)). Resolves this decision only; whether Strategy is itself a domain concept, and whether a rejected/abandoned Act still deposits into Knowledge, remain open ([OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md)).
4. ~~**Cross-version replay scope.**~~ **RESOLVED 2026-07-20** — see [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md): scoped to same-version only; cross-version replay is explicitly out of scope, not merely undecided.
5. ~~**Validator determinism limits & the honest verification guarantee.**~~ **RESOLVED 2026-07-20** — see [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md): Validators and the Gate are re-executed for real on replay; a verdict divergence is honest data, never hidden or asserted impossible.
6. ~~**Record durability classification** — durable, not a cache.~~ **RESOLVED 2026-07-20** — see [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md): the Record is durable, ratifying [I8](../05-reference/invariants.md); commit `.foundry/acts/` to a project's own repository, the same convention `.foundry/pipelines/` already follows.
7. ~~**Authored-knowledge format stability & migration.**~~ **RESOLVED 2026-07-21** — see [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md): unstructured Markdown prose (no front-matter, no schema) is ratified as the note format, closing the question by removing what would need to be stable; `.foundry/knowledge/` should be committed to a project's own repository, extending [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s `.foundry/acts/` convention. Derived Knowledge indexing, semantic retrieval, provenance scoring, and note curation remain explicitly declined until each has a named trigger (see [../02-architecture/knowledge.md](../02-architecture/knowledge.md)).
8. ~~**Extension isolation mechanism & contract versioning.**~~ **RESOLVED 2026-07-21 (as a deliberate non-decision)** — see [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md): no mechanism chosen, deferred until a real third party wants to write a Strategy/Executor/Validator/Context Source outside this repository (see [../02-architecture/extensibility.md](../02-architecture/extensibility.md)). The maintainer directly reconfirmed on 2026-07-26 that this remains the right posture — keep waiting for that real trigger rather than deciding preemptively.
9. ~~**Cost as a first-class constraint.**~~ **RESOLVED 2026-07-21** — see [ADR-0011](../03-adrs/ADR-0011-cost-as-a-first-class-constraint.md): `CostEstimator` ([ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md) Decision 3) stays optional, not mandatory — Claude Code's subprocess has no billing signal to report, a structural limit, not a "not yet." An optional, additive `ActualCostUSD` on `Outcome`/`StepRecord`/`Act` lets an Executor that can (`executor/openai`) report real post-execution cost as reported Evidence, never a second Budget gate. `engine/budget.go`'s hardcoded ceilings and any cost reconciliation/calibration remain declined until each has a named, concrete trigger. **The paired product-strategy half is now also resolved (2026-07-26):** the maintainer confirmed directly that near-term roadmap priority should continue favoring the trust/Knowledge-compounding depth (M4/M5) over near-term single-user ergonomic wins — consistent with [vision.md](vision.md)'s own stated target audience (those who accept up-front discipline for work that compounds, not throwaway single-shot value). This is a prioritization stance, not a technical decision — no ADR needed, the same lightweight treatment the project name (item 11 below) received.
10. **Concurrency / scale model** and whether remote operation is truly additive (see [../02-architecture/system-context.md](../02-architecture/system-context.md)). **Still genuinely unproven** — but the maintainer confirmed directly (2026-07-26) the deliberate choice to keep this as an unproven working assumption for now, rather than invest in designing/validating it before a real multi-user/scale need appears. This is a confirmed *posture* (don't invest yet), not a resolution of the underlying question itself — [system-context.md](../02-architecture/system-context.md)'s own "Unresolved" callout stands unchanged.
11. ~~**The project name.**~~ **RESOLVED 2026-07-26** — the maintainer confirmed "Foundry" directly, no longer provisional.

Several of these have proposed owning ADRs; see [../03-adrs/README.md](../03-adrs/README.md).

## What may begin now

M0 work may proceed under the accepted language decision ([../03-adrs/ADR-0001-language-and-toolchain.md](../03-adrs/ADR-0001-language-and-toolchain.md)). It is insulated from the open decisions above, which gate later milestones — not the first commit.
