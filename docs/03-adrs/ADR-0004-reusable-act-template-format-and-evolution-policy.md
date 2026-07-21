# ADR-0004 — Reusable-Act Template Format & Evolution Policy

| | |
|---|---|
| **Status** | **Proposed** — drafted per [RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §9 Phase 0; not yet ratified. |
| **Date** | Drafted 2026-07-20 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted per RFC-0002 Phase 0 |
| **Ratifies** | The ADR backlog entry named in [../03-adrs/README.md](README.md) ("Reusable-Act template format & evolution policy") — the authored Pipeline document's wire format and its evolution discipline. |
| **Gates** | [RFC-0002](../01-rfcs/RFC-0002-pipeline-execution-runtime.md) §9 Phase 0's second prerequisite ADR (the first, Executor contract & capability model, shipped as [ADR-0005](ADR-0005-executor-contract-and-capability-model.md)). Also unblocks **Routing & policy** (backlog, proposed ADR-0006), which needs a settled document format to add routing semantics to rather than an implicit one. |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. RFC-0002, the RFC this ADR was drafted from, remains Draft — Proposed in its own right; ratifying this ADR does not ratify that RFC. |

---

## Context

RFC-0002 names this ADR directly as a Phase 0 prerequisite, alongside the Executor contract ADR ([ADR-0005](ADR-0005-executor-contract-and-capability-model.md), now accepted):

> **Phase 0 — Prerequisite ADRs (no code).** Write the two backlog ADRs this migration needs before its compatibility surfaces harden: (a) reusable-Act-template / Pipeline-definition format & evolution policy, (b) Executor contract & capability model.

RFC-0002 §6 gives the reason it can't wait: the authored Pipeline document is "cheap [to change] while pre-1.0 (schema has no external consumers yet); becomes a real compatibility surface the moment a second Foundry instance reads a Pipeline definition." That moment hasn't arrived — there is still exactly one implementation, one machine, one author per project — but the shape decided here is what a second reader would inherit, so it is decided deliberately now rather than by accretion.

**What already exists in code**, which this ADR ratifies or amends rather than invents from nothing:

- The wire format is JSON, decoded by `engine.DecodePipelineDocument` (`engine/document.go`) into a *distinct* type from the runtime `engine.Pipeline` — the separation is deliberate and already present specifically so this ADR's question wouldn't get silently pre-decided by reusing Go struct tags as the wire contract.
- `PipelineDocument` has three top-level fields — `name`, `steps`, `repair` — and each `StepDocument` has two required fields (`id`, `kind`) plus four optional, `omitempty` fields (`capability`, `executor`, `feeds_forward`, `target`) reserved for the Router and unexercised by any of the five documents shipped today.
- **No version field exists anywhere** — not in the JSON, not in the Go structs, not in any loader. Both `engine/document.go` and `engine/step.go` carry comments explicitly deferring this to "the unwritten ADR."
- Decoding uses plain `json.Unmarshal` — unknown fields at any level are silently dropped. Only known-field *values* are validated: empty `Name`, zero `Steps`, negative `MaxAttempts`, an unrecognized `Kind` string, or a `Repair.Target` that names no declared Step ID are all hard decode/registration errors.
- Step `Kind` is a closed set of exactly five values (`generate`, `verify`, `approve`, `apply`, `record`); [docs/04-guides/pipelines.md](../04-guides/pipelines.md) states the house rule directly: "A Step Kind PipelineStrategy does not recognize is a decode-time error, never a silently skipped Step." Today that fail-loud discipline applies to `Kind` only — not to unrecognized fields elsewhere in the document, which is an inconsistency this ADR resolves.
- `Name` must be globally unique across built-in and project-authored Pipelines; `engine.Registry.Register` hard-errors on collision.

**On terminology:** [terminology.md](../05-reference/terminology.md) does not define "reusable Act template" as a canonical noun — it appears only in the archived table as the retirement mapping for the rejected term *Skill* ("derivative of Act; not irreducible"). The artifact this ADR actually governs is the **Pipeline document** — an authored, named **Strategy** — not a new domain concept. This ADR keeps the backlog's inherited title for cross-reference continuity but does not ratify "Act template" as vocabulary in its own right.

This ADR does not touch `.foundry/acts/<id>/act.json` (the *recorded* Act trace) — that compatibility surface belongs to the separate, still-unwritten "Persistence, content-addressing & on-disk layout" backlog ADR. It also does not resolve [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md) (whether Pipeline is the system's spine or one Strategy among several) or [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) (cross-Engine-version replay) — both adjacent, neither this ADR's to decide.

---

## Decision

1. **The reusable artifact is the authored Pipeline document. No separate "Act template" format is introduced.** A single reusable Step is already a one-Step Pipeline document; adding a second, smaller format for that case would be two ways to express the same thing. Every future reference to "a reusable Act template" means "a named Pipeline document under `.foundry/pipelines/`."

2. **The wire format stays JSON-only.** RFC-0002 §6 floated YAML-authored/JSON-canonical as a future option; it is not adopted now. Nothing in the codebase parses YAML today, and no author has asked for it. Revisit only if a real authoring-ergonomics need surfaces (e.g., comments or anchors become worth the added parsing/round-trip layer) — not speculatively here.

3. **No explicit schema-version field is added yet.** With exactly one implementation and no external consumer, guessing at a versioning scheme now risks the premature-hardening RFC-0002 §10 warns against. Instead, this ADR ratifies the additive-only discipline already implicit in the code as an explicit, binding policy going forward: new document fields must be optional, `omitempty`, and decode to a safe zero-value default on any document written before the field existed. The existing `PipelineDocument`/`Pipeline` type split is ratified as the permanent mechanism that keeps this possible without ever coupling the wire format to Go struct internals.

4. **Unknown fields, at any level, become a hard decode error.** `engine.DecodePipelineDocument` adopts `json.Decoder.DisallowUnknownFields()`. This generalizes the fail-loud principle the codebase already applies to `Kind` — "never a silently skipped Step" — to the whole document: a typo'd field name or a stray leftover key is caught at decode time, not silently ignored. This is a deliberate trade against forward compatibility: a document written for a newer schema, if it adds a field an older binary doesn't know about, will now fail to decode on that older binary instead of silently ignoring the new field. That trade is acceptable today because no such cross-version scenario exists; it is exactly the trigger for introducing a real version field and negotiated tolerance later (see Open Questions).

5. **Step `Kind` remains a closed set of exactly five values.** Adding a sixth Kind is a breaking change to this ADR, not a document-level extension point — it requires an explicit amendment here (or a superseding ADR), the same way a new Kind today already requires a code change to `domain.StepKind*`, not just a new document.

6. **Pipeline naming stays globally unique; there is no in-system multi-version-per-name concept.** The existing registration-time collision error (built-in vs. project, project vs. project) is ratified as-is. Evolving a Pipeline means editing or replacing its JSON file in place; git history is the version history, matching how the rest of this project treats history (archive, don't duplicate).

7. **The Router-reserved fields (`capability`, `executor`, `feeds_forward`, `target`) keep their current optional, unexercised status.** Their semantics are out of scope here and belong to **Routing & policy** (backlog, proposed ADR-0006) — this ADR only guarantees they remain valid, additive, `omitempty` fields under Decision 3's discipline.

---

## Alternatives Considered

### Adopt YAML-authored / JSON-canonical now (RFC-0002 §6's floated option)
- **For:** Matches RFC-0002's own stated preference; YAML supports comments, which JSON-authored-by-hand cannot.
- **Against:** No implementation exists today, no author has asked for it, and it adds a parse/round-trip/canonicalization layer to build and maintain for zero present benefit. Speculative for a need that hasn't materialized.
- **Verdict:** Rejected for now. Revisit only on a real authoring-ergonomics complaint.

### Add an explicit `"version"` field now
- **For:** Forecloses ever having to retrofit versioning onto documents that predate it.
- **Against:** With one implementation and no second reader, there's no real compatibility question to version against yet — any scheme chosen now (integer? semver? date-stamp?) is a guess, and RFC-0002 §10 explicitly warns against hardening a surface before a real need forces the shape. The `PipelineDocument`/`Pipeline` type split already insulates the runtime from whatever scheme is chosen later.
- **Verdict:** Rejected for now, in favor of Decision 3's additive-only discipline plus an explicit revisit trigger (Open Questions).

### Keep unknown-field tolerance as-is (status quo: silent `json.Unmarshal`)
- **For:** Never breaks a document that has extra, unused keys; maximally forward-tolerant.
- **Against:** Inconsistent with the project's own stated principle for `Kind` ("decode-time error, never silently skipped") — a misspelled `"capabilty"` or `"repiar"` key would silently vanish today rather than surfacing as an authoring mistake. Silent tolerance is a worse failure mode than a clear decode error while the project has exactly one author per document and no forward-compatibility need to protect yet.
- **Verdict:** Rejected in favor of Decision 4 (`DisallowUnknownFields`). Revisit when a genuine cross-version scenario exists.

### A separate, smaller "Act template" format for a single reusable Step
- **For:** Distinguishes "a whole authored workflow" (Pipeline) from "a small reusable snippet" (Act template), which is closer to the backlog's inherited name.
- **Against:** A one-Step Pipeline document already expresses this with zero extra ceremony; a second format for the same case is duplication with no capability gain, and `terminology.md` already treats "Act template" as a derivative label, not an irreducible concept.
- **Verdict:** Rejected. Ratified instead as Decision 1: one format, the Pipeline document.

---

## Consequences

### What this decision makes EASIER
- **RFC-0002 §9 Phase 0 is now fully satisfied** (both prerequisite ADRs exist), unblocking further hardening of the Pipeline/Step schema without an open question hanging over it.
- **Authors get immediate, precise feedback** on typo'd or stray fields instead of a silently-ignored key producing confusing runtime behavior later.
- **Routing & policy** (backlog ADR-0006) inherits a settled document format and evolution discipline to add routing semantics to, rather than having to decide document versioning as a side effect of adding routing fields.
- **No new abstraction to maintain** — Decision 1 keeps exactly one authored-document concept (Pipeline), not two overlapping ones.

### What this decision makes HARDER
- **Decision 4 is a breaking change in principle**, though not in practice today: any currently-authored document with an extraneous or misspelled field would now fail to decode where it previously decoded silently. Verified against all five shipped documents (`default`, `review`, `feature`, `bugfix`, `release`) — none carry a stray field, so no existing document breaks.
- **Forward compatibility is deliberately not solved.** A future newer document (with a genuinely new optional field) loaded by an older binary will now hard-fail with "unknown field" rather than degrading gracefully. That is an explicit, accepted trade until the version-field trigger fires (see Open Questions) — not an oversight.
- **Adding a sixth Step Kind now requires amending this ADR**, not just shipping a document — a deliberate constraint (Decision 5), not a limitation to work around.

### Reversibility
Medium. Turning on `DisallowUnknownFields` (Decision 4) is a one-line code change but is user-visible — it converts a previously-silent pass into a hard error — so it needs a changelog note, not just a code diff. Everything else (additive-field discipline, no version field yet, closed Kind set, global name uniqueness) is either already the shipped behavior or a documentation-only ratification; none of it requires a data migration.

---

## Migration Strategy

Turn on `json.Decoder.DisallowUnknownFields()` in `engine.DecodePipelineDocument`. No data migration is needed: all five currently-shipped Pipeline documents (`engine/pipelines/default.json`, `engine/pipelines/review.json`, `.foundry/pipelines/feature.json`, `.foundry/pipelines/bugfix.json`, `.foundry/pipelines/release.json`) already decode cleanly under strict field checking — verified against their actual field sets, no stray keys present. Document the additive-only field policy (Decision 3) and the closed Step Kind set (Decision 5) in [docs/04-guides/pipelines.md](../04-guides/pipelines.md) alongside the existing "decode-time error, never silently skipped" line.

---

## Future ADR Dependencies

- **Routing & policy** (backlog, proposed ADR-0006): inherits Decision 7 — the Router-reserved fields' current optional, unexercised status — as its starting point, and must define their semantics within Decision 3's additive-only discipline rather than reopening document versioning to do so.
- **Persistence, content-addressing & on-disk layout** (backlog): owns the separate `act.json` recorded-trace compatibility surface. This ADR's additive-evolution policy applies only to the *authored* Pipeline document, not the *recorded* Act trace — the two surfaces are explicitly not conflated here.
- **Replay & determinism contract** (backlog): cross-Engine-version replay ([OQ-003](../06-open-questions/OQ-003-replay-across-versions.md)) will likely need this ADR's version-field trigger (Open Questions, below) to have already fired before it can be meaningfully scoped; noted as adjacent, not resolved here.

---

## Open Questions

1. **What exactly fires the "add a version field now" trigger?** "The moment a second Foundry instance/version reads a Pipeline document" (RFC-0002 §6's own framing) is qualitative, not a hard criterion. Left open until a concrete scenario (a shared team repo, a second machine, an upgrade across a breaking Foundry release) actually arises.
2. **Should Step Kind ever gain a document-level extension escape hatch** (e.g., a declared custom Kind resolved by a project-registered handler), or does this become moot depending on how [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md) resolves (Pipeline as the spine vs. one Strategy among several)? Not decided here.
3. **Not resolved by this ADR, and not to be read as settled:** [OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md) (is Pipeline the system's spine or one Strategy among several) and [OQ-003](../06-open-questions/OQ-003-replay-across-versions.md) (cross-Engine-version replay scope) — both adjacent to this ADR's subject matter, neither owned by it.

---

## Review Checklist

To be completed at ratification:

- [ ] **No contradiction with accepted documents.** Confirm against [ADR-0005](ADR-0005-executor-contract-and-capability-model.md) (Router-reserved fields, Decision 7 here, must not preempt ADR-0006) and [ADR-0010](ADR-0010-vcs-pr-integration-and-apply-targets.md) (registration-time publish-policy validation is a separate check, unaffected by Decision 4's decode-time check).
- [ ] **Decision 4 verified against all shipped documents.** All five current Pipeline documents decode cleanly under `DisallowUnknownFields` — re-verify at ratification time in case new documents were added since drafting.
- [ ] **Decision 1 (no separate Act-template format) does not contradict `terminology.md`.** Confirmed: "reusable Act template" is not a canonical noun there; this ADR does not introduce one.
- [ ] **OQ-002 and OQ-003 are cited as explicitly out of scope**, not silently resolved.
- [ ] **Process caveat resolved.** Ratify under [ADR-0000](ADR-0000-governance-and-ratification-process.md); update this Status row and the backlog table in [README.md](README.md) in the same ratifying commit.

---

_This ADR fixes the authored Pipeline document as the sole reusable-Act artifact, keeps its wire format JSON-only with no version field yet, and converts unknown-field tolerance into a hard decode error to match the project's existing fail-loud discipline for Step Kind — while explicitly flagging that forward compatibility across Foundry versions is an accepted, deliberate gap until a real second-reader scenario forces a version field into existence._
