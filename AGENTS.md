# AGENTS.md — Repository Instructions for AI Agents

> Provider-agnostic instructions for any AI agent working in this repository. `CLAUDE.md` is a symbolic link to this file. **Do not add provider-specific instruction files;** all agent guidance lives here.

## Mission

Foundry turns human intent into engineering outcomes that can be **trusted** (justified, accountable, recorded) and that **compound** (the project learns from each one). See [docs/00-overview/vision.md](docs/00-overview/vision.md). The model is replaceable substrate, never the point.

## This repository is documentation-first

The documentation is the source of truth; code implements it. Before acting, read the docs in precedence order. Do not infer the architecture from source code or from archived files.

## Documentation hierarchy & decision precedence

Read and trust documents in this order. **When two active documents disagree, the higher one wins; the lower must be updated or archived — never leave a contradiction.**

1. `README.md` — entry point & navigation
2. `docs/00-overview/` — vision, principles, glossary, roadmap (canonical current truth)
3. `docs/01-rfcs/` — foundational decisions & reasoning
4. `docs/03-adrs/` — specific **accepted** decisions (binding; architecture must not contradict them)
5. `docs/02-architecture/` — what the system is, split by responsibility
6. `docs/05-reference/` — canonical terminology, concept map, invariants
7. `docs/04-guides/` — how to work in the repo
8. `docs/06-open-questions/` — **active deliberation; NON-CANONICAL.** Read for context on unresolved questions; never cite as a decision or write its recommendations into canonical docs.
9. `docs/archive/` — **historical only; ignore unless explicitly asked**

## Canonical sources (where to look for X)

| You need… | Go to |
|---|---|
| What a term means | `docs/05-reference/terminology.md` (the *only* place terms are defined) |
| The rules that must always hold | `docs/05-reference/invariants.md` |
| What the domain is | `docs/02-architecture/domain.md` |
| How outcomes are produced | `docs/02-architecture/execution.md` |
| How trust works | `docs/02-architecture/trust.md` |
| What durable knowledge is | `docs/02-architecture/knowledge.md` |
| What can be extended | `docs/02-architecture/extensibility.md` |
| System boundaries | `docs/02-architecture/system-context.md` |
| What is decided vs still open | `docs/03-adrs/README.md`, `docs/00-overview/roadmap.md` |

## Maturity rules (do not hide uncertainty)

Every major concept carries a **maturity** level (defined in `docs/04-guides/documentation.md`; status index in `docs/05-reference/concepts.md`):

- **CANONICAL** — accepted; must be followed. **Currently nothing is CANONICAL** — the ceiling is PROVISIONAL until a governance process exists (`docs/06-open-questions/OQ-006-governance-model.md`).
- **PROVISIONAL** — current best understanding; build on it, but never present it as settled truth.
- **OPEN QUESTION** — unresolved; lives in `docs/06-open-questions/`.
- **REJECTED** — historical only.

Rules:
- **Never silently upgrade a PROVISIONAL or OPEN QUESTION into canonical-sounding prose.** Preserve the qualifier ("current working model", "proposed", "not yet ratified").
- The **Act** model and the domain vocabulary are PROVISIONAL working hypotheses, not architectural truth. The *Act* and *Engine* nouns were coined in this project's own reasoning, not drawn from an accepted document.
- Maturity is orthogonal to precedence: a higher-precedence doc still wins a conflict, but winning does not make it CANONICAL.

## Terminology rules

- Use **canonical terms only**, as defined in `docs/05-reference/terminology.md`.
- **Never** use retired terms in new content: *Workflow, Stage, Provider, Skill, Runtime/Kernel*. The canonical replacements are *Act, Step, Executor, (reusable Act template), Engine*.
- The fundamental domain unit is the **Act**. The **Pipeline** is one **Strategy**, not the center. A **model** is one **Executor**, not a domain concept.

## Documentation rules

- One concept = one document. Definitions exist once; reference, never duplicate.
- Active docs describe only the current system. Do not update obsolete docs — archive them (git preserves history).
- Architecture files each answer exactly one question; do not mix concerns.

## Contribution rules

- Every change leaves the repository building, tested, and usable (vertical slices, no big-bang).
- Honor the [invariants](docs/05-reference/invariants.md); a change that breaks one is wrong.
- Build the deterministic core before integrating any model.

## How to resolve conflicting documents

1. Identify each document's precedence level (above).
2. The higher-precedence statement is authoritative.
3. Update or archive the lower one so the contradiction is removed.
4. If the conflict is between things of equal precedence or reflects an open architectural question, **do not resolve it silently** — surface it (see `docs/00-overview/roadmap.md` open decisions). There is no ratified governance process yet; escalate rather than assume.

## Historical isolation (hard guarantees)

These are guarantees, not suggestions:

1. **Canonical documents never contain retired vocabulary** except in an explicit "this term is retired" mapping (the table in `terminology.md`, `glossary.md`, `concepts.md`). If you see *Workflow, Stage, Provider, Skill, Runtime/Kernel* used as live vocabulary in an active doc, that is a defect — fix or report it.
2. **Active and historical content never mix in one document.** Archived material stays under `docs/archive/`; active docs may *link* to it for derivation, but must not import its terminology or conclusions as current.
3. **`docs/archive/` is historical context only.** Do not use it to understand the current system. Do not treat review conclusions there as decisions — reviews were never canonical.
4. **Obsolete docs are archived, never updated in place.** If an active doc becomes wrong, move it to `archive/`; git preserves history.

Only consult `docs/archive/` when explicitly asked, or to trace *why* a current decision was made.
