# Release Guide

> How Foundry is released. **Preliminary** — the project is pre-implementation; this captures intended practice and will firm up as the build reaches a releasable state ([../00-overview/roadmap.md](../00-overview/roadmap.md), M5–M7). Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Principles

- **`main` is always releasable.** Every merge leaves a working, tested system.
- **Reproducible, signed artifacts.** Releases are built from a known commit with verifiable, signed outputs.
- **Honest versioning.** Pre-1.0, internal contracts may change with documented migration notes. At 1.0, the durable-core contracts (the Act lifecycle, the Record, Judgment semantics, the extension contract) are frozen under semantic versioning; the substrate edge continues to evolve.

## Compatibility surfaces gated by release

A release must not silently break a compatibility surface owned by an ADR (see [../03-adrs/README.md](../03-adrs/README.md)): the Record format and hashing, the reusable-Act template schema, the Executor/extension contracts, the CLI/output contract, and the Authored-Knowledge format. Each change to these follows its owning ADR's versioning policy.

## Changelog

Generated from the commit history (conventional commits). A release's notes are grounded in the recorded history of what changed and why.

> **Unresolved (human decision required):** the exact pre-1.0 vs post-1.0 freeze scope depends on decisions still in the [ADR backlog](../03-adrs/README.md) and on a governance process that does not yet exist. Treat the freeze commitments above as intended, not ratified.
