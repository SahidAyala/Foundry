# ADR-0007 — Knowledge & Semantic Store

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-21 under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s governance process, the same day it was drafted. |
| **Date** | Drafted 2026-07-21; ratified 2026-07-21 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | The ADR backlog entry named in [README.md](README.md) ("Knowledge & semantic store") — Authored-Knowledge note schema and format stability, `.foundry/knowledge/`'s durability classification, and whether/when Derived Knowledge indexing and semantic retrieval get built. |
| **Gates** | [knowledge.md](../02-architecture/knowledge.md)'s "Unresolved: the format stability and cross-version migration of Authored Knowledge" callout; [roadmap.md](../00-overview/roadmap.md) open decision 7; [invariants.md](../05-reference/invariants.md) I11's "pending owning ADR" note; [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 and [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md), both of which name this ADR as the thing they feed but explicitly do not ratify. |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s own Future ADR Dependencies section notes that its commit-vs-cache line for `.foundry/acts/` does **not** automatically extend to `.foundry/knowledge/` — this ADR re-derives the equivalent conclusion independently (Decision 6) rather than silently inheriting it. |

---

## Context

**What is already built, and this ADR ratifies rather than invents:**

- `workspace.KnowledgeNoteApplier` (`workspace/knowledge_applier.go`, [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6, shipped): writes one new Markdown file per Act to `.foundry/knowledge/<act-id>-<slug>.md` — `# <Intent>\n\nAct: <act-id>\n\n<content>\n`. No front-matter, no tags, no structured metadata. It never edits an existing note; a correction or update is a new file, never a rewrite.
- `workspace.ProjectDocApplier` (same file): a second, distinct target — appends prose to one human-facing document named by `.foundry/config.json`'s `docs_path`. [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §2.6 already scoped this **out** of retrieval — it is a document a person reads, not a corpus an Executor's Context draws from.
- `knowledge.Gatherer` (`knowledge/gatherer.go`, [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md), shipped): the read side. A full directory scan of `.foundry/knowledge/` per Act, naive lexical word-overlap matching against the Intent (no embeddings, no index), bounded by `maxNotes` (3) and `maxContextBytes` (20 KiB), each result attributed to its source file (`".foundry/knowledge/<name>.md:\n<content>"`) — satisfying [I7](../05-reference/invariants.md)'s provenance requirement with zero new data shape. Composed alongside `gatherer.NaiveGatherer` via `gatherer.Compose`.
- Neither Applier nor Gatherer parses a note back into any typed structure — `os.ReadFile` and string/regex matching are the entire mechanism. There is no `json.Unmarshal`, no front-matter decoder, nothing that could reject an old note as an "unknown field" the way `engine.DecodePipelineDocument` does for Pipeline documents ([ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 4).

**What both shipping RFCs explicitly named and explicitly left to this ADR** ([RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §3, verbatim in spirit):

1. Authored Knowledge note schema and versioning — no front-matter, tags, or structured metadata exists; whether any should is undecided.
2. Semantic / embeddings-based retrieval — declined so far, revisit only "if a real project's corpus grows large enough that keyword matching demonstrably misses relevant notes."
3. Derived Knowledge (an index or cache over Authored notes) — [knowledge.md](../02-architecture/knowledge.md)'s own Derived layer is not built; a full-directory scan is the entire mechanism today.
4. Curating or superseding stale/contradictory notes — nothing resolves what happens when two retrieved notes disagree.
5. Whether Knowledge or Act is the domain center ([OQ-001](../06-open-questions/OQ-001-domain-center.md)) — this ADR's decisions are agnostic to that question's resolution, exactly as RFC-0005 already argued; the store, its format, and its durability classification work identically whichever pole eventually wins.

**A gap ADR-0002 explicitly left open for here:** [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) ratified `.foundry/acts/` as durable and committed to a project's own repository, but its own Future ADR Dependencies section states this does **not** automatically extend to `.foundry/knowledge/` — "though it may reasonably reach the same conclusion by the same reasoning." That reasoning is re-derived independently below (Decision 6), not assumed.

**An honesty note on this ADR's own backlog title:** [README.md](README.md) calls this entry "Knowledge & semantic store." This ADR does not deliver a semantic store — it decides *whether one is needed yet* (it is not) and names the trigger for when that changes. Flagged explicitly rather than silently shipping less than the title promises, the same discipline [ADR-0006](ADR-0006-routing-and-policy.md) applied to its own "capability-based negotiation" scope cut.

This ADR does not resolve **Extension isolation & contract versioning** (backlog, ADR-0008) or **Cost as a first-class constraint** (backlog, ADR-0011) — neither is touched here.

---

## Decision

1. **Plain, unparsed Markdown prose is ratified as Authored Knowledge's entire note format — no front-matter, no tags, no structured metadata.** `workspace.KnowledgeNoteApplier`'s exact shipped shape (`# <Intent>\n\nAct: <act-id>\n\n<content>\n`) is the format. This is not a placeholder awaiting a schema; it is the deliberate, ratified answer, for the same "no consumer, no schema" reason [ADR-0006](ADR-0006-routing-and-policy.md) and [ADR-0009](ADR-0009-cli-and-output-contract.md) each declined their own speculative surface: nothing today reads a note as anything other than bytes to lexically match, so there is no field a schema would serve.

2. **This closes the format-stability question, not by deferring it but by observing that unstructured text has nothing to destabilize.** [knowledge.md](../02-architecture/knowledge.md)'s "Unresolved: format stability and cross-version migration" callout presumed a schema that could break across versions, the way `act.json` (guarded by [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)) or a Pipeline document (guarded by [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md)) could. A plain Markdown file, read only as text, cannot fail to decode — any future Foundry version, any external tool, any human with a text editor can always read it, which is [I11](../05-reference/invariants.md)'s "readable independently of any model or vendor" satisfied about as literally as possible. **If a future ADR ever adds machine-parsed structure to a note** (front-matter, a schema), *that* ADR must specify additive-only evolution with a descriptive, doc-linking rejection of unrecognized structure, mirroring [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) Decision 4's precedent exactly — this ADR does not build that structure now, so it cannot yet need that guard.

3. **Derived Knowledge indexing is explicitly declined for now, closing this ADR's namesake gap with a decision, not an ellipsis.** [knowledge.md](../02-architecture/knowledge.md)'s Derived layer (recomputable, disposable, never the source of truth — [I9](../05-reference/invariants.md)) remains conceptual. `knowledge.Gatherer`'s full-directory scan is ratified as the entire retrieval mechanism until a **named trigger** fires: a real project's `.foundry/knowledge/` corpus grows large enough that the scan's cost is measured and material, or that naive lexical matching demonstrably misses relevant notes a human can point to. Neither has happened yet — [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §5 named exactly this as an accepted, not-yet-triggered risk. **If Derived Knowledge is ever built, it must satisfy [I9](../05-reference/invariants.md) literally: fully recomputable from Authored notes alone, safely deletable with no information loss, never itself queried as if it were a source of truth** — a guardrail on the *eventual* mechanism, not a design for one now.

4. **Semantic (embeddings-based) retrieval is explicitly declined for now, for the same named-trigger reason as Decision 3**, plus one more: an embedding call would make Context *retrieval itself* depend on a live model invocation, a materially bigger kind of non-determinism than a filesystem read — [system-context.md](../02-architecture/system-context.md)'s determinism boundary already classifies Context Sources as the non-deterministic edge, but this would deepen that edge's dependency on substrate in a way worth naming, not quietly accepting, given [I12](../05-reference/invariants.md)'s "the model is substrate, never a domain concept." If semantic retrieval is ever built, it should compose alongside naive lexical matching via the existing `gatherer.Compose` seam, not replace it outright, unless a future decision argues otherwise.

5. **No mechanism for curating, superseding, or resolving contradictory notes is built.** Notes remain append-only, matching [I10](../05-reference/invariants.md) ("project state is never silently mutated"). A human, or an Act, may reference an earlier Act's ID or note filename in a new note's prose as an informal convention — Foundry tracks no formal supersession relationship, and disagreement between two retrieved notes is left to whichever Executor or human reads them, exactly as [knowledge.md](../02-architecture/knowledge.md) already states.

6. **`.foundry/knowledge/` is ratified as durable and should be committed to a project's own git repository by default — the same conclusion [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) reached for `.foundry/acts/`, re-derived here independently rather than assumed.** A Knowledge note is the Outcome of an accepted Act ([knowledge.md](../02-architecture/knowledge.md): "a change to Authored Knowledge is an Outcome of an Act, reviewed and accepted by an Authority like any other change"); gitignoring it would contradict [I9](../05-reference/invariants.md) (source of truth, not a cache) and [I11](../05-reference/invariants.md) (a project owns and can export its Authored Knowledge) exactly as it would for the Record. No code enforces this — it is a convention, the same way committing `.foundry/pipelines/` and (per ADR-0002) `.foundry/acts/` already are conventions Foundry's code does not check.

7. **`workspace.ProjectDocApplier`'s target remains outside Authored Knowledge's retrieval corpus, ratifying [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §2.6's scope boundary as a decision rather than only a proposal.** The `project-doc` file is human-facing documentation Foundry happens to append to, not Authored Knowledge in the technical sense this ADR governs — it is not retrieved, not attributed, not bound by Decisions 1–6 above.

8. **Provenance scoring — ranking retrieved notes by source reliability or recency beyond `knowledge.Gatherer`'s existing overlap-score ordering — is out of scope, for the same no-consumer reason as Decisions 3–4.** Today's provenance guarantee ([I7](../05-reference/invariants.md)) is satisfied by attribution (every retrieved note names its source file); *weighting* sources against each other is a distinct, unbuilt idea this ADR does not design.

---

## Alternatives Considered

### Add YAML front-matter (tags, dates, topics) to notes now
- **For:** Would enable future filtering, browsing, or a `foundry knowledge list` command without full-text scanning; matches how many note-taking/knowledge-base tools structure entries.
- **Against:** No consumer reads a structured field anywhere today — `knowledge.Gatherer` matches raw text, not a parsed schema. Adding one now is the same "no consumer, no schema" trap [ADR-0006](ADR-0006-routing-and-policy.md) and [ADR-0009](ADR-0009-cli-and-output-contract.md) each already declined, and it would create a new decode surface this ADR would then have to version prematurely.
- **Verdict:** Rejected. Ratified instead as Decision 1 — plain, unparsed prose.

### Build a Derived Knowledge index now (an inverted index file, or an embedded search engine)
- **For:** Would remove the full-directory-scan cost [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §5 already named as a real, accepted risk; would make browsing/listing notes possible without reading every file.
- **Against:** No project has yet grown a corpus large enough to make that scan cost real or measured. An index or embedded search engine is itself a Derived-Knowledge cache ([I9](../05-reference/invariants.md)) — building one speculatively, with no concrete performance or UX need naming its exact shape, repeats the premature-generality mistake [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) corrected for the Record (an anticipated embedded SQLite driver that was never actually needed).
- **Verdict:** Rejected for now. Ratified instead as Decision 3 — declined with a named trigger, not left ambiguous.

### Build semantic (embeddings-based) retrieval now
- **For:** Would improve match quality as a corpus grows more varied in phrasing than exact keyword overlap can catch.
- **Against:** [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §3 already named and declined this; no real corpus exists yet to demonstrate lexical matching's insufficiency. It also introduces a live model dependency into Context retrieval itself — a new, deeper kind of non-determinism worth naming under [I12](../05-reference/invariants.md), not a free upgrade.
- **Verdict:** Rejected for now. Ratified instead as Decision 4.

### Build a formal note-supersession mechanism (a field naming an older note as superseded)
- **For:** Directly addresses the real, named gap of what happens when two retrieved notes disagree.
- **Against:** Same no-consumer-no-schema reasoning as front-matter. [knowledge.md](../02-architecture/knowledge.md) already states curation is left to a human or a future deliberate Act, not a mechanism Foundry enforces — designing a supersession graph now is designing for a corpus scale and curation workflow that does not exist yet.
- **Verdict:** Rejected. Ratified instead as Decision 5 — append-only, informal cross-referencing only, no enforced mechanism.

### Gitignore `.foundry/knowledge/`, treating it as local/derived
- **For:** Keeps a project's git history focused on source code and Acts; avoids Knowledge notes growing the repository forever.
- **Against:** Directly contradicts [I9](../05-reference/invariants.md) (Authored Knowledge is the source of truth, not a cache) and [I11](../05-reference/invariants.md) (a project owns and can export its Authored Knowledge) — the identical reasoning [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) applied to `.foundry/acts/`.
- **Verdict:** Rejected. Ratified instead as Decision 6.

---

## Consequences

### What this decision makes EASIER
- **[knowledge.md](../02-architecture/knowledge.md)'s "Unresolved: format stability" callout is resolved** with a concrete decision — unstructured prose has nothing to destabilize — instead of remaining "intended-but-unproven."
- **[roadmap.md](../00-overview/roadmap.md) open decision 7 and [I11](../05-reference/invariants.md)'s "pending owning ADR" note are closed.**
- **`.foundry/knowledge/` gets the same concrete commit-to-git guidance `.foundry/acts/` already has** ([ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)), closing the gap that ADR explicitly left for here.
- **A future Derived Knowledge or semantic-retrieval effort, if the named triggers ever fire, inherits explicit guardrails** ([I9](../05-reference/invariants.md) recomputability, composition via `gatherer.Compose`) instead of starting from nothing.
- **[RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 and [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) both get the ratification they explicitly deferred to this ADR.**

### What this decision makes HARDER
- **Nothing structurally** — like [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md), [ADR-0003](ADR-0003-replay-and-determinism-contract.md), and [ADR-0006](ADR-0006-routing-and-policy.md), this mostly ratifies already-shipped behavior and declines speculative work.
- **Corpus growth cost remains a real, accepted, unsolved limitation** ([RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §5) — the full-directory scan stays the mechanism until a named trigger fires, not solved by this ADR.
- **Retrieval quality (naive lexical matching) remains a real, accepted limitation**, unchanged from RFC-0005.
- **[roadmap.md](../00-overview/roadmap.md) M4 remains "Partial"** after this ADR — the *policy* questions this ADR owns are resolved, but Derived Knowledge, semantic retrieval, and provenance scoring remain unbuilt code, exactly as Decisions 3, 4, and 8 rule they should stay until their own triggers fire.

### Reversibility
High. Every decision here either ratifies already-shipped, already-tested behavior (Decisions 1, 2, 7) or declines to build something new with a named trigger for revisiting it (Decisions 3, 4, 5, 8) or extends an existing convention by identical reasoning (Decision 6) — nothing to unwind later.

---

## Migration Strategy

No code changes; no data migration. This ADR ratifies `workspace.KnowledgeNoteApplier`, `workspace.ProjectDocApplier`, and `knowledge.Gatherer` exactly as already implemented and tested, and declines new work with named triggers.

1. Update [knowledge.md](../02-architecture/knowledge.md)'s "Unresolved (human decision required)" section: replace the "intended-but-unproven" framing with a citation to this ADR and Decision 2's reasoning (unstructured prose has nothing to destabilize).
2. Add a `.foundry/knowledge/`-should-be-committed recommendation to [getting-started.md](../04-guides/getting-started.md), mirroring [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s existing `.foundry/acts/` recommendation there.
3. Update [roadmap.md](../00-overview/roadmap.md) open decision 7 ("Authored-knowledge format stability & migration") to RESOLVED, mirroring decisions 4–6's existing strikethrough treatment.
4. Update [invariants.md](../05-reference/invariants.md)'s closing note to remove I11 from the "pending owning ADR" list.
5. Update [README.md](README.md): move this row from Backlog to Accepted upon ratification.
6. Update [RFC-0004](../01-rfcs/RFC-0004-multi-executor-router-and-publish-policy.md) §4's "Knowledge & semantic store" row and [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) §4's equivalent row to note this ADR as Accepted rather than backlog/proposed.
7. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR section, its M4 row (noting the policy questions are now resolved even though M4 remains code-partial), and its changelog.

---

## Future ADR Dependencies

- **Extension isolation & contract versioning** (backlog, ADR-0008): no dependency — Context Sources are already an established third-party extension point ([extensibility.md](../02-architecture/extensibility.md)); this ADR does not change how a third-party Context Source, semantic or otherwise, would be isolated or versioned.
- **Cost as a first-class constraint** (backlog, ADR-0011): no direct dependency, though a future Derived Knowledge index or semantic retrieval call, if either is ever built per Decisions 3–4's triggers, would introduce new infrastructure or per-call cost that ADR might eventually want to weigh — named, not decided here.

---

## Open Questions

Carried forward, not resolved here:

1. **At what concrete scale** (note count, total corpus bytes, or measured scan latency) does the full-directory-scan retrieval mechanism become a real problem worth solving? Not decided speculatively — Decision 3 declines to guess.
2. **What should Derived Knowledge's concrete shape be**, once its trigger fires — an index file format, an invalidation strategy? Left to that future moment.
3. **Should semantic retrieval, if ever justified, replace or compose alongside naive lexical matching?** Decision 4 recommends composition via the existing `gatherer.Compose` seam, but does not bind a future ADR to that choice.
4. **How should curation of contradictory or stale notes eventually work**, if a real project accumulates enough notes for this to matter in practice? Left to human/Act judgment per [knowledge.md](../02-architecture/knowledge.md), not designed here.

---

## Review Checklist

Walked through at ratification (2026-07-21):

- [x] **No contradiction with accepted documents.** Checked against [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) (durability/commit reasoning applied consistently), [ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md) (unknown-field-rejection precedent correctly cited as a future guard, not applied prematurely), and [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)/[ADR-0006](ADR-0006-routing-and-policy.md)/[ADR-0009](ADR-0009-cli-and-output-contract.md) (no overlap).
- [x] **Decisions 1 and 7 verified against the actual shipped code.** Re-read at ratification: `workspace/knowledge_applier.go` (`KnowledgeNoteApplier`'s exact unstructured-prose format; `ProjectDocApplier`'s separate, non-retrieved target) and `knowledge/gatherer.go` (raw-text lexical matching, no decode step) — both match this ADR's description exactly.
- [x] **RFC-0005 §3's five named open items are all addressed here** (schema, semantic retrieval, Derived Knowledge, curation, OQ-001 neutrality) — none silently dropped.
- [x] **Process caveat resolved.** Ratified under [ADR-0000](ADR-0000-governance-and-ratification-process.md); this Status row, [README.md](README.md)'s backlog table, and the Migration Strategy's downstream docs all updated in the same ratifying pass.

---

_This ADR ratifies Foundry's already-shipped Knowledge store exactly as built — unstructured Markdown prose notes under `.foundry/knowledge/`, written once per Act and never rewritten, retrieved by full-directory-scan lexical matching with mandatory source attribution — as the deliberate, sufficient shape for today, not a placeholder awaiting a schema. It closes the format-stability question by observing that unparsed text has nothing to destabilize, extends `.foundry/acts/`'s commit-to-git convention to `.foundry/knowledge/` by identical reasoning, and explicitly declines Derived Knowledge indexing, semantic retrieval, provenance scoring, and formal note curation until each has its own named, concrete trigger — none of them designed speculatively ahead of a real need this codebase has consistently refused to anticipate without one._
