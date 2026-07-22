# Architecture — Extensibility

> **Maturity: PROVISIONAL** (the *requirements* are grounded and ratified by [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md); the isolation *mechanism* and versioning remain deliberately UNDECIDED — the same ADR closes [OQ-005](../06-open-questions/OQ-005-extension-isolation.md) with its own "decide nothing yet" recommendation rather than overriding it).
>
> **This document answers exactly one question: _What may be extended?_**
> It does not define the domain ([domain.md](domain.md)) or the boundaries ([system-context.md](system-context.md)). Terms are defined in [../05-reference/terminology.md](../05-reference/terminology.md).

## The principle: the substrate is open, the core is closed

Extensibility follows one rule: **everything that is mechanism may be extended; nothing that defines the Act or its trust may be redefined.** Cleverness belongs at the replaceable edge; the part that records, verifies, and holds accountability stays conservative.

## Open for extension (the substrate edge)

These are expected to grow, including by third parties:

- **Strategies** — new ways to produce an Act (new Pipeline kinds, agent loops, deterministic procedures).
- **Executors** — new resources that perform work (new model backends, tools, human-task kinds). A model is just one Executor.
- **Validators** — new checks contributing to Evidence.
- **Context Sources** — new origins of knowledge feeding the *considered* Evidence.
- **Router policies** — new placement strategies over Capabilities.

## Closed for redefinition (the durable core)

These define what an Act *is* and may not be altered by an extension:

- The **Act** and its lifecycle.
- The meaning of a **Judgment** and the requirement of an **Authority**.
- The **Record** and its immutability.
- The contract that an Outcome is **untrusted until verified**.

An extension may add a new way to *produce* or *check* work; it may never change what it means for work to be *trusted* or *recorded*.

## Trust and safety of extensions

Because extensions can include third-party code running near a project's code and credentials, the extension boundary is also a trust boundary:

- Extensions declare the **Capabilities** they require; they receive only what is granted (default-deny).
- Extension output is **untrusted** like any Executor output — it passes the same verification and Judgment.

> **Resolved (deliberate non-decision):** per [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md) (Accepted 2026-07-21), the **isolation mechanism** for third-party extensions (e.g. out-of-process vs sandboxed execution) and the **versioning policy** for the extension contract remain undecided by ratified choice, not oversight — no real third-party extension exists to design either against. Earlier drafts asserted a specific mechanism (gRPC/protobuf); that assertion was withdrawn as premature and [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md) has since corrected the one place it had gone uncorrected ([ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) clause 4). This document's requirements (default-deny, untrusted-until-verified, language-agnostic boundary) stand as the actual, sufficient decision for today. The mechanism/versioning choice is deferred to a future ADR, triggered only by a real third party wanting to write a Strategy, Executor, Validator, or Context Source outside this repository.
