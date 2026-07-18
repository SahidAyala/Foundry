# RFC-0005 — Authored Knowledge Retrieval: Closing the Read Side of Knowledge-Lite Capture

| | |
|---|---|
| **Status** | Draft — Proposed (seeking ratification; a governance process now exists — [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md) — but this RFC has not itself been individually ratified through it) |
| **Authors** | Principal architect review (AI-assisted), for Foundry Core |
| **Reviewers** | _(pending)_ |
| **Supersedes** | — |
| **Superseded by** | — |
| **Created** | 2026-07-15 |
| **Related** | [RFC-0004](RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 (Knowledge-lite capture — this RFC closes its read side, does not reopen its write side), [knowledge.md](../02-architecture/knowledge.md), [execution.md](../02-architecture/execution.md) (lifecycle step 2), [trust.md](../02-architecture/trust.md) (I7 provenance), [system-context.md](../02-architecture/system-context.md) (determinism boundary, Context Sources), [extensibility.md](../02-architecture/extensibility.md) (Context Sources as an open extension point), [terminology.md](../05-reference/terminology.md), [OQ-001](../06-open-questions/OQ-001-domain-center.md), [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md), [roadmap.md](../00-overview/roadmap.md) M4 |

> **What this document is.** RFC-0004 §2.6 shipped "Knowledge-lite capture": an Act can write a durable note to `.foundry/knowledge/`, but nothing ever reads one back. `gatherer/gatherer.go`'s own package doc says as much: Knowledge-based context is "a distinct, unbuilt concern — M4... not a gap in what this package does today." This RFC is that concern, deliberately still narrow: it formalizes `.foundry/knowledge/` as Foundry's actual Authored Knowledge store, and adds one new, naive, provenance-attributed way to retrieve from it into an Act's *considered* Evidence. It is **not** the whole of M4 — Derived Knowledge indexing, semantic retrieval, and Authored-knowledge format stability are explicitly named and explicitly deferred (§3), the same way RFC-0004 §2.6 named Knowledge-lite capture as "explicitly not M4" without preempting it.
>
> **Maturity discipline.** PROVISIONAL and non-canonical, exactly as RFC-0002 through RFC-0004 are treated. It does not resolve [OQ-001](../06-open-questions/OQ-001-domain-center.md) (Act vs. Knowledge as the domain center) or [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md), and it does not silently ratify the "Knowledge & semantic store" ADR backlog entry — see §4.

---

## 0. Executive summary

Foundry's own architecture already promises this: [execution.md](../02-architecture/execution.md)'s lifecycle step 2 is "Gather Evidence (considered) — the Engine assembles Context for the work **from Knowledge and the world**, bounded by Budget." Today only "the world" is real — `gatherer.NaiveGatherer` reads repository files named or implied by an Intent. "From Knowledge" has never been built. Meanwhile, [RFC-0004](RFC-0004-multi-executor-router-and-publish-policy.md) §2.6 shipped a write-only precursor: an `apply` Step can target `knowledge-note`, depositing a Markdown file under `.foundry/knowledge/<act-id>-<slug>.md`. Nothing in the codebase ever reads that directory back. The compounding promise in [knowledge.md](../02-architecture/knowledge.md) — "the project is better positioned for the next Act because of the last one" — is not yet real; it is a write into a directory nothing consults.

This RFC proposes the smallest change that makes it real: a new **Context Source** ([extensibility.md](../02-architecture/extensibility.md) already names these as an open extension point) that retrieves from `.foundry/knowledge/` using the same naive, lexical, no-embeddings approach `NaiveGatherer` already uses for repository files — not semantic search, not an index, not a new note schema. Composed alongside `NaiveGatherer` behind the unchanged `engine.Gatherer` port, a project that has never written a Knowledge note sees no change at all.

**Validated (§1):** `gatherer.NaiveGatherer` performs pure filesystem lexical matching with no Knowledge awareness; `.foundry/knowledge/` is written to (Piece 4) but never read; `engine.Gatherer` is a single-instance port on `Engine`, with no existing composition mechanism for more than one Context Source.

**Not a contradiction, but a real gap:** nothing here overturns [knowledge.md](../02-architecture/knowledge.md)'s Authored/Derived model — it *fulfills* it for the first time by naming a concrete Authored store. The "distinct, unbuilt concern" `gatherer.go` already names is exactly what this RFC builds.

**Recommendation (§6):** four small, independently provable pieces — formalize the store in docs, compose multiple Gatherers, build the naive retrieval Context Source, wire it into both composition roots — none blocking the others, all landable without touching `engine.Strategy`, `runSteps`, or any Step kind.

---

## 1. Current state — what §0's claims rest on

- `gatherer/gatherer.go`'s `NaiveGatherer.Gather` runs three bounded phases over the repository filesystem only: files named in the Intent, an identifier-content fallback, then supplementary README/nearby-directory context — all read via `os.ReadFile` against `repoPath`. Nothing in this file opens `.foundry/knowledge/`.
- `workspace/knowledge_applier.go`'s `KnowledgeNoteApplier` (RFC-0004 §2.6, shipped) writes one new file per Act to `.foundry/knowledge/<act-id>-<slug>.md` — `# <Intent>\n\nAct: <act-id>\n\n<content>\n`. It never edits an existing note (a fresh file per Act, by construction append-only) and nothing reads the directory back afterward.
- `engine.Engine` holds exactly one `gatherer Gatherer` field (`NewEngine`'s first parameter); `RunBudgeted` calls `e.gatherer.Gather(ctx, intent)` exactly once, before the Strategy runs, producing the `considered []string` every Step downstream sees. There is no registry, router, or composition mechanism for more than one Gatherer today — the same shape `engine.Executor` was in before Piece 1 built `ExecutorRegistry`/`Router`, and `engine.Applier` was in before Piece 4 built `ApplierRegistry`.
- [knowledge.md](../02-architecture/knowledge.md) defines the Authored/Derived split and the durability/portability promise entirely in the abstract — it names no concrete storage location, file format, or retrieval mechanism. `.foundry/knowledge/` exists in code (Piece 4) but is not yet named in that document as *the* Authored Knowledge store.
- `replay.Verify` ([replay/replay.go](../../replay/replay.go)) never re-invokes `Gather` at all: it replays only a recorded generate Step's `Produced` patch and re-runs `Verify`. Whatever `considered` context an Act recorded is read from `domain.Act.ConsideredFiles` / `StepRecord.Considered`, never re-derived. This matters directly for §2.6 below.

---

## 2. Target design

### 2.1 Formalize `.foundry/knowledge/` as the Authored Knowledge store

[knowledge.md](../02-architecture/knowledge.md) is updated to name what already exists in code as the current, PROVISIONAL concrete shape of Authored Knowledge: one Markdown file per contributing Act under `.foundry/knowledge/`, named `<act-id>-<slug>.md`, written only via an `apply` Step targeting `knowledge-note` (never edited in place — a correction or update is a new note, not a rewrite of an old one, matching I10's "never silently mutated" and the Record's own append-only posture). This is documentation only; `workspace.KnowledgeNoteApplier`'s behavior does not change. It explicitly does **not** decide a structured note schema (front-matter, tags, topics) — that is the separate "Authored-knowledge format stability & migration" question [knowledge.md](../02-architecture/knowledge.md) itself already flags as unresolved, and remains so.

### 2.2 `gatherer.Compose` — more than one Context Source behind one port

A new, small, pure function (or a wrapper type) in the existing `gatherer` package:

```go
// Compose returns a Gatherer that runs each of sources in order and
// concatenates their considered Context, so more than one Context Source
// (extensibility.md) can feed one Act without any change to engine.Gatherer,
// engine.Engine, or engine.RunBudgeted's single Gather call.
func Compose(sources ...engine.Gatherer) engine.Gatherer
```

This mirrors `ExecutorRegistry`/`Router` (Piece 1) and `ApplierRegistry` (Piece 4): the port stays a single method: a small, additive seam lets one composition root wire more than one concrete implementation behind it. A composition root that never calls `Compose` — every existing test and every project without a `.foundry/knowledge/` directory — is unaffected; `Compose(NaiveGatherer)` alone (one source) behaves identically to using `NaiveGatherer` directly.

### 2.3 A naive Knowledge Context Source

New package `knowledge`, holding a `Gatherer` implementing `engine.Gatherer`:

```go
type Gatherer struct { /* repoPath string */ }
func NewGatherer(repoPath string) *Gatherer
func (g *Gatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error)
```

`Gather` reads every file in `.foundry/knowledge/` (a missing directory is not an error — empty result, mirroring `LoadExecutorConfig`'s "missing file → empty" pattern this codebase already uses repeatedly), matches each note against the Intent using the **same lexical approach `NaiveGatherer` already established** — the identifier/token extraction `gatherer.go` already has, matched against each note's content — and returns the top-N matching notes' content, each entry attributed to its source file (`".foundry/knowledge/<name>.md:\n<content>"`, mirroring `NaiveGatherer`'s own `"name:\ncontent"` convention exactly, satisfying I7's provenance requirement with zero new data shape). Bounded by its own byte budget and match count, exactly as `gatherer.maxContextBytes` and `maxIdentifierMatches` already bound `NaiveGatherer`. Deliberately **not** semantic: no embeddings, no vector index, no ranking model — the same "naive is honest and provable" posture `gatherer.go`'s own package doc already states for repository files.

### 2.4 Wiring

Both composition roots (`cmd/foundry/commands/do.go`'s `wireEngine`, `session.NewSession`) replace their bare `gatherer.NewNaiveGatherer(root)` with `gatherer.Compose(gatherer.NewNaiveGatherer(root), knowledge.NewGatherer(root))`. A project with no `.foundry/knowledge/` directory yet sees byte-for-byte the same `considered` Context as today.

### 2.5 Replay and determinism: no new risk

A Context Source is already classified as the non-deterministic edge ([system-context.md](../02-architecture/system-context.md)'s determinism boundary: "Context Sources... read the live world"); what crosses it is recorded by content, not re-derived. `replay.Verify` already never re-invokes `Gather` at all (§1) — it replays only the recorded generate Step's `Produced` patch. Knowledge retrieval's output is recorded verbatim in `act.ConsideredFiles` / `StepRecord.Considered` exactly like `NaiveGatherer`'s file reads already are, and introduces **no new replay risk beyond what already exists** for a repository file that changes between an Act and a later replay. [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)'s same-version scoping is unaffected and not reopened here.

### 2.6 Scope boundary: `project-doc` is not retrieved

Only `.foundry/knowledge/` (the `knowledge-note` target's directory) is retrieved. The `project-doc` target's file (RFC-0004 §2.6, e.g. a human-facing decisions log or changelog) is a document a person reads, not one this RFC feeds back into an Executor's Context — a deliberate scope decision, not an oversight, made so this RFC's retrieval mechanism has exactly one, uniformly-shaped corpus to work over.

---

## 3. What remains genuinely unresolved (named, not decided here)

- **Authored Knowledge note schema and versioning.** No front-matter, tags, or structured metadata is added. §2.1 formalizes *where* notes live, not their internal shape. This is the "Authored-knowledge format stability & migration" question knowledge.md already names as unresolved, and it stays that way.
- **Semantic / embeddings-based retrieval.** §2.3 is lexical only, by deliberate analogy to `NaiveGatherer`'s own naive posture. Revisit only if a real project's `.foundry/knowledge/` corpus grows large enough that keyword matching demonstrably misses relevant notes — the same "don't build it until a real need motivates it" discipline the Router's capability-negotiation layer already follows.
- **Derived Knowledge (an index or cache over Authored notes).** [knowledge.md](../02-architecture/knowledge.md)'s Derived layer — recomputable, disposable, never authoritative (I9) — is not built here. A full-directory scan per Act (§2.3) is the whole mechanism; an index is a deferred optimization, not a decided absence.
- **Curating or superseding stale/contradictory notes.** Notes accumulate append-only (§2.1); nothing here resolves what happens when two retrieved notes disagree. Left to whichever Executor reads them, or a future, deliberate human-authored curation Act — not a new mechanism this RFC invents.
- **Whether Knowledge or Act is the domain center** ([OQ-001](../06-open-questions/OQ-001-domain-center.md)). This RFC's mechanism is agnostic to that question's resolution — retrieval and the Authored/Derived split work identically whichever pole eventually wins.

---

## 4. Backlog ADRs this RFC touches

- **Knowledge & semantic store** (ADR backlog, [ADR README](../03-adrs/README.md)) — this RFC proposes a concrete, deliberately narrow shape for its read side (§2.1–§2.4); it does not ratify that ADR, and explicitly leaves the note-schema and semantic-retrieval questions (§3) to it.
- **Replay & determinism contract** (ADR backlog) — §2.5 argues Knowledge retrieval introduces no new risk under today's same-version, record-don't-re-derive replay model; that ADR, when written, should confirm rather than silently inherit this argument.
- **Reusable-Act template format & evolution policy** (ADR backlog) — untouched; §2.1 deliberately declines to design a note schema so it does not preempt this ADR either.

---

## 5. Risks

- **Retrieval quality.** Naive keyword matching may retrieve irrelevant notes or miss relevant ones as the corpus grows — an accepted, named risk for a first cut, identical in kind to `NaiveGatherer`'s own accepted imprecision for repository files.
- **Prompt bloat.** Retrieved notes add to an Executor's Context alongside repository files; §2.3's own separate byte/count budget bounds Knowledge's contribution, but the two Gatherers' outputs simply concatenate (§2.2) — total Context size is not itself bounded across both. This is a real, named limitation, not a silent gap: Budget (`engine/budget.go`) bounds cost and iteration count, not token/byte count, today, for any Gatherer.
- **Corpus growth cost.** A full-directory scan per Act (§2.3) is cheap at small scale and is the entire mechanism deliberately chosen here; it is a real, accepted, and named future scaling concern (§3's Derived Knowledge deferral), not solved now.

---

## 6. Final recommendation — sequenced, none blocking the others

1. **Formalize `.foundry/knowledge/` in [knowledge.md](../02-architecture/knowledge.md)** (§2.1) — documentation only, no code, provable by inspection.
2. **`gatherer.Compose`** (§2.2) — small, pure, provable entirely with existing Gatherers (`NaiveGatherer` alone, twice, or with a fake) before any retrieval logic exists.
3. **`knowledge.Gatherer`** (§2.3) — the naive retrieval mechanism, tested in isolation against a fixture `.foundry/knowledge/` directory.
4. **Wire `Compose(NaiveGatherer, knowledge.Gatherer)` into both composition roots** (§2.4) — the one integration point, additive and backward-compatible by construction.

Derived Knowledge indexing, semantic retrieval, and Authored-knowledge format stability (§3) are named, deliberately deferred follow-ups — this RFC does not schedule them, and building them now would be exactly the kind of premature machinery this codebase has consistently declined to build ahead of a real, motivating need.

A governance process now exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but this RFC has not yet been individually ratified through it. Until it is, treat it as RFC-0001 through RFC-0004 are treated: Draft, Proposed, argued with rather than deferred to.

---

_This RFC is meant to be argued with. Its falsifiable core claim: everything §2 proposes is either a direct, minimal fulfillment of what [execution.md](../02-architecture/execution.md) already promises ("Context... from Knowledge and the world") and [knowledge.md](../02-architecture/knowledge.md) already implies, or a small, precedented composition seam (`Compose`) already proven twice by `ExecutorRegistry`/`Router` and `ApplierRegistry`. If a piece of this design does not fit one of those two buckets, that is the trigger to extend this RFC, not to build around it silently._
