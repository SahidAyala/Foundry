# ADR-0009 — CLI & Output Contract

| | |
|---|---|
| **Status** | **Proposed** — drafted per [RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md) §6 and the ADR backlog; not yet ratified. |
| **Date** | Drafted 2026-07-20 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted per RFC-0003 §6 |
| **Ratifies** | The ADR backlog entry named in [../03-adrs/README.md](README.md) ("CLI & output contract") — the product-shape decision (interactive session vs. flag CLI), the command/flag/exit-code stability policy, and the slash-command surface's own stability promise. |
| **Gates** | [RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md) §6's named "CLI & output contract" gap. Also one of the five compatibility surfaces [release.md](../04-guides/release.md) names as "gated by release" — that guide's own text says its "Honest versioning" principle is "intended, not ratified" until this ADR exists. |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. RFC-0003, the RFC this ADR was drafted from, remains Draft — Proposed in its own right; ratifying this ADR does not ratify that RFC. |

---

## Context

RFC-0003 names this ADR directly, and narrowly, at §6:

> **CLI & output contract** — gates whether `foundry`'s primary interface is the interactive session or the flag CLI, and the slash-command surface's own stability promise.

That is RFC-0003's only sentence on this subject — it does not itself decide a stability policy, an output format, or exit-code semantics anywhere in its 171 lines. It also flags one more thing explicitly as this ADR's to close, at §5: `engine.PipelineProvider` still uses the retired term *Provider*, and RFC-0003 "does not rename anything; it records the tension for whoever writes the 'CLI & output contract' ... ADR to close explicitly."

**What is already decided, informally, and already implemented — this ADR ratifies it, not invents it:**

