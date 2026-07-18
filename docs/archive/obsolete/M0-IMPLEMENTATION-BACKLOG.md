# M0 Implementation Backlog

> **Executable roadmap for M0:** each PR produces working software that leaves the repository compilable. Written for sequential execution. Refer to [m0-plan.md](m0-plan.md) for strategy and [../02-architecture/](../02-architecture/) for domain concepts.

**Repository structure after M0:** See [m0-plan.md §5](m0-plan.md#5-repository--package-structure). The canonical module path is provisional (ADR-0001 Open Question 3); these PRs use `foundry` as a placeholder.

---

## Pre-implementation checklist (one-time, before PR-001)

These are external decisions, not code PRs. Complete these first.

### PIC-1: Pin Atlas build & test commands

**Goal:** Establish the deterministic validators that `verify` will execute.

**Action:**
- Identify the Atlas repository (or a lightweight sample repo for M0.0 testing)
- Document the exact commands that run Atlas's build and test (e.g., `go test ./...`, `make build`)
- Create a tiny test fixture repo if Atlas's own CI commands are too heavy for golden-test regression
- Store these in a config file (not yet integrated; just documented for reference in PR-006)

**Outcome:** `build_cmd` and `test_cmd` are known; later PRs embed them as constants.

---

### PIC-2: Confirm provider & key strategy

**Goal:** Settle how M0 will access the real executor (LLM) in M0.1.

**Action:**
- Choose the single M0 provider (e.g., Claude API, OpenAI, etc.) based on team preference
- Decide key read strategy: env var name (e.g., `FOUNDRY_ANTHROPIC_KEY`), config file path, or both
- Document in a comment in PR-006 (the real executor PR) where the key is read from

**Outcome:** The real executor implementation knows how to initialize its client.

---

### PIC-3: Create sample Atlas repo for M0.0 golden tests

**Goal:** Provide a deterministic, unchanging test fixture for the walking skeleton.

**Action:**
- Create a tiny Go or other-language project in `test/fixtures/atlas-sample/` with:
  - A simple buildable file (e.g., a main.go or a Makefile)
  - A test that initially passes
  - A known, scripted breakage that M0 will "fix" as a test case
- Version-control it; treat it as immutable for the duration of M0.0 regression testing
- Document the exact build/test commands that run on it

**Outcome:** M0.0 integration tests run against a known repo without network.

---

## M0.0: Walking Skeleton (PRs 001–007)

The goal: produce one fully deterministic Act end-to-end, prove the architecture, pass all tests with zero network.

### PR-001: Domain types (pure, no imports except stdlib)

**Goal:** Foundational data structures that everything else builds on.

**Motivation:** The Act lifecycle is the architecture. Coding it first forces clarity on every concept and unblocks all other work.

**Files created:**
- `cmd/foundry/main.go` — empty `main`, just enough for `go build` to work
- `go.mod`, `go.sum` — module root
- `.gitignore` — standard Go
- `internal/domain/act.go` — Act, Intent, Outcome, Evidence, Judgment, Authority, Budget
- `internal/domain/act_test.go` — constructor tests, immutability checks
- `Makefile` — `make build`, `make test`

**Files modified:** None.

**Public APIs introduced:**
```go
// Act lifecycle types in package domain
type Act struct {
  ID          string
  Intent      Intent
  Evidence    Evidence
  Outcome     *Outcome
  Judgment    *Judgment
  CreatedAt   time.Time
}

type Intent struct {
  Text   string
  Author string
}

type Evidence struct {
  Considered []string // what was assembled
  Checked    []string // what was verified
}

type Outcome struct {
  Patch string // unified diff
}

type Judgment struct {
  Verdict   string // "pass", "fail", "repair"
  Authority Authority
  At        time.Time
}

type Authority struct {
  User string
}

type Budget struct {
  MaxIterations int
  MaxCostUSD    float64
}
```

**Acceptance criteria:**
- `go build ./cmd/foundry` succeeds
- `go test ./internal/domain` passes
- All domain constructors validate pre-conditions (e.g., Intent.Text is not empty)
- Acts cannot be mutated after creation (freeze semantics; expose via immutable accessors)
- Race detector: `go test -race ./...` passes

**Tests to write:**
- Unit: domain constructors with valid/invalid inputs
- Unit: Evidence accumulation (can append, cannot mutate past entries)
- Unit: Judgment verdict validation (only "pass", "fail", "repair" allowed)
- Property: Act is a pure value type (serializable to/from JSON round-trip)

**What becomes possible after:**
- All subsequent packages depend only on `domain` and stdlib
- Record can serialize Acts to disk
- Tests can construct Acts and assert on their shape

**Intentionally NOT implemented yet:**
- Act mutation / phase transitions (will come in Engine)
- Serialization (JSON tag boilerplate added; actual Marshaler in PR-002)
- Strategic interpretation of Evidence or Judgment (that is Engine logic)
- Any type with mutable state

---

### PR-002: Record — filesystem-backed immutable Act store

**Goal:** Persistence from day one; Acts written are never lost.

**Motivation:** The Record is the source of truth. Implementing it early forces the serialization contract to be right from the start and gives all later work a golden-test surface.

**Files created:**
- `internal/record/store.go` — Recorder interface + FileStore implementation
- `internal/record/store_test.go` — write/read roundtrip, content addressing, immutability
- `internal/record/json.go` — Act → JSON encoder (for durability)

**Files modified:**
- `internal/domain/act.go` — add `MarshalJSON()`, `UnmarshalJSON()` for full serialization

**Public APIs introduced:**
```go
// Recorder is the port declared by Engine (but implemented here)
type Recorder interface {
  Write(ctx context.Context, act *domain.Act) error
  Read(ctx context.Context, actID string) (*domain.Act, error)
  List(ctx context.Context) ([]*domain.Act, error)
}

// FileStore is the one real Recorder implementation for M0
type FileStore struct {
  root string // directory path
}

func NewFileStore(root string) (*FileStore, error)
```

**Acceptance criteria:**
- Acts are written to `<root>/<act.ID>/act.json` (one file per Act, immutable)
- Reading returns identical Act (JSON round-trip preserves all fields)
- Second write to same ActID fails (immutability enforced)
- `go test -race ./internal/record` passes
- FileStore creates the root directory if it does not exist
- List returns Acts in creation order (deterministic)

**Tests to write:**
- Unit: Write then Read returns identical Act
- Unit: Second Write to same ID fails with specific error
- Integration: Multiple Acts written and read back in sequence
- Golden: Act JSON shape is human-readable and stable (snapshot test)
- Property: Read(Write(act)) == act

**What becomes possible after:**
- Golden tests for the entire Engine can assert on persisted Acts
- The Record history becomes the source of truth for audit and replay (deferred, but the surface is ready)

**Intentionally NOT implemented yet:**
- Compression, batching, or optimization
- Deletion (Acts are immutable; cleanup is a later feature)
- Concurrency guarantees beyond filesystem atomicity
- Cloud/remote storage backends
- Knowledge store (deferred to M4)

---

### PR-003: Validators and Gate

**Goal:** Deterministic verification; run checks and make a machine verdict.

**Motivation:** Validators are the only M0 check that is not scripted — they run real commands against the Outcome. They must work standalone so Engine logic can be tested independently.

**Files created:**
- `internal/verify/validator.go` — Validator type, Run logic
- `internal/verify/gate.go` — Gate, verdict evaluation
- `internal/verify/validator_test.go` — runs shell commands, parses exit codes

**Public APIs introduced:**
```go
// Validator is one check (e.g. "run tests")
type Validator struct {
  Name string
  Cmd  string // shell command to run
}

// Gate evaluates validators and makes a verdict
type Gate struct {
  validators []*Validator
  rule       string // "all-pass" only, for M0
}

func NewGate(rule string, validators ...*Validator) (*Gate, error)

func (g *Gate) Evaluate(ctx context.Context, workspace string) (*domain.Judgment, error)
// Returns Judgment with Verdict in {"pass", "fail", "repair"}
```

**Acceptance criteria:**
- Validator.Run() executes shell command in specified directory
- Exit code 0 → finding passes; non-zero → fails
- Gate.Evaluate() with "all-pass" rule: all validators must pass
- Judgment carries validator output as Evidence
- `go test -race ./internal/verify` passes
- Tests work without network

**Tests to write:**
- Unit: Validator with passing command
- Unit: Validator with failing command
- Unit: Gate with mixed results (one pass, one fail)
- Unit: Gate with all-pass rule
- Golden: Judgment JSON shape

**What becomes possible after:**
- Engine can invoke validators as a pure function
- Tests can assert on Judgment verdicts

**Intentionally NOT implemented yet:**
- Validator routing or selection logic
- Complex verdict rules (only "all-pass" for M0)
- Repair-loop feedback (PR-010 adds that)
- Cost accounting (M0.1)
- Formatter output for the human Authority (PR-008 does that)

---

### PR-004: Workspace — git branch isolation

**Goal:** Safely apply patches to a project without mutating the original.

**Motivation:** Isolation is non-negotiable. Using a throwaway git branch is the simplest, most auditable approach.

**Files created:**
- `internal/workspace/workspace.go` — Workspace type, Apply, Clean
- `internal/workspace/workspace_test.go` — branch creation, patch application, cleanup

**Public APIs introduced:**
```go
// Workspace represents an isolated copy of a project
type Workspace struct {
  repoPath   string
  branchName string
  patchPath  string // unified diff file
}

func NewWorkspace(repoPath string, branchName string) (*Workspace, error)

func (w *Workspace) Apply(ctx context.Context, patch string) error
// patch is a unified diff string; git apply it to the branch

func (w *Workspace) Clean(ctx context.Context) error
// Delete the throwaway branch
```

**Acceptance criteria:**
- NewWorkspace checks that repoPath is a valid git repo
- Apply writes patch to a temp file, runs `git apply`, reports conflicts as errors
- Clean deletes the branch and returns original state
- Two concurrent Workspaces with different branch names don't interfere
- `go test -race ./internal/workspace` passes (test uses real git commands; fixture is the sample repo from PIC-3)

**Tests to write:**
- Integration: Create workspace, apply valid patch, verify file contents
- Integration: Apply patch with conflict, verify error
- Integration: Two workspaces, both apply different patches, both clean without interference
- Unit: Branch name validation (no unsafe names)

**What becomes possible after:**
- Engine can safely experiment without touching the original repo

**Intentionally NOT implemented yet:**
- Worktree (git worktree); simpler branch isolation is enough for M0
- Stashing or conflict resolution (conflicts escalate to the human)
- Transactional semantics across multiple patches

---

### PR-005: Engine (scaffolding) + Scripted Executor

**Goal:** The Engine skeleton and the first (deterministic) Executor implementation.

**Motivation:** The Engine is the heartbeat of the system. The scripted executor proves that the entire Act lifecycle works without any nondeterminism or network. This PR is the architecture in action.

**Files created:**
- `internal/engine/engine.go` — Engine struct, Run method, Act lifecycle
- `internal/engine/ports.go` — Executor, Verifier, Recorder, Gatherer port interfaces (declared here)
- `internal/executor/scripted.go` — ScriptedExecutor (test fixture)
- `internal/engine/engine_test.go` — golden test: full Act lifecycle

**Public APIs introduced:**
```go
// Engine produces Acts
type Engine struct {
  recorder  domain.Recorder
  verifier  domain.Verifier
  gatherer  domain.Gatherer
  executor  domain.Executor
  // ... etc; see internal wiring in cmd/foundry
}

func NewEngine(
  recorder domain.Recorder,
  verifier domain.Verifier,
  gatherer domain.Gatherer,
  executor domain.Executor,
) *Engine

func (e *Engine) Run(ctx context.Context, intent *domain.Intent) (*domain.Act, error)
// Full Act lifecycle: gather, execute, verify, judge, record

// Executor is the port for executing work (one user: scripted; second user: real, in PR-006)
type Executor interface {
  Execute(ctx context.Context, intent *domain.Intent, context []string) (*domain.Outcome, error)
}

// Verifier runs validators and makes a verdict
type Verifier interface {
  Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error)
}

// Gatherer assembles Context
type Gatherer interface {
  Gather(ctx context.Context, intent *domain.Intent) ([]string, error)
}

// Recorder persists Acts
type Recorder interface {
  Write(ctx context.Context, act *domain.Act) error
  Read(ctx context.Context, actID string) (*domain.Act, error)
}

// ScriptedExecutor returns a fixed patch (deterministic for testing)
type ScriptedExecutor struct {
  patch string // hard-coded test patch
}

func (s *ScriptedExecutor) Execute(...) (*domain.Outcome, error)
// Always returns the same patch, deterministically
```

**Acceptance criteria:**
- Engine.Run() follows the full Act lifecycle: gather → execute → verify → judge → record
- ScriptedExecutor returns a fixed patch (deterministic)
- Full golden test: Intent → Act with recorded Evidence and Outcome
- Act is persisted to Record after successful Judgment
- `go test ./internal/engine` passes; test is deterministic (no flakes)

**Tests to write:**
- Golden: Full Act lifecycle with scripted executor → recorded Act JSON
- Unit: Engine with failing validator → Judgment.Verdict == "fail"
- Unit: Engine with passing validator → Judgment.Verdict == "pass"
- Integration: Engine Run + Record.Read returns identical Act (proves persistence works)
- Invariant: After Run, the Act is immutable (cannot be modified)

**What becomes possible after:**
- The walking skeleton runs end-to-end
- All engine logic can be tested in isolation by providing mock adapters

**Intentionally NOT implemented yet:**
- Repair loop (PR-010)
- Budget enforcement (M0.1)
- Human approval / Authority capture (PR-008 adds that)
- Cost tracking
- Any real Executor (PR-006)
- Adaptive or multi-step Strategies (M2+)

---

### PR-006: CLI scaffolding + `foundry do` command (without approval)

**Goal:** Bootstrap the command-line interface; wire domain → engine → output.

**Motivation:** The CLI is the surface. Once it is wired, the user can actually run the tool and see the Act lifecycle in action.

**Files created:**
- `internal/cli/cli.go` — CLI struct, ParseArgs
- `cmd/foundry/main.go` — bootstrap, dependency injection
- `cmd/foundry/commands/do.go` — `foundry do` implementation

**Public APIs introduced:**
```go
// CLI parses and executes commands
type CLI struct {
  // wired at startup in main
}

func (c *CLI) Do(ctx context.Context, intent string, repoPath string) error
// Parse intent, run Engine, display results
// NO human approval yet
```

**Acceptance criteria:**
- `foundry do "add a feature" --repo <path>` executes the full Act lifecycle
- Results are printed to stdout (Act ID, Intent, Outcome patch, Judgment verdict)
- Exit code reflects Judgment.Verdict (0 for pass, 1 for fail)
- `go build ./cmd/foundry` produces `./foundry` binary
- Help: `foundry do --help` shows usage

**Tests to write:**
- Integration: `foundry do` with scripted executor, verify output format
- Unit: CLI argument parsing
- Integration: Full CLI → Engine → Record → output

**What becomes possible after:**
- The tool is executable end-to-end
- Humans can see what M0.0 does

**Intentionally NOT implemented yet:**
- Human approval (next PR)
- Diff rendering (M0.3 adds pretty printing)
- History inspection (`log`, `show` commands)
- Real Executor
- Budget caps
- Repair-loop interaction

---

### PR-007: Human approval + Authority capture

**Goal:** The Judgment must be accepted by a human Authority before the Outcome is applied.

**Motivation:** This is the trust boundary. The human must see the evidence and make the call.

**Files created:**
- `internal/cli/approval.go` — prompt user, capture Authority
- `cmd/foundry/commands/do.go` — updated to show diff and prompt

**Public APIs introduced:**
```go
// PromptForApproval shows the Outcome and prompts the user
func PromptForApproval(outcome *domain.Outcome, evidence *domain.Evidence) (*domain.Authority, error)
// Reads from stdin; returns Authority{User} or error if user declines
```

**Acceptance criteria:**
- After Engine.Run(), before record, CLI shows:
  - The proposed patch (diff)
  - The Judgment verdict
  - A prompt: "Approve and apply? (y/n)"
- If user enters 'y', Record the Act and apply the patch to the repo
- If 'n', exit without applying (Act is still recorded with "rejected" status — deferred to M0.2)
- Authority captures `os.Getenv("USER")` or `whoami`
- Judgment carries the Authority

**Tests to write:**
- Integration: Mock stdin, test 'y' path (apply)
- Integration: Mock stdin, test 'n' path (decline)
- Unit: Authority capture

**What becomes possible after:**
- M0.0 walking skeleton is complete
- The tool produces real, user-approved recorded Acts

**Intentionally NOT implemented yet:**
- Rejection handling (deferred; PR-010 makes rejection recovery possible)
- Batch approval
- Approval policies or delegation

---

## M0.0 Complete: Walking Skeleton

After PR-007:
- `foundry do "<intent>" --repo <sample-repo>` produces one Act end-to-end
- The Act is recorded immutably
- The patch is reviewed by a human
- All verification is deterministic (no network, no model)
- **Golden test:** run `foundry do` on sample repo, assert the recorded Act matches expected shape

**At this point**, the architecture is proven. The Act lifecycle works. Judgment is human-owned. Everything is recorded.

---

## M0.1: Real Work (PRs 008–010)

The goal: swap in a real Executor and capability to actually develop code (bounded by cost and iteration limits).

### PR-008: Naive Context Gatherer

**Goal:** Assemble Context from the Intent (naive: heuristics only).

**Motivation:** Executors (especially real ones) need input. For M0, a naive approach is enough: extract file names from the intent string, read them as context.

**Files created:**
- `internal/context/gatherer.go` — Context Gatherer implementation
- `internal/context/gatherer_test.go` — heuristic testing

**Public APIs introduced:**
```go
// NaiveGatherer satisfies the Gatherer port
type NaiveGatherer struct{}

func (ng *NaiveGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error)
// Extracts file names from intent string (regex pattern matching)
// Reads those files from the repo and returns their contents as strings
```

**Acceptance criteria:**
- Intent "add logging to main.go" → Gather returns contents of `main.go` from the repo
- Missing files → included in Evidence as "not found" (no hard error)
- Output is bounded (e.g., max 100KB of context per Act)
- `go test ./internal/context` passes

**Tests to write:**
- Unit: Intent with file names → Gather extracts them
- Unit: Intent with missing files → Gather reports as not found
- Integration: Gather against sample repo

**What becomes possible after:**
- Executors receive actual project context, not empty input

**Intentionally NOT implemented yet:**
- Semantic ranking or compaction
- Provenance metadata (who provided each piece of context)
- Knowledge-based context
- Budget accounting for context size
- Advanced heuristics (M4 adds semantic retrieval)

---

### PR-009: Real Executor (provider-backed)

**Goal:** Wire up a live LLM executor (first real, non-scripted implementation of the Executor port).

**Motivation:** M0.1 is where we move from "architecture proof" to "actually useful tool". This executor makes the tool capable of producing real code changes.

**Files created:**
- `internal/executor/provider.go` — ProviderExecutor implementation
- `internal/executor/provider_test.go` — cassette-based tests (no live calls in PR CI)

**Public APIs introduced:**
```go
// ProviderExecutor calls a real LLM (e.g., Claude API)
type ProviderExecutor struct {
  apiKey   string
  endpoint string
  // ...
}

func NewProviderExecutor(apiKey string) (*ProviderExecutor, error)

func (pe *ProviderExecutor) Execute(ctx context.Context, intent *domain.Intent, context []string) (*domain.Outcome, error)
// Call the provider's API, get a patch back, return Outcome
```

**Acceptance criteria:**
- ProviderExecutor reads API key from environment (from PIC-2)
- Execute assembles a prompt from Intent + Context, calls the provider API, parses the response for a patch
- Returns Outcome with Patch or error if API call fails
- All tests use cassettes (recorded responses); no live API calls in CI
- Test fixture: cassette for "add logging to main.go" on sample repo

**Tests to write:**
- Integration (cassette): Execute with real-looking Intent + Context → Outcome with valid patch
- Integration (cassette): API error → Execute returns error
- Unit: Prompt assembly (deterministic; snapshot test)
- Unit: Response parsing (handles various formats)

**What becomes possible after:**
- The tool can produce real code changes
- M0.1 validates Atlas integration

**Intentionally NOT implemented yet:**
- Streaming responses
- Tool use or multi-turn interaction
- Provider fallback/routing
- Cost accounting (comes in next iteration, PR-010)
- Timeout handling beyond context timeout
- Retry logic (handled at Engine level if needed later)

---

### PR-010: Budget & Iteration Limits

**Goal:** Prevent runaway costs and infinite loops; bounded repair.

**Motivation:** With a real executor, the tool can be expensive. M0.1 is unusable without hard limits.

**Files created:**
- `internal/engine/budget.go` — Budget enforcement logic
- Updated: `internal/engine/engine.go` — integrate budget checks

**Public APIs introduced:**
```go
// Budget is already a domain type; add enforcement logic
func (e *Engine) RunBudgeted(ctx context.Context, intent *domain.Intent, budget *domain.Budget) (*domain.Act, error)
// Enforces max iterations and cost cap; returns error if budget exceeded
```

**Acceptance criteria:**
- Budget.MaxIterations hardcoded to 2 for M0.1 (repair can happen once)
- Budget.MaxCostUSD hardcoded to $1.00 for M0.1 (safety cap; can be adjusted)
- Each Executor.Execute call increments iteration count and tracks cost estimate
- If budget exceeded, Engine halts and returns error (Judgment.Verdict == "budget-exceeded")
- Evidence includes cost tracking

**Tests to write:**
- Unit: Budget enforcement with max iterations
- Unit: Budget enforcement with cost cap
- Integration: Run to max iterations, verify verdict

**What becomes possible after:**
- M0.1 is safe to run on real projects
- The repair loop (below) can be added without risk of infinite cost

---

## M0.1 Complete: Real Work

After PR-010:
- `foundry do "add logging to main.go" --repo <atlas>` produces a real code change
- The change is verified (tests pass, build succeeds)
- The patch is reviewed by a human and approved
- Cost is bounded and tracked
- Everything is recorded

**At this point**, the tool is useful enough to develop on Atlas, one change at a time.

---

## M0.2: Repair (PR-011)

The goal: on validator failure, feed findings back to the executor once (bounded repair attempt).

### PR-011: Repair Loop (bounded to 1 attempt)

**Goal:** If validators fail, allow the executor to try to fix it once.

**Motivation:** Small changes often fail tests. One repair attempt makes the tool much more useful without opening the door to infinite loops.

**Files created:**
- `internal/engine/repair.go` — repair orchestration
- Updated: `internal/engine/engine.go` — integrate repair into Run

**Public APIs introduced:**
```go
// Engine.Run already exists; update to include repair:
// If Judgment.Verdict == "fail" and iterations < budget, attempt repair once.
// Feed validator findings back to Executor.Execute as part of Intent context.
```

**Acceptance criteria:**
- After Judgment with Verdict == "fail", if MaxIterations not reached:
  - Feed validator findings (what failed) back to Executor as additional context
  - Execute again (iteration 2)
  - Verify again
  - Judgment verdict becomes final
- If Verdict == "pass" after repair, apply the new Outcome
- If Verdict == "fail" after repair, final verdict is "fail" (cannot repair further)
- Iteration counter increments; budget is checked

**Tests to write:**
- Integration: First attempt fails, repair passes
- Integration: First attempt passes (no repair needed)
- Integration: Both attempts fail (final verdict = fail)
- Golden: Act with repair loop recorded, Evidence shows both iterations

**What becomes possible after:**
- The tool survives test failures
- Repair is bounded (exactly 1 attempt; prevents infinite loops)
- M0.2 is "usable" for small, straightforward changes

**Intentionally NOT implemented yet:**
- Multiple repair attempts (stay at 1 for M0)
- Repair strategy selection (straight re-run only)
- Repair-specific context or prompts (could come in M0.3)
- Advanced failure analysis or diagnosis

---

## M0.2 Complete: Repair

After PR-011:
- Validator failures trigger a bounded repair attempt
- If repair passes, the change is applied
- If repair fails, the human sees the final failure and decides
- Cost remains bounded

**At this point**, the tool is robust enough for daily use on simple changes.

---

## M0.3: Usable (PRs 012–013)

The goal: better context and history inspection (`foundry log`, `foundry show`).

### PR-012: History Inspection Commands

**Goal:** Users can review past Acts and re-run them.

**Motivation:** Audit and learning. Without history, the Record is write-only.

**Files created:**
- `cmd/foundry/commands/log.go` — `foundry log` command
- `cmd/foundry/commands/show.go` — `foundry show <act-id>` command
- Updated: `internal/cli/cli.go` — wire new commands

**Public APIs introduced:**
```go
// CLI commands
func (c *CLI) Log(ctx context.Context, limit int) error
// List all Acts (ID, Intent, Status, Timestamp)

func (c *CLI) Show(ctx context.Context, actID string) error
// Show full Act (Intent, Evidence, Outcome, Judgment)
```

**Acceptance criteria:**
- `foundry log` lists Acts (default last 10)
- `foundry log -n 50` lists last 50
- `foundry show <act-id>` shows the full Act (pretty-printed)
- Patch is shown as unified diff
- Evidence is readable (list of what was considered, what was checked)
- Both commands read from Record (filesystem store)

**Tests to write:**
- Integration: `foundry log` after several Acts recorded
- Integration: `foundry show <id>` for each recorded Act
- Unit: Diff pretty-printing (golden test)

**What becomes possible after:**
- Users have visibility into what the tool has done
- Debugging and learning from past decisions

**Intentionally NOT implemented yet:**
- Filtering or search
- Re-running past Acts
- Export (JSON, CSV, etc.)
- Remote history sync
- Knowledge extraction from history

---

### PR-013: Context & Diff Polish

**Goal:** Better naive context (repo structure awareness) and readable diffs.

**Motivation:** M0.3 is "usable daily". Better context means better outcomes. Readable diffs mean easier review.

**Files created:**
- Updated: `internal/context/gatherer.go` — add file-tree heuristics, README reading
- Updated: `internal/cli/commands/do.go` — add diff rendering

**Public APIs introduced:**
```go
// NaiveGatherer improvements:
// - Read README files as context
// - Include nearby files (same directory, related names)
// - Limit context size with priority (config files first, then docs, then code)
```

**Acceptance criteria:**
- Gatherer includes README.md if present
- Gatherer includes other files in the same directory as matched files
- Context size remains bounded (max 100KB)
- Diff output is formatted readably (colors in terminal, or ANSI codes)
- Judgment verdict is clear ("✓ pass", "✗ fail")

**Tests to write:**
- Integration: Gatherer against sample repo with README
- Golden: Diff rendering (snapshot test of formatted output)
- Unit: Context size enforcement

**What becomes possible after:**
- M0.3 is complete
- The tool is ready for daily use on Atlas

**Intentionally NOT implemented yet:**
- Semantic file selection (comes in M4)
- Custom diff formats (unified diff only)
- Interactive CLI (coming in later milestones)
- Configuration files for context hints

---

## M0.3 Complete: Usable

After PR-013:
- `foundry do` produces real code changes with one repair attempt
- Context is better (includes READMEs, nearby files)
- Diffs are readable and easy to review
- `foundry log` and `foundry show` let users inspect history
- The tool is usable daily on Atlas for small-to-medium changes

**At this point**, M0 ships. The entire Act lifecycle is real, human-approved, recorded, and useful.

---

## Post-M0.0 (M0.0 regression guard)

Every PR from M0.1 onward must pass a golden test that proves M0.0 still works:
- `foundry do "test intent" --repo <sample-repo>` with **scripted executor** produces a recorded Act
- The recorded Act matches the expected golden shape (JSON snapshot)
- This test runs in CI and never calls a live API

---

## Build & Test Discipline

**Every PR:**
- [ ] `go build ./cmd/foundry` succeeds
- [ ] `go test -race ./...` passes
- [ ] Linting: `golangci-lint run` (if configured)
- [ ] Golden tests are snapshot-tested (not checked-in in `*.json`; regenerated and asserted)
- [ ] No live API calls in CI (cassettes only)
- [ ] Repository remains in a state where a user can `git clone` and build immediately

**CI / Merge discipline:**
- Main branch is always releasable (green CI required)
- Conventional commits (scope: domain/engine/executor/verify/context/workspace/record/cli; type: feat/fix/refactor)
- `CHANGELOG.md` generated from commits (deferred; use for M1 release)

---

## Open Decisions During M0

These are not architecture questions (those are in [../06-open-questions/](../06-open-questions/)); these are implementation choices that M0 makes provisionally:

1. **Repair depth:** M0.2 allows 1 repair attempt. Could be 0 (strict), could be 3. Design call; 1 is a balance.
2. **Budget constants:** Max 2 iterations, $1.00 max cost. Safe defaults; can be config in M0.3.
3. **Context size limit:** 100KB. Tunable; golden tests will reveal if it's too small.
4. **Validator scope:** Only shell commands (Cmd field). No structured plugins yet (M6).
5. **Real executor choice:** Deferred to PIC-2. Once chosen (PR-009 implements it), it is the M0 executor.

---

## Success Criteria for M0

- [x] Architecture is proven end-to-end (Acts are produced, verified, judged, recorded)
- [x] All verification is deterministic (no model nondeterminism in M0.0)
- [x] Human Authority is always in the loop (approval required)
- [x] Record is immutable and complete (audit trail)
- [ ] At least one real, verified code change produced on Atlas by Foundry (M0.3 definition)
- [ ] Tool is usable daily for small-to-medium changes without crashing
- [ ] All tests pass with `-race` detector
- [ ] Repository is releasable (build artifacts, cross-platform, no scaffolding)

---

## Implementation Order Summary

A sequential checklist for an engineer:

1. **PIC-1/2/3:** Pin Atlas commands, provider choice, create sample repo
2. **PR-001:** Domain types
3. **PR-002:** Record (filesystem store)
4. **PR-003:** Validators & Gate
5. **PR-004:** Workspace (git branch isolation)
6. **PR-005:** Engine + Scripted Executor (first executable)
7. **PR-006:** CLI `foundry do` command
8. **PR-007:** Human approval → **M0.0 complete**
9. **PR-008:** Naive Context Gatherer
10. **PR-009:** Real Executor (provider-backed)
11. **PR-010:** Budget & Iteration Limits → **M0.1 complete**
12. **PR-011:** Repair Loop → **M0.2 complete**
13. **PR-012:** History Inspection (`log`, `show`)
14. **PR-013:** Context & Diff Polish → **M0.3 complete**

**Parallel work:** Documentation updates, CI setup, test fixtures. No parallel PRs in code.

---

## Risk Mitigation

| Risk | M0 Mitigation |
|---|---|
| Executor output unparseable | Cassettes + schema tests; PR-009 includes parser validation |
| Git apply failures | Workspace escalates to human; no auto-resolve |
| Context too small | Golden tests will fail; iterate context size in PR-008/013 |
| Budget too tight | Start at $1.00; increase if needed (constants are easy to adjust) |
| M0.0 regression | Golden test on every PR from M0.1 onward; runs with scripted executor |

---

## Acceptance Criteria Summary (per milestone)

### M0.0: Walking Skeleton
- [ ] `foundry do "test intent" --repo <sample>` runs end-to-end with scripted executor
- [ ] Human approves or declines patch
- [ ] Act is recorded immutably
- [ ] Golden test passes: recorded Act matches expected shape
- [ ] All tests pass with `-race`
- [ ] Repository builds and is releasable

### M0.1: Real Work
- [ ] Real Executor is integrated (provider-backed)
- [ ] Context Gatherer is naive but functional
- [ ] Budget is enforced (max 2 iterations, $1.00 cap)
- [ ] `foundry do` produces real code changes on sample repo
- [ ] M0.0 golden test still passes (regression guard)

### M0.2: Repair
- [ ] Validator failure triggers bounded repair (1 attempt)
- [ ] Cost stays within budget
- [ ] Final verdict is deterministic
- [ ] Golden test includes repair flow

### M0.3: Usable
- [ ] `foundry log` lists past Acts
- [ ] `foundry show <id>` displays full Act
- [ ] Context includes READMEs and nearby files
- [ ] Diffs are readable (formatted output)
- [ ] Tool is ready for daily use on Atlas

---

_This backlog is the execution plan for M0. It is sequentially ordered; each PR unblocks the next. No speculation, no scaffolding, no frameworks — just Acts and the code that produces them._
