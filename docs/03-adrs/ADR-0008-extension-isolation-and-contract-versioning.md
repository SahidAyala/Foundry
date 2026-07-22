# ADR-0008 — Extension Isolation & Contract Versioning

| | |
|---|---|
| **Status** | **Accepted** — ratified 2026-07-21 under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s governance process, the same day it was drafted. |
| **Date** | Drafted 2026-07-21; ratified 2026-07-21 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | The ADR backlog entry named in [README.md](README.md) ("Extension isolation & contract versioning") — whether a third-party extension's isolation mechanism (in-process, out-of-process, sandboxed) is chosen now, and what versioning policy the extension contract follows. |
| **Gates** | [extensibility.md](../02-architecture/extensibility.md)'s "Unresolved: the isolation mechanism... and the versioning policy" callout; [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) in full; [ADR-0001](ADR-0001-language-and-toolchain.md) clause 4's live, uncorrected pre-commitment to "a language-neutral wire format (gRPC/protobuf), decided in ADR-0008"; [roadmap.md](../00-overview/roadmap.md) M6 (Extensibility, not started). |
| **Process note** | Drafted under [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s process. Unlike every prior ADR in this series except this one, there is **no shipped code at all** to ratify or extend — `implementation-status.md` confirms "no third-party extension surface exists." This ADR's Decision is closer in shape to [ADR-0006](ADR-0006-routing-and-policy.md)'s capability-negotiation deferral than to [ADR-0007](ADR-0007-knowledge-and-semantic-store.md)/[ADR-0011](ADR-0011-cost-as-a-first-class-constraint.md)'s ratify-what-shipped pattern — but it also performs a real, live correction (Decision 2), the same move [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) made for ADR-0001's stale SQLite clause. |

---

## Context

**What extensibility.md already settles, and this ADR does not reopen:** [extensibility.md](../02-architecture/extensibility.md) states the *requirements* for any future extension mechanism as already-decided policy: extensions declare the Capabilities they require and receive only what is granted (default-deny); extension output is untrusted like any Executor output, passing the same verification and Judgment; the durable core (the Act, Judgment, Record) may never be redefined by an extension, only the substrate edge (Strategies, Executors, Validators, Context Sources, Router policies) may grow. [I14](../05-reference/invariants.md) states the same boundary as an invariant. None of this is reopened here.

**What genuinely has no decision yet, confirmed by [OQ-005](../06-open-questions/OQ-005-extension-isolation.md):** which isolation mechanism a third-party extension runs under (in-process, out-of-process subprocess/RPC, a sandboxed runtime such as a WASM component model, or a tiered combination), and how the extension contract itself is versioned (semver policy, pre-1.0 breakage rules). OQ-005's own status is explicitly **OPEN**, and its own current recommendation is: *"Decide nothing on mechanism yet... make the mechanism + versioning a dedicated ADR at the extensibility milestone, decoupled from the language ADR."*

**A real, live contradiction this ADR must correct, not merely describe — the same kind of correction [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) performed for ADR-0001's SQLite clause:** [ADR-0001](ADR-0001-language-and-toolchain.md) (Accepted, 2026-06-29 — before this project's own governance process, before M0, before OQ-005 was ever written) states in its Decision, clause 4: *"The extension boundary is not Go. Per R2, the plugin/provider contract is defined in a language-neutral wire format (gRPC/protobuf), decided in ADR-0008. ... This clause is binding."* Its Consequences section goes further, treating the choice as settled: *"ADR-0008 (extension contract): Go's out-of-process plugin ecosystem (gRPC, go-plugin-class tooling) is the most trodden path of any candidate. The primary isolation mechanism (gRPC subprocess) becomes low-risk."* Its Future ADR Dependencies section states ADR-0008 "must define the language-neutral boundary that clause 4 requires" and that WASM is only "a later hardening track" behind gRPC-subprocess as primary.

