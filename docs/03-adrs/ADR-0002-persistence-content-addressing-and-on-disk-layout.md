# ADR-0002 — Persistence, Content-Addressing & On-Disk Layout

| | |
|---|---|
| **Status** | **Proposed** — drafted per the ADR backlog ([README.md](README.md)) and [ADR-0001](ADR-0001-language-and-toolchain.md)'s own forward reference; not yet ratified. |
| **Date** | Drafted 2026-07-20 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | The ADR backlog entry named in [README.md](README.md) ("Persistence, content-addressing & on-disk layout") — Record durability, hash/canonicalization, and what is committed vs. cached. |
| **Gates** | [docs/02-architecture/trust.md](../02-architecture/trust.md)'s "Unresolved: the durability classification of the Record itself"; [roadmap.md](../00-overview/roadmap.md) open decision 6; [invariants.md](../05-reference/invariants.md) I8's "pending owning ADR" note; [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md); [RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §4.5/§10's "the Record's on-disk shape is a compatibility surface with no owning ADR yet." Also corrects [ADR-0001](ADR-0001-language-and-toolchain.md) clause 2's specific pre-commitment (see Context). |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. This is the ADR ADR-0001 itself named "ADR-0002 (Persistence)" and forward-referenced — ratifying it also resolves that ADR's own named dependency, per its Future ADR Dependencies section. |

---

## Context

**A real contradiction this ADR must resolve, not just describe.** [ADR-0001](ADR-0001-language-and-toolchain.md) (Accepted, 2026-06-29 — before M0 was built) pre-commits this exact ADR to a specific technical direction: "clause 2... is the reason ADR-0002 must select a pure-Go storage driver" and its Consequences section states "ADR-0002 (persistence): a mature pure-Go SQLite implementation exists, so the storage engine can be embedded while preserving R1." **No such storage engine exists in the codebase, and none was ever built.** What was actually shipped — `record.FileStore` (`record/store.go`) — is one flat JSON file per Act (`<root>/<act.ID>/act.json`), written via `os.OpenFile` with `O_EXCL`; no database, no query engine, no embedded SQLite anywhere in the module graph (confirmed: no `sqlite`/database import exists in `go.mod` or any `.go` file).

This is not an oversight to quietly correct — [principles.md](../00-overview/principles.md)'s **Filesystem-first persistence** principle, which sits at a *higher* precedence tier than any ADR per AGENTS.md's documentation hierarchy (`00-overview` outranks `03-adrs`), already settles this the other way: *"The canonical storage format for Foundry is the project's filesystem. Every durable artifact (Acts, Knowledge, Evidence, configuration, and history) is represented as versionable files. Databases, indexes, caches, and remote services are optional derived storage layers and must never become the canonical source of truth."* ADR-0001's SQLite pre-commitment was written against the archived pre-M0 architecture's heavier design (a "kernel," a database-backed manifest); the actual, shipped, higher-precedence-grounded design is flat files. AGENTS.md's own conflict-resolution rule is explicit: *"When two active documents disagree, the higher one wins; the lower must be updated... never leave a contradiction."* This ADR is that update — the same treatment [README.md](README.md)'s Accepted table already gives ADR-0001's *other* stale pre-commitment (an extension-isolation mechanism it "is not its to decide," flagged as a pending amendment there).

**What is already built, and this ADR ratifies rather than invents:**

- `record.FileStore` (`record/store.go`): `Write` persists one Act as `<root>/<act.ID>/act.json`, JSON-encoded via `json.MarshalIndent` (`record/json.go`), refusing a second write for the same ID (`ErrAlreadyExists`) — immutable once written, matching [I8](../05-reference/invariants.md) exactly. `Read`/`List` decode the same files back; `List` sorts by `CreatedAt` then `ID`.
- `record.CheckpointStore` (`record/checkpoint.go`): a **separate**, mutable, overwritable `<root>/<act.ID>/checkpoint.json`, sharing `FileStore`'s root but never read or written by `FileStore` itself. `Save` overwrites; `Delete` removes it once an Act reaches a terminal disposition; `List` returns every Act with a surviving checkpoint (the resumable set). This is exactly [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md)'s "Alternative 1," already implemented, with that question still formally marked OPEN.
- Neither store computes, stores, or checks a content hash anywhere. `domain.StepRecord.Produced` (a Generate Step's output) stores a Patch's raw text inline (`engine/steps.go`'s `producedPatch`) — an Artifact's identity today is "whatever Act and Step index it lives at," not a hash of its own content, despite [terminology.md](../05-reference/terminology.md)'s Artifact definition ("identity is its content").
- No project convention exists yet for whether `.foundry/acts/` should be committed to a project's own git repository. This repository's own `.foundry/pipelines/*.json` already are (`git ls-files .foundry/` confirms it); `.foundry/acts/` does not yet exist in this repository (no Acts have been produced against Foundry's own repo), so there is no existing practice to contradict either way — a genuinely open, concrete question this ADR settles.

This ADR does not resolve **Replay & determinism contract** (backlog, next in dependency order) — cross-version replay scope and validator-determinism strength are that ADR's question (and [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)/[OQ-004](../06-open-questions/OQ-004-validator-determinism.md)'s), not this one's, even though `act.json`'s on-disk shape (this ADR's concern) is what a future cross-version replay would have to read. Nor does it resolve **Knowledge & semantic store** (backlog, ADR-0007) — `.foundry/knowledge/`'s own persistence and format-stability question belongs there.

---

## Decision

1. **`record.FileStore`'s on-disk layout is ratified as the Record's entire persistence mechanism.** One flat, human-readable JSON file per Act at `<root>/<act.ID>/act.json`, written exactly once (`O_EXCL`), immutable thereafter. No database, embedded or otherwise, is added. This promotes an already-shipped mechanism into a governed decision under [ADR-0000](ADR-0000-governance-and-ratification-process.md), the same move [ADR-0006](ADR-0006-routing-and-policy.md) Decision 1 made for the Router.

2. **This explicitly corrects [ADR-0001](ADR-0001-language-and-toolchain.md) clause 2's specific pre-commitment** ("ADR-0002 must select a pure-Go storage driver... a mature pure-Go SQLite implementation... the storage engine can be embedded"). Per [principles.md](../00-overview/principles.md)'s Filesystem-first persistence principle — higher precedence than any ADR — flat versionable files, not an embedded database, are Foundry's canonical storage format. ADR-0001 itself remains Accepted and otherwise unamended; only this one forward-referenced clause is superseded, exactly as ADR-0001 invited ("ADR-0002 must select..." — it does, and selects none).

3. **`record.CheckpointStore` is ratified as the sole mechanism for an Act's in-progress (not-yet-terminal) state, closing [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md).** A checkpoint (`checkpoint.json`) is explicitly **not** the Record: mutable, overwritable, deleted once an Act reaches a terminal disposition (pass, exhausted repair, rejected, budget-exceeded, or — per [the apply-failure fix](../00-overview/implementation-status.md) — apply-failed). This promotes OQ-008's already-implemented "Alternative 1" from a documented recommendation to a ratified decision.

4. **The Record is durable, not a disposable cache — ratifying [I8](../05-reference/invariants.md) with a concrete, practical consequence: a project's `.foundry/acts/` directory should be committed to that project's own git repository by default**, exactly the way this repository's own `.foundry/pipelines/*.json` already are. This follows directly from [principles.md](../00-overview/principles.md)'s "every durable artifact... is represented as *versionable* files" — Foundry itself enforces nothing here (no code change; see Migration Strategy), the same way committing `.foundry/pipelines/` is convention, not a mechanism Foundry's code checks.

5. **`checkpoint.json` is explicitly cache-like, not required to be committed** — unlike `act.json`, it has no audit value once superseded or deleted, and a project may gitignore `**/checkpoint.json` without losing anything durable. This draws the "what is committed vs. cached" line the backlog names precisely at the `FileStore`/`CheckpointStore` boundary that already exists in code.

6. **No content-addressing or hashing is added now.** `Act.ID` (a random, unique 16-hex-character identifier, `domain.NewAct`) is sufficient identity for everything the Record does today — audit, replay, `foundry show`/`log`/`replay`. Building real content-addressed storage for a Patch or other Artifact speculatively, with no consumer that needs content identity independent of its owning Act, repeats the exact premature-generality mistake [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) and [ADR-0006](ADR-0006-routing-and-policy.md) already declined elsewhere. Revisit only once a real need exists — e.g., Derived Knowledge deduplication, or an Artifact genuinely shared/referenced across more than one Act.

7. **Today's JSON encoding is ratified as already sufficiently canonical for `domain.Act`'s current shape.** `json.MarshalIndent` produces a deterministic byte sequence for a given Act value because `Act` and its nested types (`StepRecord`, etc.) contain only structs and slices — never a Go map, whose key iteration order is not guaranteed. No canonicalization work is needed today. This becomes a live question again only if a future field adds a genuinely unordered key set (a `map[string]T`) to `domain.Act` itself — not the case anywhere today.

8. **`act.json` carries no version field, mirroring [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 3's identical reasoning for the Pipeline-document format** — additive-only evolution (RFC-0002 §4.5's own Step-trace addition is the precedent) until a second, genuinely independent reader of `act.json` exists. An old Act being `foundry show`n or replayed by a newer build of this same codebase is not that second reader — it is the scope **Replay & determinism contract** (next in the backlog) owns explicitly; this ADR only rules that the storage format itself stays unversioned and additive for now, not on cross-version replay's honest guarantee.

---

## Alternatives Considered

### Embed a pure-Go SQLite (or similar) database, as ADR-0001 originally anticipated
- **For:** Indexed queries (`foundry log` by date range, verdict, or Intent substring) without a full directory scan; atomic multi-record transactions if the Record's shape ever needs them.
- **Against:** Directly contradicts [principles.md](../00-overview/principles.md)'s ratified, higher-precedence Filesystem-first mandate. Trades a human-readable, `grep`-able, `git diff`-able plain-JSON file for an opaque binary blob — a real loss for a project whose own audit/portability story depends on "readable independently of any model or vendor" ([I11](../05-reference/invariants.md), stated for Knowledge but the same value applies here). No concrete query need exists at today's scale (a single project's Acts, produced by one user or a small team) to justify the complexity.
- **Verdict:** Rejected. Ratified instead as Decisions 1–2 — flat files, correcting ADR-0001's stale assumption rather than inheriting it silently.

### Build real content-addressed Artifact storage now
- **For:** Matches terminology.md's Artifact definition literally; would let a future Derived Knowledge indexer deduplicate identical Patches across Acts for free.
- **Against:** No consumer reads or needs a content hash anywhere in the codebase today — building it speculatively is the exact "no consumer, no schema" trap [ADR-0009](ADR-0009-cli-and-output-contract.md) named for `--json` and [ADR-0006](ADR-0006-routing-and-policy.md) named for capability-based negotiation.
- **Verdict:** Rejected for now. Ratified instead as Decision 6 — revisit once a real need (Derived Knowledge dedup, cross-Act Artifact sharing) exists.

### Gitignore `.foundry/acts/` by default, treating it as a local-only build cache
- **For:** Keeps a project's git history focused on source code; avoids Act patch/Evidence text growing the repository forever.
- **Against:** Directly contradicts [I8](../05-reference/invariants.md) ("never a disposable cache") and [principles.md](../00-overview/principles.md)'s "versionable files" mandate — a project that gitignores its own Record has no backup of its audit trail beyond whatever separate discipline it invents, exactly the failure mode "durable, not a cache" exists to prevent.
- **Verdict:** Rejected. Ratified instead as Decision 4 — commit `.foundry/acts/`, the same convention `.foundry/pipelines/` already follows.

### Merge the checkpoint into `act.json` itself (a mutable "draft" phase in the Record)
- **For:** One file per Act instead of two; no separate store type to maintain.
- **Against:** This is [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md)'s own Alternative 2, already argued against there: it blurs "recorded" (immutable, trusted) with "in progress" (mutable, not yet trusted), and risks a caller accidentally relying on a "recorded" Act that can still change underneath it — the exact failure "write-once" exists to prevent.
- **Verdict:** Rejected. Ratified instead as Decision 3 — a separate `CheckpointStore`, already built.

### Add a version field to `act.json` now
- **For:** `act.json` genuinely is read across builds of this same codebase (unlike a Pipeline document, which is always freshly authored) — arguably a stronger case for versioning now than [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) had for the Pipeline-document format.
- **Against:** Cross-version replay's *honest guarantee* is explicitly the next backlog ADR's question (Replay & determinism contract, [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)) — deciding a version field's mechanics here, before that ADR decides what guarantee it is even meant to support, risks designing the field around the wrong scope.
- **Verdict:** Rejected here, deliberately deferred to **Replay & determinism contract** — flagged explicitly (Decision 8, Future ADR Dependencies), not silently dropped.

---

## Consequences

### What this decision makes EASIER
- **A real, previously-unnoticed contradiction between an Accepted ADR and this project's own higher-precedence principles is corrected**, rather than left for a future contributor to discover and be misled by.
- **[OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md) is closed** — the checkpoint/Record split is now a ratified decision, not only a documented recommendation.
- **[I8](../05-reference/invariants.md)'s "durable, not a cache" gets a concrete, actionable consequence** (commit `.foundry/acts/`) instead of remaining an abstract principle with no practical guidance.
- **[RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §4.5/§10's "no owning ADR yet" gap for the Record's on-disk shape is closed.**
- **Replay & determinism contract (next in the backlog) inherits a settled storage format** to reason about cross-version replay against, rather than an implicit one.

### What this decision makes HARDER
- **Nothing structurally** — like [ADR-0006](ADR-0006-routing-and-policy.md), this mostly ratifies already-shipped behavior and declines speculative work.
- **Repository growth over time** if `.foundry/acts/` is committed as recommended — every Act's Patch and Evidence text lives forever in git history. A real, named cost, not hidden.
- **`record.FileStore.List` remains a full directory scan and full decode of every Act on every call** (`record/store.go`) — acceptable at today's scale (a single project's Acts) but a genuine limitation this ADR does not solve, since Decision 1 declines to add any indexing mechanism. Named honestly here rather than left implicit.

### Reversibility
High for Decisions 1, 3, 6, 7, and 8 (ratifying already-shipped behavior, declining speculative work) — nothing to unwind. Medium for Decisions 4–5 (the commit-vs-gitignore recommendation): a soft, guide-level convention Foundry's code does not enforce, so any project remains free to choose differently, but reversing an *already-committed* history of Acts is not itself undoable (git history, once pushed, is not easily rewritten).

---

## Migration Strategy

No code changes; no data migration. This ADR ratifies `record.FileStore` and `record.CheckpointStore` exactly as already implemented and tested.

1. Correct [ADR-0001](ADR-0001-language-and-toolchain.md)'s Future ADR Dependencies line for ADR-0002 (Decision 2) — it must stop reading as though an embedded database was chosen.
2. Add a `.gitignore` recommendation and a one-line commit-to-git recommendation for `.foundry/acts/` to [getting-started.md](../04-guides/getting-started.md), near its existing `.foundry/acts/` mention.
3. Add a "Record format & persistence" subsection to [release.md](../04-guides/release.md)'s "Compatibility surfaces gated by release" section (mirroring [ADR-0009](ADR-0009-cli-and-output-contract.md)'s own CLI subsection), and update its closing disclaimer.
4. Resolve [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md): update its own Status section and [docs/06-open-questions/README.md](../06-open-questions/README.md)'s index row, carrying its two remaining sub-questions forward into this ADR's Open Questions.
5. Update [roadmap.md](../00-overview/roadmap.md) open decision 6 (Record durability classification) to RESOLVED, mirroring decision 1's own strikethrough treatment.
6. Update [invariants.md](../05-reference/invariants.md)'s closing note to remove I8 from the "pending owning ADR" list.
7. Update [README.md](README.md): move this row from Backlog to Accepted; add a corrective note to ADR-0001's own Accepted-table row (mirroring its existing "pending amendments" note for the extension-isolation pre-commitment).
8. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR section and changelog.

---

## Future ADR Dependencies

- **Replay & determinism contract** (backlog, next in dependency order): inherits this ADR's ratified on-disk format (Decisions 1, 7, 8) as the concrete bytes any cross-version replay guarantee would have to read; must explicitly decide whether `act.json` ever needs a version field, rather than silently inheriting Decision 8's "not yet."
- **Knowledge & semantic store** (backlog, ADR-0007): does not inherit this ADR's specific commit-vs-cache line (Decisions 4–5) automatically — `.foundry/knowledge/`'s own durability classification is that ADR's question, though it may reasonably reach the same conclusion by the same reasoning.
- **Cost as a first-class constraint** (backlog, ADR-0011): no dependency.

---

## Open Questions

Carried forward from [OQ-008](../06-open-questions/OQ-008-in-progress-act-persistence.md), not resolved here:

1. **Should the checkpoint's on-disk shape ever be promoted directly into `act.json`**, or does the terminal write always re-derive from the finished in-memory Act? Today: the latter — the checkpoint is discarded, not renamed. Revisit only if a concrete reason to preserve checkpoint history emerges.
2. **Does a future multi-Pipeline-attempt (e.g. resuming across a repair boundary) change which alternative is right?** Not resolved here.

New to this ADR:

3. **At what scale does `record.FileStore.List`'s full-directory-scan become a real problem**, and what's the minimal fix (an index file? pagination?) when it does? Not decided speculatively here — Decision 1 declines to build one now.

---

## Review Checklist

To be completed at ratification:

- [ ] **No contradiction with accepted documents**, beyond the one this ADR deliberately corrects (ADR-0001 clause 2). Confirm against [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) (JSON-only wire format precedent, consistent) and [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)/[ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) (no overlap).
- [ ] **ADR-0001's corrected clause reads accurately** once amended — it must not claim an embedded database was chosen, nor silently drop the fact that it once said so.
- [ ] **Decisions 1, 3, and 7 verified against the actual shipped code** — `record/store.go`, `record/checkpoint.go`, `record/json.go`, `domain/act.go` (confirming no map fields) — re-read at ratification to confirm nothing has drifted since drafting.
- [ ] **OQ-008's two sub-questions are carried forward accurately**, not silently dropped, in this ADR's own Open Questions.
- [ ] **Process caveat resolved.** Ratify under [ADR-0000](ADR-0000-governance-and-ratification-process.md); update this Status row and the backlog table in [README.md](README.md) in the same ratifying commit.

---

_This ADR ratifies `record.FileStore` and `record.CheckpointStore` exactly as already shipped — flat, human-readable, versionable JSON files, no embedded database, no content-addressing — while explicitly correcting ADR-0001's stale pre-commitment to an embedded SQLite storage driver, a contradiction with principles.md's higher-precedence Filesystem-first mandate that predates this project's actual architecture. It closes OQ-008, gives I8's durability guarantee a concrete practical consequence (commit the Record to git), and declines both content-addressing and a version field until a real, named need for either exists._
