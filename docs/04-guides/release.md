# Release Guide

> How Foundry is released. **Preliminary** — the project is pre-implementation; this captures intended practice and will firm up as the build reaches a releasable state ([../00-overview/roadmap.md](../00-overview/roadmap.md), M5–M7). Terms: [../05-reference/terminology.md](../05-reference/terminology.md).

## Principles

- **`main` is always releasable.** Every merge leaves a working, tested system.
- **Reproducible, signed artifacts.** Releases are built from a known commit with verifiable, signed outputs.
- **Honest versioning.** Pre-1.0, internal contracts may change with documented migration notes. At 1.0, the durable-core contracts (the Act lifecycle, the Record, Judgment semantics, the extension contract) are frozen under semantic versioning; the substrate edge continues to evolve.

## Compatibility surfaces gated by release

A release must not silently break a compatibility surface owned by an ADR (see [../03-adrs/README.md](../03-adrs/README.md)): the Record format and hashing, the reusable-Act template schema, the Executor/extension contracts, the CLI/output contract, and the Authored-Knowledge format. Each change to these follows its owning ADR's versioning policy.

### Record format & persistence

Ratified in [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md) (2026-07-20) — **Status: Accepted.** The current, real behavior:

- **One flat, human-readable JSON file per Act** (`<root>/<act.ID>/act.json`), written exactly once and immutable thereafter — no embedded database, no content-addressing. `<root>/<act.ID>/checkpoint.json` is a separate, mutable, overwritable file holding an interrupted Act's in-progress state, explicitly not the Record itself.
- **Commit `.foundry/acts/` to a project's own repository** — it is durable audit history, never a disposable cache (I8). `checkpoint.json` files are the one exception and may be gitignored.
- **No version field exists on `act.json` yet** — additive-only evolution, mirroring [ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md)'s identical policy for the Pipeline-document schema. Whether cross-version replay ever needs one is the **Replay & determinism contract** ADR's question (backlog), not decided here.

### CLI & output contract

Ratified in [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md) (2026-07-20) — **Status: Accepted.** The current, real behavior, per that ADR's Decisions 2 and 4:

- **Pre-1.0, the flag CLI's commands, flags, and exit codes, and the slash-command vocabulary, may change** — but never silently: a breaking change carries a `BREAKING CHANGE:`-style note in its commit, surfaced through the conventional-commit-driven changelog above. At 1.0, this surface is frozen under semantic versioning, per this guide's "Honest versioning" principle.
- **Exit codes, for every one-shot command (`do`, `log`, `show`, `replay`, `resume`) and `foundry`'s own top-level dispatch:** `0` for success or `-h`/`--help`, `1` for a runtime or internal error, `2` for an argument-parsing or usage error. This is deliberately coarse — a verification failure, a declined approval, a missing repository, and a config error all currently collapse to exit code `1`; there is no distinct code per failure class yet.
- **No machine-readable output mode exists** (no `--json`, no structured schema) — every command writes plain, human-oriented text. Revisit only against a real CI-consumer need, not speculatively.

## Changelog

Generated from the commit history (conventional commits). A release's notes are grounded in the recorded history of what changed and why.

> **Partially resolved.** [ADR-0000](../03-adrs/ADR-0000-governance-and-ratification-process.md)'s governance process exists and has already ratified most surfaces above: the Record format and hashing ([ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md), see above), the reusable-Act template schema ([ADR-0004](../03-adrs/ADR-0004-reusable-act-template-format-and-evolution-policy.md)), the Executor contract ([ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md)), and the CLI & output contract ([ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md), see above). The extension-isolation half of "Executor/extension contracts" and the Authored-Knowledge format remain [ADR backlog](../03-adrs/README.md) items — treat the freeze commitments for those specifically as intended, not ratified, until each is drafted and accepted.
