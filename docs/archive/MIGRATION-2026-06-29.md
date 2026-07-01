# Documentation Refactor — Record (2026-06-29)

> Historical record of the documentation-first refactor. Not canonical. Kept so the integration of archived reviews into canonical docs is traceable.

## What changed

The repository was refactored (not migrated) from a set of evolving documents — a monolithic `ARCHITECTURE.md`, a detailed roadmap, a single RFC, one ADR, and eight Architecture Review Board reviews — into a documentation-first structure with a single source of truth per concept. The refactor preserved **knowledge**, not **file boundaries**: concepts were extracted, merged, de-duplicated, and moved to single owning documents; obsolete material was archived; terminology was canonicalized.

The defining knowledge change: the canonical domain model is now the **Act** model (a justified, accountable transition of project state), the latest first-principles conclusion — superseding the original *Workflow/Stage/Provider* model and reframing the *Pipeline/Node/Executor* model as execution mechanism below the domain line.

## Old → New mapping

| Old file | New location(s) |
|---|---|
| `docs/ARCHITECTURE.md` | **Archived** → `archive/obsolete/`. Content extracted: domain → (replaced by) `02-architecture/domain.md`; execution → `02-architecture/execution.md`; knowledge → `02-architecture/knowledge.md`; trust/determinism → `02-architecture/trust.md`; extension → `02-architecture/extensibility.md`; boundaries/deployment → `02-architecture/system-context.md`; glossary/terms → `05-reference/terminology.md`; principles → `00-overview/principles.md` |
| `docs/IMPLEMENTATION-ROADMAP.md` | **Archived** → `archive/obsolete/`. Milestones → `00-overview/roadmap.md`; build approach/testing/CI/structure → `04-guides/development.md`; release notes → `04-guides/release.md` |
| `docs/rfcs/RFC-0001-vision-and-product-philosophy.md` | **Moved** → `01-rfcs/` (kept as the decision record). Distilled into `00-overview/vision.md` + `00-overview/principles.md` |
| `docs/adrs/ADR-0001-language-and-toolchain.md` | **Moved** → `03-adrs/`; indexed in `03-adrs/README.md` (with pending-amendment note) |
| `docs/reviews/RFC-0001-review-round-1.md` | **Archived** → `archive/reviews/`. Accepted ideas integrated: open governance/cost/adoption items → `00-overview/roadmap.md`, `00-overview/principles.md` |
| `docs/reviews/pre-implementation-adr-gate.md` | **Archived**. ADR backlog → `03-adrs/README.md` |
| `docs/reviews/ADR-0001-review.md` | **Archived**. Pending-amendment + isolation-mechanism caveat → `03-adrs/README.md`, `02-architecture/extensibility.md` |
| `docs/reviews/ADR-0001-change-list.md` | **Archived**. Same caveats integrated as above |
| `docs/reviews/architecture-consistency-review.md` | **Archived**. Contradictions resolved into canonical terminology + invariants |
| `docs/reviews/architecture-freeze-review.md` | **Archived**. Compatibility surfaces → `03-adrs/README.md`; open questions → `00-overview/roadmap.md` |
| `docs/reviews/greenfield-architecture-review.md` | **Archived**. Execution model & boundaries → `02-architecture/execution.md`, `system-context.md` |
| `docs/reviews/irreducible-domain-model.md` | **Archived**. The Act model → `02-architecture/domain.md`, `05-reference/terminology.md` |

## Archived documents

`archive/obsolete/ARCHITECTURE.md`, `archive/obsolete/IMPLEMENTATION-ROADMAP.md`, and all eight reviews under `archive/reviews/`.

## Newly created canonical documents

`README.md`; `AGENTS.md` (expanded) + `CLAUDE.md` (symlink); `docs/00-overview/{vision,principles,glossary,roadmap}.md`; `docs/02-architecture/{domain,execution,knowledge,trust,extensibility,system-context}.md`; `docs/03-adrs/README.md`; `docs/04-guides/{contributing,development,documentation,release}.md`; `docs/05-reference/{terminology,concepts,invariants}.md`; archive READMEs.

## Terminology changes (canonicalized)

| Retired | Canonical | Kind |
|---|---|---|
| Workflow / WorkflowDefinition / WorkflowRun | **Act** | domain |
| Stage | **Step** | mechanism |
| Provider | **Executor** (a model is one Executor) | mechanism / substrate |
| Skill | reusable **Act** template | derivative |
| Runtime / Kernel | **Engine** | mechanism |
| (Pipeline as the center) | **Pipeline = one Strategy** | demoted |

## Unresolved questions requiring human decisions

Carried forward, unresolved, into `00-overview/roadmap.md`: governance/ratification process; principle priority ordering; Act-vs-Knowledge center; cross-version replay scope; validator-determinism limits; Record durability classification; Authored-Knowledge format/migration; extension isolation mechanism & versioning; cost as a constraint and near-term value; concurrency/scale; the project name.
