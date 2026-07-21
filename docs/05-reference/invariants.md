# Invariants

> **Maturity: PROVISIONAL.** These are the rules Foundry intends to uphold, expressed as the operational form of the principles in [../00-overview/principles.md](../00-overview/principles.md). Most (I1–I12) are *strongly grounded* in RFC-0001 and would survive a change to the domain model. A few (notably I8, I13) are phrased in the provisional **Act/Engine** vocabulary and would be **re-stated, not abandoned**, if [OQ-001](../06-open-questions/OQ-001-domain-center.md)/[OQ-002](../06-open-questions/OQ-002-pipeline-as-strategy.md) resolve differently. A governance process now exists ([ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)), but none of these invariants have themselves been individually walked through ratification yet — treat them as PROVISIONAL until one is. Terms: [terminology.md](terminology.md).

## Control & determinism
- **I1 — Control flow is owned by the Engine, never by a model.** A model fills in work; it never decides what runs next, whether a verdict passed, or whether an Act is done.
- **I2 — Process determinism, not output determinism.** Foundry guarantees the *structure* of an Act is reproducible and replayable; it never claims a model produces identical text.
- **I3 — Replay re-derives the deterministic and replays the recorded.** Deterministic work is re-executed and must match; non-deterministic Executor output is replayed from the Record.

## Trust
- **I4 — Every Outcome is untrusted until verified.** Executor (and extension) output earns trust only by passing verification and an Authority's acceptance.
- **I5 — Accountability never leaves a human silently.** A consequential or outward Outcome is owned by an Authority. Approval is the default; autonomy is an opt-in dial, never a removal of ownership.
- **I6 — Verification is deterministic-first.** Model-based judgment is a last resort for the irreducibly subjective, never the default check.
- **I7 — Provenance is mandatory.** Every piece of considered Evidence is attributable to its source.

## Durability & knowledge
- **I8 — The Record is durable and immutable.** It is the audit trail and the replay source; it is never a disposable cache.
- **I9 — Authored Knowledge is the source of truth; Derived Knowledge is a cache.** The two are never conflated.
- **I10 — Project state is never silently mutated.** Changes to code or Authored Knowledge are Outcomes that an Authority accepts; Foundry never rewrites a project's source of truth behind its back.
- **I11 — A project owns and can export its Authored Knowledge.** It remains readable independently of any model or vendor.

## Substrate & structure
- **I12 — The model is substrate, never a domain concept.** A correct statement of what Foundry is does not mention an LLM.
- **I13 — No single execution Strategy is privileged.** The Pipeline is one Strategy among several.
- **I14 — The durable core is closed; the substrate edge is open.** Extensions may add Strategies, Executors, Validators, and Context Sources; they may never redefine the Act, the Judgment, or the Record.

> Only **I11 (knowledge migration)** still has a **pending owning ADR**; it states intent not yet fully proven in a decision. See [../03-adrs/README.md](../03-adrs/README.md). **I8's durability classification** is ratified by [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md); **I3's cross-version scope and I6's validator-determinism limits** are ratified by [ADR-0003](../03-adrs/ADR-0003-replay-and-determinism-contract.md).