- **The product-shape direction.** The maintainer's 2026-07-18 decision (recorded informally in [README.md](README.md)'s ADR-0009 backlog row) is that the interactive session is Foundry's primary interface; the flag-based commands are a secondary, CI/automation-only surface. [getting-started.md](../04-guides/getting-started.md) already documents this split under two section headers: "Interactive session — the primary way to use Foundry" and "One-shot command — for CI and automation scripting only," and states plainly: "The one-shot `foundry do` command below exists specifically for CI and automation scripting; it is not a second, equally-preferred way to use Foundry day to day."
- **The actual flag CLI**, in `cmd/foundry/main.go`: five commands — `do`, `log`, `show`, `replay`, `resume` — plus `-h`/`--help`/`help`. Flags across them: `--repo`/`-repo` (all five), `-n`/`--limit` (`log` only). Invoking `foundry` with no arguments starts the interactive session (`runSession`), replacing the old "print usage, exit 2" behavior; the comment at `main.go` is explicit that "the one-shot ... subcommands ... remain available unchanged for scripting and CI; the interactive session is additive, not a replacement."
- **An exit-code convention that already exists in every command file, but is asserted nowhere as a promise**: `0` for success or `-h`/`--help`, `1` for a runtime/internal error, `2` for an argument-parsing/usage error. It is consistent across `do.go`, `log.go`, `show.go`, `replay.go`, `resume.go`, and `main.go`'s own dispatch — an implicit accident of five independently-written files agreeing, not a documented contract.
- **The actual slash-command surface**, in `session/repl.go`'s `DefaultCommandRegistry`: `/init`, `/feature`, `/bug`, `/review`, `/release`, plus `/exit`/`/quit` handled directly in `handleLine`. A project's own authored Pipeline document can already back its own additional slash command the same way (the registry's own doc comment: "this registry is the default set, not the only possible one").
- **No machine-readable output contract exists.** Every command writes free-form, human-oriented text (optionally ANSI-colored on a TTY, via `cli/render.go`); there is no `--json` flag, no structured output mode, and no distinct exit code per failure class — a verification failure, a declined approval, a missing repo, and a config error all currently collapse to exit code `1`.
- **`release.md` already states an intended (not yet ratified) versioning principle** for exactly this surface: "Pre-1.0, internal contracts may change with documented migration notes. At 1.0, the durable-core contracts ... are frozen under semantic versioning." Its own disclaimer: "Treat the freeze commitments above as intended, not ratified" until the owning ADR (this one) exists.

**What is a real, live discrepancy this ADR must address, not just describe:** `cli/interactive_renderer.go`'s startup banner tells every user to "Type /init to get started, or /help for a list of commands" — but `/help` is not registered anywhere. RFC-0003 §3.1's own original sketch additionally named `/status`, `/pipeline`, `/approve`, `/reject` as the target slash-command set — none of those were built either; the shipped vocabulary (`/init`, `/feature`, `/bug`, `/review`, `/release`) diverged from that sketch during implementation, which is expected and fine, but the banner actively promising a nonexistent command is a real, user-facing defect, not a documentation nuance.

**Terminology gap this ADR fills, not just inherits:** [terminology.md](../05-reference/terminology.md) defines no noun for "the interactive session" or "a slash command" today — the only brush with it is a parenthetical example under **Strategy** ("a human-driven session"). This ADR is the first document to mint that vocabulary, matching AGENTS.md's allowance to build on PROVISIONAL ground while marking it as such.

This ADR does not resolve **Routing & policy** (backlog, ADR-0006) or **Extension isolation & contract versioning** (backlog, ADR-0008) — a third-party's ability to add its own CLI command or slash command is that ADR's question, not this one's.

---

## Decision

1. **The product-shape direction is formally ratified.** The interactive session (`foundry`, no subcommand) is Foundry's primary interface. `foundry do`, `log`, `show`, `replay`, and `resume` remain a secondary, explicitly CI/automation-focused surface — not a parallel, equally-preferred way to use Foundry day to day. This promotes the maintainer's informal 2026-07-18 call (README.md's ADR-0009 row) into a governed decision under [ADR-0000](ADR-0000-governance-and-ratification-process.md), without ratifying RFC-0003 as a whole.

2. **Pre-1.0, the flag CLI's commands, flags, and exit codes may change; each such change must carry a documented migration note.** This ratifies [release.md](../04-guides/release.md)'s already-stated "Honest versioning" principle for this specific surface: no semver freeze yet, but a breaking change to `do`/`log`/`show`/`replay`/`resume`'s arguments, flags, or exit-code meaning is called out (a `BREAKING CHANGE:`-style note in the commit, surfaced through this repository's conventional-commit-driven changelog) rather than landing silently. At 1.0, this surface is frozen under semantic versioning, per release.md's existing text.

3. **The slash-command surface carries the same pre-1.0 policy as Decision 2, explicitly** — RFC-0003 §6 names both together, and this ADR does not treat them differently. Today's five (`/init`, `/feature`, `/bug`, `/review`, `/release`) plus `/exit`/`/quit` are the current, real vocabulary; none of them are frozen yet. A project's own authored Pipeline document may continue to back its own additional slash command with zero new ceremony — this ADR does not change that mechanism, only confirms it stays available.

4. **The existing exit-code convention (`0` success/help, `1` runtime error, `2` usage error) is ratified as the current, documented contract**, not a new invention — it already matches every shipped command's actual behavior. It is deliberately coarse: no distinct code exists per failure class (verification failure, declined approval, missing repository, configuration error), and this ADR does not add one. Refining it is left open (see Open Questions), not decided speculatively here.

5. **No machine-readable output contract (JSON or otherwise) is added.** Plain, human-oriented text plus the exit code (Decision 4) is the entire output contract today. Nothing in this ADR invents a `--json` schema that doesn't exist in code, per AGENTS.md's rule against describing unwritten code. Revisit only when a real CI consumer needs to parse more than an exit code — not speculatively here.

6. **The startup banner's phantom `/help` reference is a defect, fixed by building `/help`, not by removing the reference.** `/help` becomes a real command in `session.DefaultCommandRegistry`, listing the registry's own registered commands (name plus one-line description) — the minimum needed to make the banner's own promise true. RFC-0003 §3.1's original sketch of `/status`, `/pipeline`, `/approve`, `/reject` is treated as superseded by the vocabulary actually shipped (`/feature`, `/bug`, `/review`, `/release`); those four are not built now, since nothing today needs them — RFC-0003 is not rewritten, but its sketch should not be read as a commitment this ADR carries forward.

7. **`engine.PipelineProvider` (and its implementations `engine.BuiltinProvider`, `project.FilesystemPipelineProvider`) are renamed to retire the live use of *Provider*** — `engine.PipelineSource`, `engine.BuiltinPipelineSource`, `project.FilesystemPipelineSource`. This closes the exact tension RFC-0003 §5 assigned to "whoever writes the CLI & output contract ... ADR": *Provider* here never meant the retired concept (a model/vendor abstraction, replaced by *Executor*) — it discovers Pipeline documents from a source (filesystem, embedded asset, builtin Go value) — but AGENTS.md's historical-isolation rule treats the word itself as a defect wherever it appears as live vocabulary, regardless of which meaning it carries. *Source* is not yet a defined canonical noun either (only roadmap.md's informal "Context Sources" for a future M6 concept), so this decision mints it for Pipeline discovery specifically; a future Context Source concept is free to reuse or diverge from it.

