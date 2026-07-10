# Foundry

> *Foundry is a working name and is provisional.*

**Foundry turns human intent into engineering outcomes that can be trusted — justified, accountable, and recorded — and that compound, because the project learns from each one.**

It is not an IDE, a chatbot, an autonomous agent, a prompt tool, or a wrapper around a model. It is a system for the **responsible evolution of a project's state**, in which the model is replaceable substrate and the durable value is the project's own process, knowledge, and auditable history.

## Why it exists

Generating code is commoditizing; the durable value is everything around it — the process, the **knowledge** it accumulates, the **verification** that makes output safe to depend on, and the **record** of how a system reached its current state. Better models make this layer *more* valuable, because they increase the volume of machine work that must be made trustworthy. Full reasoning: [docs/00-overview/vision.md](docs/00-overview/vision.md).

## High-level architecture (current working model)

> This is the project's **current working model**, not ratified truth. Its center is an open question ([docs/06-open-questions/](docs/06-open-questions/)); treat it as the best current understanding, not the final word.

The working model centers the domain on the **Act** — *a justified, accountable transition of project state*. Every capability (implement, review, design, secure, release, learn) is an Act. An Act carries an **Intent**, is produced by a pluggable **Strategy** (a predeclared pipeline is just one option), accumulates **Evidence** (what was considered and checked), yields an **Outcome**, and passes a **Judgment** owned by a human **Authority** — all preserved immutably and feeding the project's durable **Knowledge**.

The model, the pipeline, and the execution machinery are *substrate below the domain line* — replaceable, never the center. (This corrects an earlier model that centered on workflows and providers; see `docs/archive/` for history. Whether the *Act* itself is the right center is [OQ-001](docs/06-open-questions/OQ-001-domain-center.md).)

## Repository layout

```
README.md            ← you are here
AGENTS.md            ← instructions for AI agents (CLAUDE.md links to it)
docs/
  00-overview/       vision · principles · glossary · roadmap
  01-rfcs/           foundational decisions & reasoning
  02-architecture/   domain · execution · knowledge · trust · extensibility · system-context
  03-adrs/           accepted decisions (+ the backlog of open ones)
  04-guides/         contributing · development · documentation · release
  05-reference/      terminology (canonical) · concepts (+ maturity index) · invariants
  06-open-questions/ unresolved architecture — NON-CANONICAL deliberation
  archive/           historical only — obsolete docs, reviews, rejected RFCs
```

Every major concept carries a **maturity** level — CANONICAL / PROVISIONAL / OPEN QUESTION / REJECTED — so settled knowledge is never confused with current understanding or speculation. The status of every concept is indexed in [docs/05-reference/concepts.md](docs/05-reference/concepts.md). **Currently nothing is CANONICAL** — see Status below.

## How to navigate the documentation

Documentation is **single-source-of-truth**: every concept is defined once, in `docs/05-reference/terminology.md`; everything else references it. When documents disagree, the higher-precedence one wins:

**README → Overview → RFCs → Accepted ADRs → Architecture → Reference → Guides → Archive.**

`docs/archive/` is historical context only — do not treat it as current.

## Where to start reading

1. [docs/00-overview/vision.md](docs/00-overview/vision.md) — why Foundry exists
2. [docs/00-overview/principles.md](docs/00-overview/principles.md) — the values that govern every decision
3. [docs/02-architecture/domain.md](docs/02-architecture/domain.md) — the core model (the Act)
4. [docs/05-reference/terminology.md](docs/05-reference/terminology.md) — the canonical vocabulary
5. [docs/00-overview/roadmap.md](docs/00-overview/roadmap.md) — what is being built, and the open decisions that still need a human

## Install

**Requirements:** [Go](https://go.dev/dl/) 1.21+. `git` is also required for the one-line install below (not needed if you already have a local clone).

### One-line install (no clone needed)

```
curl -fsSL https://raw.githubusercontent.com/SahidAyala/Foundry/main/install.sh | bash
```

This fetches the source into a temporary directory, builds it, and removes the temporary directory afterward — nothing but the `foundry` binary is left on your machine.

### From a local clone

```
git clone git@github.com:SahidAyala/Foundry.git
cd Foundry
./install.sh
```

### What it does

Either way, the binary is installed to `/usr/local/bin` (override with `FOUNDRY_INSTALL_DIR=<dir>`; `sudo` is only invoked if that directory isn't writable). If the install directory isn't already on your `PATH`, the script prints the line to add to your shell profile (`~/.zshrc`, `~/.bashrc`, ...).

Once installed, run `foundry` from any directory to start an interactive session.

## Status

**Implementation active.** Architecture direction is set; M0 backlog is finalized and executable. Several foundational decisions remain open (governance, the precise center of the domain, replay scope, knowledge migration, and more) and are listed honestly in [docs/00-overview/roadmap.md](docs/00-overview/roadmap.md). Only the language/toolchain decision is accepted ([docs/03-adrs/ADR-0001-language-and-toolchain.md](docs/03-adrs/ADR-0001-language-and-toolchain.md)).

**Start here for implementation:** [docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md](docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md) (13 PRs, sequential) or [docs/04-guides/M0-QUICK-REFERENCE.md](docs/04-guides/M0-QUICK-REFERENCE.md) (one-page checklist).
