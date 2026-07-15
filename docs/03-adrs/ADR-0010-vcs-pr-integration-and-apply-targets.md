# ADR-0010 — VCS/PR Integration & Apply Targets

| | |
|---|---|
| **Status** | **Proposed** — drafted per [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.5 and [RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md) §4.1; **not ratified**. No governance process exists yet ([OQ-006](../06-open-questions/OQ-006-governance-model.md)); this ADR must not be treated as decided until one does. |
| **Date** | 2026-07-15 |
| **Deciders** | _(pending — proposed by RFC-0003/RFC-0004, AI-assisted, for whoever eventually reviews it)_ |
| **Ratifies** | Nothing yet. Proposes a shape for the ADR backlog entry named in [../03-adrs/README.md](README.md) ("VCS/PR integration & Apply targets"), numbered ADR-0010 to match the row order [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) already implied (0006 Routing & policy, 0007 Knowledge & semantic store, 0008 Extension isolation, 0009 CLI & output contract, 0010 this entry, 0011 Cost). |
| **Gates** | Piece 6 of [multi-executor-router-implementation-plan.md](../04-guides/multi-executor-router-implementation-plan.md) — the `remote-pr` apply target, `.foundry/config.json`'s publish-policy fields, and the `vcs` package. Per that plan, Piece 6 does not start until this ADR exists — the highest-risk, least-precedented piece, sequenced last deliberately. |
| **Process note** | Same posture as RFC-0003/RFC-0004 and [ADR-0005](ADR-0005-executor-contract-and-capability-model.md): Draft — Proposed, argued with rather than deferred to, pending [OQ-006](../06-open-questions/OQ-006-governance-model.md). Nothing here gates any already-shipped work (Pieces 1–5); it gates only the one piece that is Foundry's first outbound write to shared infrastructure. |

---

## Context

Every apply Step Foundry executes today stays entirely on the developer's own machine. `local` (`workspace.GitApplier`/`ApplyAct`) fast-forwards the developer's own branch with a patch a human already approved via an `approve` Step. Piece 4 of [multi-executor-router-implementation-plan.md](../04-guides/multi-executor-router-implementation-plan.md) (shipped) added two more targets on the same footing — `knowledge-note` and `project-doc` — both local file writes, resolved through the same `engine.ApplierRegistry`, gated behind the same `approve`-then-`apply` Step ordering `runSteps` already enforces (`act.ApprovedAt != nil`).

RFC-0003 §4.1 named "commit and open a pull request" as the request's highest-risk piece and deliberately did not resolve it, only shaped the decision: this is not a bigger `Apply`, but a new axis entirely, along three dimensions at once — it **leaves the machine** (Foundry's first outbound write to shared infrastructure, not just the developer's own checkout), it **touches Authority (I5)** (is a PR itself a second review surface, distinct from the Act's own `approve` Step), and it **needs a credential Foundry does not manage today** (a VCS host token, not an Executor's API key, though the shape rhymes).

RFC-0004 §2.5 proposed a concrete shape for this ADR to ratify or reject:
- A new apply target, `remote-pr`, alongside `local` (`Apply`'s own meaning is not extended, per RFC-0003 §4.1's explicit recommendation).
- A project-level, enforced switch — `.foundry/config.json`'s `require_approval_before_remote_publish` — not a per-Pipeline courtesy.
- Enforcement at Pipeline registration, not at run time: a Pipeline declaring `remote-pr` without a preceding `approve` Step is a load-time configuration error a human sees immediately, never a silent bypass discovered after an Act ran.
- A mechanism that shells out to the `gh` CLI — subprocess, not an embedded API client, mirroring [ADR-0001](ADR-0001-language-and-toolchain.md)'s existing extension-boundary posture (`executor/claude`, `git apply`) — reading a host credential from an environment variable named in project config, never stored by Foundry, mirroring [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) Decision 5's Executor-credential pattern.

This ADR resolves what RFC-0004 §2.5 named but did not itself ratify: exactly how the project-level policy is represented and enforced, exactly what Foundry's accountability story does and does not cover once a PR is opened, and exactly where the new mechanism lives.

---

## Decision

1. **`Apply`'s meaning does not change.** A Step declares an apply Target — `local` (default, unchanged), `knowledge-note`/`project-doc` (Piece 4, shipped), or the new `ApplyTargetRemotePR = "remote-pr"` — resolved through the same `engine.ApplierRegistry` Piece 4 already built. No new Step kind, no `engine`, `engine.Strategy`, or `engine.Router` change. `runSteps`' existing `StepKindApply` case (`resolveApplier`, unmodified) already generalizes to this target exactly as it does to `knowledge-note`/`project-doc` today.

2. **`project.Config` gains two additive fields**, mirroring `ExecutorConfig.APIKeyEnv`'s credential-by-reference pattern (ADR-0005 Decision 5):
   ```go
   type Config struct {
       DocsPath                           string `json:"docs_path"`
       RequireApprovalBeforeRemotePublish bool   `json:"require_approval_before_remote_publish"`
       RemotePublishTokenEnv              string `json:"remote_publish_token_env"`
   }
   ```
   Zero values (absent from `.foundry/config.json`) mean "no requirement, no configured token" — a project that never opts in sees no change, exactly as `DocsPath`'s own absence means today. `LoadConfig`'s shape and behavior (missing file → zero `Config`, not an error) does not change.

3. **Enforcement is a load-time check inside `engine.PipelineRegistry`, not a runtime check inside `runSteps`.** `PipelineRegistry` gains:
   ```go
   // SetPublishPolicy configures whether Register/RegisterMany require an
   // approve Step before any apply Step targeting ApplyTargetRemotePR,
   // mirroring .foundry/config.json's require_approval_before_remote_publish.
   // Never calling it (the zero value) means no such requirement is
   // enforced — exactly today's behavior, and exactly what an absent
   // config file means.
   func (r *PipelineRegistry) SetPublishPolicy(requireApprovalBeforeRemotePublish bool)
   ```
   When set `true`, `Register`/`RegisterMany` refuse — leaving the registry unchanged, the same "no partial registration" contract `Register` already keeps for a duplicate name — any Pipeline declaring a `remote-pr` apply Step with no `approve` Step earlier in the same `Steps` sequence. The composition root calls `SetPublishPolicy` from `project.LoadConfig`'s result *before* registering any Pipeline, so a misconfigured Pipeline is an error a human sees at `foundry`'s startup (or `/init`/`ReloadPipelines` in the interactive session), never a bypass discovered only after an Act already ran and spent Budget.

4. **The required `approve` Step is the same, already-existing mechanism** — `domain.StepKindApprove` and `Authority.Decide` — not a second kind, and not a distinguished "publish approval" versus "Act approval." A Pipeline author may reuse the Act's one `approve` Step (if it precedes the `remote-pr` apply Step) or declare a second one immediately before publish; either satisfies Decision 3's registration-time check and `runSteps`' existing `act.ApprovedAt != nil` runtime check (unmodified), exactly as `local` is gated today.

5. **Opening the pull request is Foundry's terminal action for this target**, mirroring `local`'s "land the branch" as terminal. Foundry does not model the PR's own review or merge as a further Judgment, `approve` Step, or Record entry — that is a separate, GitHub-native review process outside Foundry's Record, exactly as whatever a developer does with a locally-landed commit afterward is outside it today. This directly answers RFC-0003 §4.1's open question — **no**, a PR is not a Foundry-modeled second review surface. Foundry's entire accountability story for the Act ends at Decision 4's `approve` Step, per I5; nothing polls, watches, or reacts to the PR's later fate.

6. **Foundry does not verify that the `approve` Step's Authority is the same identity `remote_publish_token_env` resolves to.** This is a named, accepted limitation (see Open Questions), not a silent gap — the same posture ADR-0005 Decision 4 already took for Capability truthfulness ("declared, not verified" until a concrete need motivates otherwise).

7. **Mechanism: a new package, `vcs`**, holding a `GitHubPRApplier` type satisfying the unchanged `engine.Applier` interface. It is constructed with its resolved parameters (e.g. `{BaseBranch, TokenEnv string}`) by the composition root — mirroring `workspace.ProjectDocApplier{DocsPath}`'s shape (Piece 4) — and shells out to the `gh` CLI to push a branch and open a pull request. `project` stays VCS-agnostic: it only decodes `RemotePublishTokenEnv` as a plain string (Decision 2); only the composition roots (`cmd/foundry/commands/do.go`, `session.Session`) know `vcs.GitHubPRApplier` exists, the same separation ADR-0005 Decision 5 established for vendor `Executor`s.

8. **Scope: GitHub, via `gh`, only.** A GitLab/Bitbucket (or other host) adapter is a distinct future package satisfying the same `engine.Applier` contract, under its own target name (e.g. `remote-mr`), if and when a project needs one — not designed here (see Open Questions).

---

## Alternatives Considered

### Extend `Apply`'s existing meaning, or gate publishing with a boolean flag rather than a declared target
- **For:** Fewer new concepts than a whole new Target value.
- **Against:** RFC-0003 §4.1 already argued this explicitly: a flag makes "what did apply just do" depend on out-of-band project state rather than the Pipeline's own declared shape, the opposite of RFC-0002 §5's "Step kinds are a closed set... not the Pipeline document inventing arbitrary behavior" discipline. A declared Target is self-documenting in the Pipeline itself.
- **Verdict:** Rejected, per RFC-0003 §4.1's own recommendation (Decision 1).

### Enforce the approval requirement at run time (inside `runSteps`) instead of at Pipeline registration
- **For:** One fewer public method on `PipelineRegistry`; reuses the exact place `local`'s own approval gate already lives.
- **Against:** A misconfigured Pipeline would run all the way to the `remote-pr` apply Step — spending real Budget on Generate/Verify Steps first — before failing, exactly the "discovered after the fact" failure mode RFC-0004 §2.5 explicitly rejected. A load-time refusal costs nothing and is visible immediately.
- **Verdict:** Rejected in favor of Decision 3's registration-time check.

### Model a PR's merge/close as part of the Act's lifecycle (poll GitHub, update Judgment on merge)
- **For:** Would give Foundry's Record a complete picture of whether published work was ultimately accepted.
- **Against:** No consumer needs this today; it requires new live-polling or webhook machinery Foundry has never had, and it risks mutating a recorded Act's Judgment well after the fact — in tension with the Record's durability/immutability (I8) and with treating a PR as anything other than a downstream artifact. Speculative machinery for a need nothing has yet.
- **Verdict:** Rejected (Decision 5). Revisit only if a real, motivating need for post-Act PR tracking appears — this ADR does not preempt that, it only declines to build it now.

### A mechanism to verify the approving Authority matches the publish credential's identity
- **For:** Would close the gap RFC-0003 §4.1 named — "does the human who approved the Act need to be the same human whose credentials open the PR."
- **Against:** No Executor credential is verified against its configuring human today either (ADR-0005 Decision 4's "declared, not verified" posture); building identity-linking here would require Foundry to manage or proxy per-human VCS credentials, contradicting the established "Foundry never manages credentials" posture (RFC-0004 §2.2, ADR-0005 Decision 5) for a need nothing has yet motivated concretely.
- **Verdict:** Rejected for this ADR (Decision 6); named as an explicit Open Question rather than silently assumed solved.

### A dedicated `vcs.Applier` interface distinct from `engine.Applier`
- **For:** Could carry VCS-specific metadata (PR URL, branch name) a generic `Applier` doesn't.
- **Against:** `engine.Applier`'s existing one-method contract (`Apply(ctx, workspace, act) error`) already generalizes fully — Piece 4 already proved this by reusing it for prose writes, not just git patches. A second interface would duplicate what Piece 4 already established works, for no concrete need.
- **Verdict:** Rejected (Decision 7). `GitHubPRApplier` satisfies `engine.Applier` exactly like every other Applier.

---

## Consequences

### What this decision makes EASIER
- **A Pipeline author declares `target: "remote-pr"` exactly the way they already declare `knowledge-note`/`project-doc`** (Piece 4) — zero new mental model, zero new Engine concept.
- **A project flips `require_approval_before_remote_publish` on or off as pure data**, with no Engine, Strategy, or Router change, and no risk of a misconfigured Pipeline running to completion before failing.
- **Whoever builds Piece 6 has a validated template twice over**: ADR-0005 Decision 5's "composition root resolves config into a concrete struct, registers it into a registry" pattern already shipped for `executor/openai` (Piece 3) and `workspace.ProjectDocApplier` (Piece 4); `vcs.GitHubPRApplier` is a third instance of the same shape, not a new one to invent.

### What this decision makes HARDER
- **Two genuinely different trust postures now coexist under one `Applier` interface.** `local`/`knowledge-note`/`project-doc` are fully reversible, single-machine, no external credential; `remote-pr` is not reversible from Foundry's own reach (Foundry cannot un-push a branch or close a PR it opened) and touches a real external identity/credential. `engine.Applier`'s contract does not distinguish these — a future caller wanting to treat "this apply Target reaches outside the machine" specially (e.g. a stricter Authority requirement, a dry-run mode) has no shared marker to switch on yet. This mirrors ADR-0005's own named consequence that two failure taxonomies (subprocess vs. HTTP) coexist unmarked under `Executor`.
- **`SetPublishPolicy` is global per `PipelineRegistry`, not per-Pipeline.** A project cannot require approval for one Pipeline's `remote-pr` target while exempting another's. This matches `require_approval_before_remote_publish`'s own project-level (not Pipeline-level) framing in RFC-0004 §2.5, but is now a locked-in narrowing, not an oversight to fix casually later.

### Reversibility
High. `SetPublishPolicy`'s default (never called) is exactly today's behavior. `ApplyTargetRemotePR` is additive — Piece 4's `ApplierRegistry` mechanism already generalizes to it untouched. Removing `vcs.GitHubPRApplier` later affects only projects that opted into `remote-pr`; no `engine.Applier`, `engine.Step`, or `runSteps` contract changes are made or would need undoing.

---

## Migration Strategy

None required. No field or Step declared before this ADR decodes any differently; `project.Config`'s two new fields and `PipelineRegistry.SetPublishPolicy` are purely additive. Building `vcs.GitHubPRApplier`, wiring `SetPublishPolicy` and the new `Config` fields into both composition roots (`cmd/foundry/commands/do.go`, `session.Session`), and writing `remote-pr`'s commit-by-commit plan is Piece 6's implementation work — a follow-up to this ADR, exactly as ADR-0005 preceded Piece 3's actual commits rather than writing them itself.

---

## Future ADR Dependencies

None identified beyond what [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) already named (the Cost ADR, proposed as ADR-0011, does not depend on this one). A future GitLab/Bitbucket adapter (Decision 8) would be its own small follow-up, not a dependency of this ADR.

---

## Open Questions

1. **Should Foundry ever verify that the `approve` Step's Authority matches the identity behind `remote_publish_token_env`?** Left open (Decision 6), matching ADR-0005's own "declared, not verified" precedent for Capability truthfulness — revisit only if a real incident or concrete need motivates it.
2. **Is a GitLab/Bitbucket (or other host) adapter in scope, and under what target name?** Not decided here (Decision 8); a future, smaller ADR or plain implementation PR can add it once a project actually needs one.
3. **What happens to a pushed-but-PR-creation-failed branch** — force-cleaned, or left on the remote for a human to inspect? Left as `vcs.GitHubPRApplier`'s own implementation choice (mirrors `ApplyAct`'s existing "never leave a *local* throwaway branch behind on success" — the remote case's failure-path convention is not decided here).
4. **Should `SetPublishPolicy` become per-Pipeline rather than per-registry** if a project ever needs mixed policies across Pipelines? Left open — no concrete need motivates it today, mirroring `Router`'s own "no capability negotiation until a real need motivates it" discipline (ADR-0005 Decision 4).

---

## Review Checklist

For whoever eventually ratifies or rejects this ADR (blocked on [OQ-006](../06-open-questions/OQ-006-governance-model.md)):

- [ ] **No contradiction with accepted documents.** Confirmed at authoring: does not contradict ADR-0001 (`gh` runs as a subprocess, in-process Go caller — the extension-boundary discussion stays separate); does not contradict [trust.md](../02-architecture/trust.md) or I5 (Decision 4 reuses, never bypasses, the existing `approve`/Authority mechanism); does not contradict I8/I10 (Decision 5 explicitly declines to let a PR's later fate mutate a recorded Act).
- [ ] **Decision 3's `SetPublishPolicy` shape actually holds** once Piece 6 builds it: does a real `.foundry/config.json` + multi-Pipeline project confirm the registration-time check is unambiguous, or does it need reshaping against real project structures?
- [ ] **Decision 6 is re-examined, not silently inherited,** the moment any credential-identity-linking mechanism is proposed for Executors or VCS hosts alike.
- [ ] **Decision 8's GitHub-only scope is still accurate** — re-open if a second VCS host adapter is ever proposed.
- [ ] **Process caveat tracked.** Reconcile this ADR's Proposed status once [OQ-006](../06-open-questions/OQ-006-governance-model.md)'s governance process exists.

---

_This ADR proposes, and does not itself ratify, the shape Foundry's first outbound, off-machine action takes. It keeps `Apply`'s meaning and `engine.Applier`'s contract unchanged, adds a project-level publish policy enforced at Pipeline-registration time rather than at run time, and explicitly declines to model a pull request's own review as a second Foundry-tracked accountability surface — deferring identity-linking and multi-host support to whichever future need actually motivates them._