8. **Two terms are added to [terminology.md](../05-reference/terminology.md)'s Mechanism section, marked PROVISIONAL like everything else there:** **Session** ("the persistent, project-rooted interactive process a user runs `foundry` into; the primary interface, per Decision 1") and **Slash Command** ("one named, user-typed instruction inside a Session, dispatched by a CommandRegistry; the vocabulary a Session understands"). Both describe shape that already exists in code (`session.Session`, `session.REPL`, `session.CommandRegistry`) and was previously undocumented at the vocabulary tier, not new design.

---

## Alternatives Considered

### Freeze the flag CLI under strict semver now, pre-1.0
- **For:** Maximum safety for any CI script already depending on today's `do`/`log`/`show` shape.
- **Against:** No external consumer exists yet beyond this repository's own tests and the maintainer's own dogfooding; freezing now risks the same premature-hardening trap ADR-0004 explicitly rejected for the Pipeline document format. `release.md` already anticipates a pre-1.0/post-1.0 split; this ADR should ratify that plan, not skip ahead of it.
- **Verdict:** Rejected. Ratified instead as Decision 2 — free to change, but never silently.

### Design and build a `--json` output mode now
- **For:** Would give CI scripts something more precise than scraping free text or reading a single collapsed exit code.
- **Against:** No consumer has asked for it, no schema exists in code today, and inventing one here would violate AGENTS.md's rule against describing unwritten code. Speculative for a need that hasn't materialized, exactly the reasoning ADR-0004 used to reject an explicit Pipeline-document version field.
- **Verdict:** Rejected for now. Revisit only against a real CI-consumer need (Open Questions).

### Build RFC-0003 §3.1's originally-sketched `/status`, `/pipeline`, `/approve`, `/reject` commands to match the RFC
- **For:** Would make the shipped slash-command surface match what RFC-0003 itself describes, closing a documentation/implementation gap.
- **Against:** Nothing today exercises or needs them; the implementation diverged from the RFC's early sketch for good reason (the shipped Pipeline-backed model — `/feature`, `/bug`, `/review`, `/release` — covers the same ground more directly). Building unused commands speculatively contradicts this project's own "no half-finished implementations, no hypothetical futures" rule.
- **Verdict:** Rejected. `/help` (Decision 6) is built because the banner actively promises it *today*; the RFC's broader sketch is left as historical context, not a backlog item this ADR creates.

### Leave `engine.PipelineProvider` as-is
- **For:** Avoids a mechanical rename touching roughly a dozen files (`engine/provider.go`, `engine/builtin_provider.go`, `project/filesystem_pipeline_provider.go`, and their tests, plus every doc comment naming them).
- **Against:** RFC-0003 explicitly assigned this exact decision to this exact ADR; leaving it unresolved would let a known, named tension sit indefinitely, contradicting AGENTS.md's historical-isolation guarantee #1 ("if you see ... Provider ... used as live vocabulary in an active doc [or, by the same principle, in active code], that is a defect").
- **Verdict:** Rejected. Ratified instead as Decision 7.

---

## Consequences

### What this decision makes EASIER
- **RFC-0003 §6's gap is closed** — the product-shape direction is governed, not just informally understood, and the slash-command/CLI stability question RFC-0003 explicitly deferred now has an answer.
- **`release.md`'s own versioning language stops being "intended, not ratified"** for this specific surface — a future release can cite this ADR directly.
- **A real user-facing bug (the phantom `/help`) is fixed**, and a known terminology tension (`PipelineProvider`) is closed, rather than both persisting as background noise for the next reader to rediscover.
- **CI scripts get an explicit, if coarse, contract to depend on** (Decision 4) instead of an accidental one no one promised would hold.

