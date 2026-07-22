# OQ-005 — How are third-party extensions isolated and versioned?

**Maturity: OPEN QUESTION** · informs [../02-architecture/extensibility.md](../02-architecture/extensibility.md) (PROVISIONAL)

## Problem
Extensions (Strategies, Executors, Validators, Context Sources) may be third-party code running near a project's code and credentials. How are they isolated, permissioned, and versioned — without locking the ecosystem to one implementation language?

## Context
An earlier draft asserted a specific isolation mechanism (out-of-process/subprocess as primary, sandbox as hardening). That assertion was withdrawn as premature: it pre-decided a question owned by a future ADR and conflicted with the prior architecture's stated preference. Only the *requirements* are settled; the *mechanism* is not.

## Alternatives
1. **In-process** — fastest, but unsafe near credentials and locks extensions to the Engine's language. (Disfavored.)
2. **Out-of-process (subprocess/RPC)** — language-agnostic, crash-isolated, mature; IPC overhead.
3. **Sandboxed runtime (e.g. WASM component model)** — strong isolation, portable; ecosystem maturity and host-integration cost.
4. **Tiered** — built-in (full trust) / signed (out-of-process) / community (sandboxed, default-deny).

## Arguments
- Requirements are clear: **default-deny capabilities, untrusted-until-verified output, language-agnostic boundary, crash isolation.**
- The mechanism trade-off (subprocess vs sandbox as *primary*) is genuinely open and should not be pre-decided in the language ADR.

## Open questions
- Which mechanism is *primary* vs a hardening track?
- How are the extension contract and ports versioned (semver; pre-1.0 breakage policy)?

## Current recommendation
**Decide nothing on mechanism yet.** Keep [extensibility.md](../02-architecture/extensibility.md) stating only the requirements. Make the mechanism + versioning a dedicated ADR at the extensibility milestone, decoupled from the language ADR. PROVISIONAL (requirements) / UNDECIDED (mechanism).

## Status
**RESOLVED** → [ADR-0008](../03-adrs/ADR-0008-extension-isolation-and-contract-versioning.md) (Accepted 2026-07-21), per this page's own recommendation: no mechanism or versioning policy is chosen; the residual mechanism claim in [ADR-0001](../03-adrs/ADR-0001-language-and-toolchain.md) (clause 4 and its downstream Consequences/"Harder"/Future-ADR-Dependencies text) is corrected in place. The mechanism choice (which of this page's four alternatives, or another) remains genuinely open, deferred to a real third-party extension request — not decided by this resolution, only formally deferred.
