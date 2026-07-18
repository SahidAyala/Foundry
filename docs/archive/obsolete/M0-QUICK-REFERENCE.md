# M0 Quick Reference

> **Status: Historical — M0 is complete.** See [../../00-overview/roadmap.md](../../00-overview/roadmap.md) for current status per milestone; this checklist's boxes predate that work and are left unchecked on purpose, as a record of the original plan.

Condensed checklist for the implementation engineer. **Read [M0-IMPLEMENTATION-BACKLOG.md](M0-IMPLEMENTATION-BACKLOG.md) for full details.**

## One-line summary

**13 PRs, 4 milestones, one architecture proven end-to-end — Acts made trustworthy and recorded.**

---

## PR checklist (sequential)

### Pre-implementation

- [ ] **PIC-1** — Pin Atlas build/test commands (external decision)
- [ ] **PIC-2** — Confirm provider + key strategy (external decision)
- [ ] **PIC-3** — Create sample Atlas repo for testing (one-time fixture)

### M0.0: Walking Skeleton (PRs 001–007)

| # | Goal | Key files | Est. effort |
|---|---|---|---|
| **001** | Domain types (pure) | `domain/act.go` | 2h |
| **002** | Record (filesystem) | `record/store.go` | 3h |
| **003** | Validators & Gate | `verify/validator.go`, `verify/gate.go` | 2h |
| **004** | Workspace (git branch) | `workspace/workspace.go` | 2h |
| **005** | Engine + Scripted Executor | `engine/engine.go`, `executor/scripted.go` | 4h |
| **006** | CLI `foundry do` | `cli/cli.go`, `cmd/foundry/commands/do.go` | 2h |
| **007** | Human approval | `cli/approval.go` | 1h |
| | **Milestone: Walking Skeleton** | **1 deterministic Act end-to-end** | **16h** |

### M0.1: Real Work (PRs 008–010)

| # | Goal | Key files | Est. effort |
|---|---|---|---|
| **008** | Naive Context | `context/gatherer.go` | 2h |
| **009** | Real Executor | `executor/provider.go` | 3h |
| **010** | Budget & Iteration | `engine/budget.go` | 2h |
| | **Milestone: Real Work** | **Produce real code with cost bounds** | **7h** |

### M0.2: Repair (PR-011)

| # | Goal | Key files | Est. effort |
|---|---|---|---|
| **011** | Repair Loop (1 attempt) | `engine/repair.go` | 2h |
| | **Milestone: Repair** | **Survive test failures (once)** | **2h** |

### M0.3: Usable (PRs 012–013)

| # | Goal | Key files | Est. effort |
|---|---|---|---|
| **012** | History (`log`, `show`) | `cli/commands/log.go`, `cli/commands/show.go` | 2h |
| **013** | Context & Diff polish | Enhanced `context/gatherer.go`, diff rendering | 2h |
| | **Milestone: Usable** | **Daily-use tool with history** | **4h** |

**Total estimated: 31 hours of focused, sequential work.**

---

## Every PR must have

- [ ] **Go code that compiles** (`go build ./cmd/foundry`)
- [ ] **Tests that pass** (`go test -race ./...`)
- [ ] **One responsibility** (no scope creep; see backlog for what's deferred)
- [ ] **Clear acceptance criteria** (copied from backlog; adapt as needed)
- [ ] **No live API calls in CI** (cassettes for PR-009+)
- [ ] **Main branch stays releasable** (green CI required to merge)

---

## Key decisions before you start

1. **Module path:** Provisional `foundry`; will change post-naming (ADR-0001 OQ-3)
2. **Provider choice** (PIC-2): Which LLM backend for PR-009? Set now; PR-009 implements it.
3. **Budget constants** (PR-010): Start at MaxIterations=2, MaxCostUSD=$1.00; tunable post-M0.0.
4. **Sample repo** (PIC-3): Build one if Atlas's CI is too heavy; version-control it, never change it during M0.0 tests.

---

## The invariants you cannot break

From [../05-reference/invariants.md](../05-reference/invariants.md); these are non-negotiable:

- [ ] Control flow is owned by Engine, never by a model
- [ ] Outputs are untrusted until verified
- [ ] The Record is immutable and durable
- [ ] Acts carry Evidence (considered + checked)
- [ ] Authority is always human (for now)
- [ ] Acts freeze after Judgment (no mutation after)

---

## CI setup (one-time)

- [ ] Linter: golangci-lint (or lighter config if not available)
- [ ] Branch protection: require green CI on `main`
- [ ] Conventional commits: scope:[a-z-]+; type: feat|fix|refactor|test|docs
- [ ] No live API calls in PR CI (use cassettes from PR-009 onward)

---

## Testing discipline

**By milestone:**

| Phase | What to test | How |
|---|---|---|
| **M0.0** | Full Act lifecycle | Golden test: `foundry do` → recorded Act matches snapshot |
| **M0.1+** | M0.0 regression | Rerun M0.0 golden test with scripted executor on every PR |
| **M0.1+** | Real executor | Cassette-based integration tests (no live API in CI) |
| **M0.2+** | Repair flow | Test both "repair succeeds" and "repair fails" paths |
| **M0.3+** | History | Test `log` and `show` commands against recorded Acts |

---

## Rollback strategy

If you get stuck:
1. **Undo the last commit** (`git reset --soft HEAD~1`) and iterate
2. **Do not force-push to main** (it's protected; blocked by CI anyway)
3. **If a PR is half-baked:** close it, start over on a new branch; cheap before merge

---

## Success signals

- [ ] `foundry do` works end-to-end (runs a test, shows results)
- [ ] Acts are recorded to disk
- [ ] A human sees the patch and can approve it
- [ ] Validator failures are captured in Evidence
- [ ] Repair loop works (failure → retry → pass or final fail)
- [ ] `foundry log` shows past Acts
- [ ] M0.0 golden test never regresses (runs on every subsequent PR)

---

## Common questions

**Q: Can I merge multiple PRs at once?**  
A: No. Sequential; each unblocks the next. Parallelism only in documentation/CI setup (external).

**Q: Can I skip a PR?**  
A: No. Each is a prerequisite. But see the backlog's §11 "Simplification challenge" — if you think two PRs should merge, discuss before starting.

**Q: What if the sample repo is too simple?**  
A: Iterate its complexity (add a test, add config, add multi-file edits). Keep it under version control; never change it during M0.0 tests.

**Q: How do I debug a failed golden test?**  
A: Compare the generated Act JSON to the snapshot. Print diffs. The invariants are: Intent matches, Evidence includes checks, Judgment verdict is deterministic.

**Q: Can I use cgo?**  
A: No. M0 is pure-Go (`CGO_ENABLED=0`). ADR-0001 details the boundary.

---

## Links

- **Full backlog:** [M0-IMPLEMENTATION-BACKLOG.md](M0-IMPLEMENTATION-BACKLOG.md)
- **M0 strategy:** [m0-plan.md](m0-plan.md)
- **Domain model:** [../02-architecture/domain.md](../02-architecture/domain.md)
- **Execution model:** [../02-architecture/execution.md](../02-architecture/execution.md)
- **Language decision:** [../03-adrs/ADR-0001-language-and-toolchain.md](../03-adrs/ADR-0001-language-and-toolchain.md)
- **Invariants:** [../05-reference/invariants.md](../05-reference/invariants.md)

---

_This is a battle plan. Read it, execute it, iterate if needed. No exploration, no framework-building, no speculation. Proof is in the Act._