**None of this was ever built, and — separately — extensibility.md and OQ-005 (both written after ADR-0001, during this project's post-refactor documentation-first pass) explicitly withdrew it as a premature assertion**, not a decision this project actually stands behind: extensibility.md's own words are *"Earlier drafts asserted a specific mechanism; that assertion has been withdrawn as premature... The choice is owned by a pending ADR."* [README.md](README.md)'s own Accepted-table note on ADR-0001 already flags this precisely: *"it currently pre-states an extension isolation mechanism that is not its to decide... The mechanism claim should be removed/deferred to the extension ADR."* This is a genuine, uncorrected contradiction between an Accepted ADR (asserting gRPC-subprocess as the binding, decided mechanism) and the higher-precedence, more-recent architecture document and open-question tier (both stating the mechanism is undecided) — AGENTS.md's own conflict-resolution rule applies: *"the lower must be updated... never leave a contradiction."*

**No real third-party extension exists to motivate a concrete choice today.** `implementation-status.md`'s M6 row: "Not started. No third-party extension surface exists." No community contributor, plugin author, or external integration has ever asked to write a Strategy, Executor, Validator, or Context Source outside this repository. Choosing gRPC/protobuf, a WASM component model, or any other concrete mechanism now would be designing infrastructure against a hypothetical consumer — exactly the pattern every other ADR in this series has consistently declined (ADR-0006's capability negotiation, ADR-0007's Derived Knowledge/semantic retrieval, ADR-0011's Budget configurability).

---

## Decision

1. **No isolation mechanism is chosen. [ADR-0001](ADR-0001-language-and-toolchain.md) clause 4's specific pre-commitment to "a language-neutral wire format (gRPC/protobuf)" is explicitly corrected, not carried forward.** This ADR ratifies [extensibility.md](../02-architecture/extensibility.md)'s existing posture as the actual, sufficient decision for today: only the *requirements* are settled (default-deny Capabilities, untrusted-until-verified output, a closed durable core / open substrate edge per [I14](../05-reference/invariants.md)); the *mechanism* — in-process, out-of-process subprocess/RPC, sandboxed (e.g. WASM), or tiered — remains genuinely undecided, closing [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) with its own recommendation rather than overriding it.

2. **R2 ("language-agnostic extension boundary," [ADR-0001](ADR-0001-language-and-toolchain.md)) remains binding and is not weakened by deferring the mechanism.** No extension exists today to violate it. The requirement — that whatever mechanism is eventually chosen must not force a plugin author to write Go — is preserved exactly as extensibility.md already states it; only its *concrete discharge* (a specific wire format, a specific runtime) is deferred, not the requirement itself.

3. **The named, concrete trigger for building a real mechanism: a real third party actually wants to write a Strategy, Executor, Validator, or Context Source outside this repository.** Not a milestone number, not the passage of time, not "M6 starting" in the abstract — an actual, specific extension request. Until that happens, today's substrate-edge extensibility (in-tree Go packages implementing `engine.Executor`, `engine.Gatherer`, etc., exactly how `executor/openai` was added) is Foundry's entire extension story, and it is sufficient: nothing today asks for more.

4. **No versioning policy is decided for the extension contract either, for the same reason — there is no concrete contract shape yet to version.** When a mechanism is eventually chosen and a real wire contract (or Go interface, if in-process is ever revisited) exists, whoever writes that follow-up decision should default to the same posture this project has applied consistently elsewhere unless a concrete reason argues otherwise: pre-1.0, **change freely but never silently** ([ADR-0009](ADR-0009-cli-and-output-contract.md)'s CLI posture), **additive-only evolution with no version field until a second independent reader of the contract exists** ([ADR-0004](ADR-0004-reusable-act-template-format-and-evolution-policy.md)'s Pipeline-document precedent, [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s identical reasoning for `act.json`). This is guidance for that future ADR to start from, not a decision this ADR makes now.

5. **[ADR-0001](ADR-0001-language-and-toolchain.md)'s own text is corrected in place** (Migration Strategy, below) so a future reader is not misled into thinking gRPC/protobuf, or gRPC-subprocess-as-primary-with-WASM-as-hardening, was ever actually decided. The correction follows [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s own precedent exactly: the stale clause is corrected, not silently left, and ADR-0001 remains Accepted and otherwise unamended.

---

## Alternatives Considered

### Ratify gRPC/protobuf-over-subprocess now, as ADR-0001 clause 4 already asserts
- **For:** Would resolve the contradiction by making ADR-0001's claim true instead of false; Go's out-of-process plugin ecosystem (gRPC, `go-plugin`-class tooling) is genuinely mature and low-risk, exactly as ADR-0001's own Consequences section argued.
- **Against:** No real extension exists to validate this shape against. Designing a concrete wire protocol, a service definition, a capability-negotiation handshake, and a credential-passing story for a consumer that has never actually shown up is precisely the premature-generality pattern this project's entire ADR series has declined every other time it came up (RFC-0002 §7 layer 2 capability negotiation, Derived Knowledge indexing, semantic retrieval, Budget configurability). A concrete third-party extension request would very likely surface real requirements (what capabilities does it actually need? what's its actual credential model? does it need streaming or one-shot calls?) that a speculative design would get wrong.
- **Verdict:** Rejected for now. Ratified instead as Decision 1 — the mechanism stays undecided, with a named trigger for revisiting.