### What this decision makes HARDER
- **The rename (Decision 7) touches every file naming `PipelineProvider`, `BuiltinProvider`, or `FilesystemPipelineProvider`** — mechanical, but real, and every doc comment referencing the old names needs the same pass.
- **Decision 2's "documented migration note" obligation** means a future flag/exit-code change can no longer just land — it needs a changelog-visible note, a small but real process cost that didn't exist before.
- **No structured output still means CI scripts that need more than an exit code must scrape text** — Decision 5 does not solve this; it explicitly defers it.

### Reversibility
High for Decisions 1–6 and 8 (ratifying existing behavior, adding a small command, adding documentation-tier vocabulary — nothing here is hard to undo or adjust). Medium for Decision 7 (the rename): reversible in principle, but a second rename later would touch the same files twice.

---

## Migration Strategy

1. Build `/help` in `session.DefaultCommandRegistry` (Decision 6), listing each registered command's name and a one-line description.
2. Rename `engine.PipelineProvider` → `engine.PipelineSource`, `engine.BuiltinProvider` → `engine.BuiltinPipelineSource`, `project.FilesystemPipelineProvider` → `project.FilesystemPipelineSource` (Decision 7), updating every reference and doc comment; no behavior change.
3. Add the exit-code convention (Decision 4) and the pre-1.0 change policy (Decision 2) to [getting-started.md](../04-guides/getting-started.md) or a new short section of [release.md](../04-guides/release.md), so Decision 2–4 are documented somewhere a user or CI author actually reads, not only in this ADR.
4. Add **Session** and **Slash Command** (Decision 8) to [terminology.md](../05-reference/terminology.md)'s Mechanism section.

No data migration; no change to any Act, Pipeline, or Record format.

---

## Future ADR Dependencies

- **Routing & policy** (backlog, ADR-0006) and **Extension isolation & contract versioning** (backlog, ADR-0008): neither inherits a hard constraint from this ADR, but a future third-party extension surface that adds its own CLI command or slash command would need to fit within Decision 2/3's pre-1.0 change policy, or explicitly supersede it.

---

## Open Questions

1. **Should exit codes eventually get a distinct code per failure class** (verification failure vs. declined approval vs. config error vs. repo-not-found)? Left open until a real CI consumer needs to distinguish them programmatically rather than by scraping text.
2. **Should a `--json` output mode ever be built**, and if so, on which commands first? Not decided here — revisit against a real consumer need, per Decision 5.
3. **Does a project-authored custom slash command need a name-collision policy** against the default registry, mirroring [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 6's global-uniqueness rule for Pipeline names? Not decided here; today's `CommandRegistry.Register` behavior on collision was not audited as part of this ADR.
4. **Should RFC-0003's originally-sketched `/status`, `/pipeline`, `/approve`, `/reject` commands ever be built?** Left to a real need surfacing, per Decision 6 — not a commitment this ADR makes.

---

## Review Checklist

To be completed at ratification:

- [ ] **No contradiction with accepted documents.** Confirm against [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) (Decision 7's naming precedent — "Source" as a fresh, not-yet-canonical noun — is consistent with how ADR-0004 handled "reusable Act template") and [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)/[ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) (no overlap).
- [ ] **Decision 6 (`/help`) and Decision 7 (rename) are actually implemented** before being marked shipped in [implementation-status.md](../00-overview/implementation-status.md).
- [ ] **`release.md`'s disclaimer ("intended, not ratified") is updated** once this ADR is accepted, since it names this exact ADR as the blocker.
- [ ] **Terminology additions (Decision 8) do not collide** with any existing term in [terminology.md](../05-reference/terminology.md) — confirmed: no prior entry for Session or Slash Command exists.
- [ ] **Process caveat resolved.** Ratify under [ADR-0000](ADR-0000-governance-and-ratification-process.md); update this Status row and the backlog table in [README.md](README.md) in the same ratifying commit.

---

_This ADR formally ratifies the interactive session as Foundry's primary interface and the flag CLI as a secondary, CI-only surface; ratifies a pre-1.0 "change freely, but never silently" policy for both the flag CLI and the slash-command vocabulary per release.md's own stated intent; declines to build a machine-readable output contract before a real consumer needs one; and closes two concrete, previously-flagged gaps — a phantom `/help` reference and the retired term still live in `PipelineProvider` — rather than leaving either to be rediscovered later._
