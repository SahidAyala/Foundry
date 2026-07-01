---
description: Create logical commits following conventional commits and this project's module scopes
---

Commit the worktree changes grouped by **logical unit**, not by file.

## Step 1: Check for changes

Run:

git status --short

If there are no changes, inform the user and stop.

---

## Step 2: Analyze changes

Review the full diff:

git diff

and detect:

- affected Go packages (`domain/<package>`, future `internal/<package>`, `cmd/<binary>`)
- Go tests (`*_test.go`)
- documentation sections (`docs/00-overview`, `docs/01-rfcs`, `docs/02-architecture`, `docs/03-adrs`, `docs/04-guides`, `docs/05-reference`, `docs/06-open-questions`, `docs/archive`)
- repo entry points and agent config (`AGENTS.md`, `CLAUDE.md`, `README.md`, `.claude/`)
- build/tooling files (`go.mod`, `go.sum`, `.gitignore`, CI config)

Group files by **logical unit**.

---

## Step 3: Generate commits

For each group, generate a commit following:

type(scope): description

### Allowed types

feat
fix
refactor
test
chore
docs

### Scope

Derived from the affected area:

domain/act.go            → domain
internal/<package>/...   → <package>
cmd/<binary>/...         → <binary>
docs/00-overview/*        → overview
docs/01-rfcs/*             → rfcs
docs/02-architecture/*     → architecture
docs/03-adrs/*             → adrs
docs/04-guides/*           → guides
docs/05-reference/*        → reference
docs/06-open-questions/*   → open-questions
docs/archive/*             → archive
AGENTS.md, CLAUDE.md, README.md, .claude/ → repo
go.mod, go.sum             → build

Examples:

feat(domain): Add Act aggregate with status transitions
test(domain): Add unit tests for Act status transitions
docs(architecture): Document the execution model for Acts
docs(adrs): Record decision on language and toolchain
chore(repo): Update agent instructions and project entry points

---

## Step 4: Create commits

For each group:

git add <files>
git commit -m "generated message"

**Never add**

Co-Authored-By

---

## Step 5: Confirm

Show:

git log --oneline -10

---

## Important rules

- Do not create commits per file
- Group related changes together
- Maximum **5 commits**
- Always use `type(scope): description`
- Keep a Go package's implementation and its `_test.go` changes in the same commit unless the tests are a deliberate follow-up commit
- Never use retired terminology in commit messages (`Workflow`, `Stage`, `Provider`, `Skill`, `Runtime`/`Kernel`) — use the canonical terms instead (`Act`, `Step`, `Executor`, `Engine`), per `AGENTS.md`
- Docs changes that only fix a lower-precedence doc to resolve a contradiction with a higher one (see `AGENTS.md` precedence order) should be a separate `docs` commit from unrelated content additions
- If there are sensitive files (`.env`, secrets, credentials), warn before continuing