### Choose a sandboxed (WASM component model) mechanism as primary instead
- **For:** Stronger isolation than a subprocess; portable; increasingly mature ecosystem.
- **Against:** Same objection as above — no concrete consumer to design against — plus OQ-005's own honest accounting that WASM's ecosystem maturity and host-integration cost are real, unresolved trade-offs, not yet worth paying without a motivating case. Go's own WASM-component-as-guest story is also less mature than some alternative implementation languages', a cost ADR-0001 itself already named honestly.
- **Verdict:** Rejected for now, for the identical no-consumer reason as gRPC/protobuf above.

### Decide a tiered model now (built-in full trust / signed out-of-process / community sandboxed default-deny)
- **For:** OQ-005's own Alternative 4; would let different trust levels get different isolation strength without committing to one mechanism for everything.
- **Against:** A tiered policy is a *composition* of mechanism choices this ADR has already declined to make individually — deciding the composition before any of its parts exist is designing a hierarchy for building blocks that don't exist yet.
- **Verdict:** Rejected for now, same reasoning.

### Leave [ADR-0001](ADR-0001-language-and-toolchain.md) clause 4 uncorrected, since "ADR-0008 hasn't been written yet" already implies it's pending
- **For:** Minimal effort; the forward reference ("decided in ADR-0008") already signals this is not yet settled to a careful reader.
- **Against:** Clause 4 does not read as pending — it reads as a **binding** decision ("This clause is binding") with a specific mechanism named (gRPC/protobuf), and the Consequences section states the choice as already made ("the primary isolation mechanism... becomes low-risk"). A careful reader following AGENTS.md's documentation hierarchy would reasonably treat an Accepted ADR's own text as settled, exactly the honesty failure AGENTS.md's "never leave a contradiction" rule exists to prevent.
- **Verdict:** Rejected. Ratified instead as Decision 5 — corrected in place, mirroring [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md)'s identical treatment of ADR-0001's SQLite clause.

---

## Consequences

### What this decision makes EASIER
- **[OQ-005](../06-open-questions/OQ-005-extension-isolation.md) is closed** with its own recommendation, not overridden.
- **A real, live contradiction in an Accepted ADR is corrected**, rather than left for a future contributor to discover and be misled by — the same value [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) delivered for ADR-0001's SQLite clause.
- **A future mechanism-and-versioning ADR, whenever a real trigger fires, inherits explicit guardrails** (R2 remains binding, a default versioning posture to start from) instead of a blank page or a stale, contradicted assumption.
- **[README.md](README.md)'s own flagged "pending amendment" on ADR-0001 is resolved.**

### What this decision makes HARDER
- **Nothing structurally** — like [ADR-0006](ADR-0006-routing-and-policy.md), this declines new work with a named trigger rather than building speculatively.
- **M6 (Extensibility) remains "Not started"** after this ADR — it resolves the *policy* contradiction and the open question's disposition, but builds no actual extension surface. Named honestly, not hidden.
- **Whoever eventually writes the mechanism ADR inherits a real design problem** (isolation vs. ergonomics vs. ecosystem maturity) this ADR explicitly declines to pre-solve — the same honest deferral [ADR-0006](ADR-0006-routing-and-policy.md) made for capability negotiation.

### Reversibility
High. This ADR ratifies a requirements-only posture that already exists ([extensibility.md](../02-architecture/extensibility.md)) and corrects a stale claim in another ADR — nothing built to unwind, and the correction itself only removes an assertion that was never true in the shipped system.

---

## Migration Strategy

No code changes; no data migration. This ADR corrects documentation only.

1. Correct [ADR-0001](ADR-0001-language-and-toolchain.md) clause 4: replace "the plugin/provider contract is defined in a language-neutral wire format (gRPC/protobuf), decided in ADR-0008" with language stating the *requirement* (language-agnostic boundary, R2) remains binding while the *mechanism* is explicitly undecided, deferred to this ADR.
2. Correct [ADR-0001](ADR-0001-language-and-toolchain.md)'s Consequences section ("ADR-0008 (extension contract): Go's out-of-process plugin ecosystem... becomes low-risk") to note this was an anticipation that was never ratified as the actual choice, mirroring exactly how [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) corrected ADR-0001's SQLite anticipation.
3. Correct [ADR-0001](ADR-0001-language-and-toolchain.md)'s "What this decision makes HARDER" bullet on WASM ("This is part of *why* ADR-0008 picks gRPC-subprocess as the primary path...") — no such pick has been made; the underlying honest cost (Go's WASM-guest story is less mature than some alternatives') stands as a *future* consideration, not a foreclosed ordering.
4. Correct [ADR-0001](ADR-0001-language-and-toolchain.md)'s Future ADR Dependencies entry for ADR-0008 ("discharges R2 — it must define the language-neutral boundary that clause 4 requires") to reflect that this ADR (ADR-0008) explicitly defers that discharge rather than performing it now.
5. Update [extensibility.md](../02-architecture/extensibility.md)'s "Unresolved" callout to cite this ADR as the formal disposition (mechanism/versioning still undecided, now a ratified deferral rather than an open question).
6. Resolve [OQ-005](../06-open-questions/OQ-005-extension-isolation.md): update its own Status section and [open-questions/README.md](../06-open-questions/README.md)'s index row.
7. Update [README.md](README.md): move this row from Backlog to Accepted upon ratification; remove ADR-0001's "pending amendment" note (now resolved by steps 1–4 above).
8. Update [implementation-status.md](../00-overview/implementation-status.md)'s ADR section, M6 row, and changelog.

---

## Future ADR Dependencies

- **A future "Extension mechanism & contract versioning" decision**, whenever the named trigger (Decision 3) fires, inherits: R2 as a binding requirement (Decision 2), extensibility.md's default-deny/untrusted-until-verified/closed-core requirements (unchanged by this ADR), and a default versioning posture to start from unless a concrete reason argues otherwise (Decision 4) — additive-only, no version field until a second independent reader exists, pre-1.0 "change freely but never silently."
- **Cost as a first-class constraint** ([ADR-0011](ADR-0011-cost-as-a-first-class-constraint.md)): no dependency — already ratified independently, and its own Future ADR Dependencies section already noted no dependency on this ADR.

---

## Open Questions

Carried forward from [OQ-005](../06-open-questions/OQ-005-extension-isolation.md), not resolved here:

1. **Which mechanism is *primary* vs. a hardening track** (subprocess/RPC vs. sandboxed WASM vs. tiered)? Left to the future ADR Decision 3 names as the trigger for.
2. **How are the extension contract and ports versioned** (semver; pre-1.0 breakage policy) in concrete detail, once a real contract shape exists? Decision 4 only states a default posture to start from, not a full policy.
3. **Is a literal WASM component-model integration ever worth its ecosystem-maturity and host-integration cost** for Foundry specifically, given Go's own less-mature guest-side story? Not decided — a real question for whoever writes the eventual mechanism ADR.

---

## Review Checklist

Walked through at ratification (2026-07-21):

- [x] **No contradiction with accepted documents** beyond the one this ADR deliberately corrects (ADR-0001 clause 4 and its downstream Consequences/Future-ADR-Dependencies text). Confirmed against [extensibility.md](../02-architecture/extensibility.md) (this ADR's Decision 1 matches its existing requirements-only posture exactly) and [I14](../05-reference/invariants.md) (unchanged).
- [x] **ADR-0001's corrected clauses read accurately** — clause 4, its Consequences section, its "Harder" WASM bullet, and its Future ADR Dependencies entry for ADR-0008 all stop asserting a decision this ADR explicitly declines to make, without erasing that ADR-0001 once anticipated one (the same honesty [ADR-0002](ADR-0002-persistence-content-addressing-and-on-disk-layout.md) preserved for its own correction).
- [x] **OQ-005's own recommendation is followed, not silently overridden** — "decide nothing on mechanism yet" is exactly what Decision 1 does.
- [x] **Process caveat resolved.** Ratified under [ADR-0000](ADR-0000-governance-and-ratification-process.md); this Status row, [README.md](README.md)'s backlog table, and the Migration Strategy's downstream docs all updated in the same ratifying pass.

---

_This ADR makes no new architectural choice about how third-party extensions are isolated or versioned — it ratifies [extensibility.md](../02-architecture/extensibility.md)'s existing requirements-only posture as sufficient for today, closing [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) with its own "decide nothing yet" recommendation. Its real, load-bearing work is correcting a live contradiction: [ADR-0001](ADR-0001-language-and-toolchain.md) clause 4 asserted a specific, binding mechanism (gRPC/protobuf-over-subprocess) that was later withdrawn as premature by this project's own post-refactor architecture documents, but was never actually corrected in ADR-0001's own text until now. R2 (the language-agnostic extension boundary) remains fully binding throughout — only the concrete mechanism and its versioning policy are deferred, to the first real third-party extension request, not to a milestone number or the passage of time._
