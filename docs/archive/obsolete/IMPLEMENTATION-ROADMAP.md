# Foundry — Implementation Roadmap (v0.1 → v1.0)

> **Status:** Proposed implementation plan. Derived from `docs/ARCHITECTURE.md` and `docs/rfcs/RFC-0001-vision-and-product-philosophy.md`. This document plans *how* to build; it does not redesign *what* to build.
>
> **Working assumptions (flagged, not decided here):**
> - **Language: Go.** Per `ARCHITECTURE.md §5.4`; ADR-0001 should ratify before PR-001 merges. If Go is rejected, §5 (layout) changes; the milestone/PR dependency logic does not.
> - **Detail tiers.** M0–M1 PRs are fully specified (the immediate build). M2 is medium detail. M3–M7 are epics with PR sketches. Near = contract; far = intention. We re-detail each milestone's PRs at the start of that milestone.
> - **Non-negotiable invariants** (from the implementation principles): every PR compiles, is tested, is independently reviewable, and leaves `main` in a working, usable state. No big-bang merges. Deterministic before AI. Runtime before providers.

---

## 0. Deviations from the Suggested Build Order (and why)

The suggested order is sound; four adjustments produce a cleaner dependency graph and better vertical slices:

1. **CLI skeleton moves to PR-002 (right after bootstrap).** Every subsequent feature becomes demonstrable through a real command. This is what makes each PR a *vertical slice* with visible user value rather than an invisible internal layer.
2. **YAML+schema is a shared foundation built before the config loader.** Config is itself YAML; building the parse+validate primitive once and having config consume it avoids two divergent YAML code paths.
3. **The Run Ledger + record/replay is pulled into M1 (v0.1), not deferred.** `ARCHITECTURE.md §22` is explicit that this *must* be in v0.1 — it is simultaneously the determinism story, the audit log, the resume mechanism, and the test harness. Building the runtime without it means retrofitting event-sourcing later, which is the expensive path.
4. **Artifact model precedes the Skill system.** Artifacts are the primitive output of *any* stage (a shell command produces one); skills are a higher-order construct that produces artifacts. Building the primitive first lets validators and gates operate before skills exist.

Net principle preserved: **build the deterministic execution engine completely before any AI touches it.** A user should be able to run gated, validated, replayable, resumable pipelines with zero providers configured — Foundry as a rigorous deterministic task runner — by the end of M1.

---

## 1. Milestone Roadmap

| Milestone | Version | Theme | "Usable system" exit state | Maps to ARCH phase |
|---|---|---|---|---|
| **M0** | v0.1-alpha | Walking skeleton | `foundry run` executes a trivial all-deterministic pipeline end-to-end and prints results | §22 v0.1 (start) |
| **M1** | v0.1 | Deterministic runtime | Gated, validated, budgeted pipelines with artifacts; runs are recorded, **replayed identically**, and resumable. No AI. | §22 v0.1 |
| **M2** | v0.2 | Skills + mock executor | Author declarative skills; run AI-*shaped* pipelines deterministically against a fixture/mock executor; bounded repair loops | §22 v0.2 |
| **M3** | v0.3 | Provider SDK + real executors | Router with capability negotiation, failover, budget gate; Anthropic + OpenAI + Ollama adapters; secrets in keychain; record/replay against real models | §22 v0.2 |
| **M4** | v0.4 | Knowledge + Context engine | Derived/Authored knowledge graph; AST/git/docs extractors; deterministic-first context with provenance, budget, caching; gated knowledge-update | §22 v0.3 |
| **M5** | v0.5 | Integration + visibility | Worktree isolation + diff/approve; GitHub adapter; ledger-grounded commit/PR; CI mode (same binary); OTel + cost metrics + local TUI dashboard | §22 v0.4 |
| **M6** | v0.6–v0.9 | Extensibility | Versioned ports + plugin SDK; out-of-process sandboxed plugins + capability permissions; minimal registry; per-port conformance suites | §22 v0.5–v0.9 |
| **M7** | v1.0 | Stability + hardening | Port contracts frozen under semver; security hardening + air-gapped mode; enterprise (remote ledger, RBAC/SSO reference); governance + docs | §22 v1.0 |

Each milestone is independently shippable and independently useful. A user could stop adopting at M1 and still have a valuable deterministic runtime; at M3 a valuable AI implement-loop; at M5 a full local-to-PR platform.

---

## 2. Repository Layout (target shape; grown incrementally, not scaffolded empty)

> We do **not** create empty directories ahead of need (that violates "avoid placeholder implementations"). This is the *destination*; each PR adds only the parts it uses. Hexagonal: `surfaces → kernel → ports ← adapters`. Nothing imports upward.

```
foundry/
├── cmd/foundry/                 # thin entrypoint; wires surfaces to kernel
├── internal/
│   ├── domain/                  # pure types, no I/O: Pipeline, Stage, Artifact,
│   │                            #   Finding, Gate, ContextBundle, Skill, Budget
│   ├── kernel/
│   │   ├── orchestrator/        # state machine, stage sequencing, repair loops, saga
│   │   ├── ledger/              # append-only event store, replay, resume
│   │   ├── budget/              # token/cost/time/iteration accounting + enforcement
│   │   └── gate/                # gate evaluation over validation reports
│   ├── ports/                   # interfaces ONLY: Executor, Validator, Context,
│   │                            #   Provider, Vcs, Extractor, Secrets, Observability
│   ├── adapters/
│   │   ├── executor/{shell,mock,anthropic,openai,ollama}/
│   │   ├── validator/{exitcode,shell}/   # + ecosystem wrappers later
│   │   ├── context/{static,resolver}/
│   │   ├── knowledge/{graph,extractors}/
│   │   ├── vcs/{git,github}/
│   │   ├── secrets/{env,keychain}/
│   │   └── observability/{stdout,otel}/
│   ├── config/                  # config loader (consumes schema/)
│   ├── schema/                  # YAML parse + JSON-schema validation (shared)
│   └── surfaces/
│       ├── cli/                 # cobra command tree (thin)
│       └── tui/                 # bubbletea (M5)
├── pkg/sdk/                     # PUBLIC contracts for plugin authors (M6)
├── api/proto/                   # gRPC plugin protocol (M6)
├── testdata/                    # fixtures, golden files, sample pipelines, cassettes
├── examples/                    # runnable example pipelines & skills
├── docs/                        # this dir
├── .github/workflows/           # CI
├── go.mod / Makefile / LICENSE
```

**Forward-compat decisions baked in early, cheaply:**
- *Ports are interfaces from M1*, but a port is only introduced when its **first** adapter exists, and only generalized when a **second** appears. No speculative abstraction.
- *Plugin authors never import `internal/`.* The public surface is `pkg/sdk` + `api/proto` (gRPC), so the eventual out-of-process, language-agnostic plugin story (M6) does not require restructuring the kernel.
- *The ledger is event-sourced from M1*, so audit/replay/resume/observability are projections, never separate subsystems.

---

## 3. Epic Breakdown

| Epic | Milestone | Description |
|---|---|---|
| **E0 — Project Foundation** | M0 | Repo, CI, CLI shell, config, YAML/schema |
| **E1 — Pipeline Model** | M0 | Domain types, DAG load, cycle detection, inspection |
| **E2 — Deterministic Runtime** | M0–M1 | Orchestrator, stage execution, built-in ops |
| **E3 — Run Ledger & Time-Travel** | M1 | Event store, replay, resume, run inspection |
| **E4 — Artifacts & Provenance** | M1 | Content-addressed artifact model & store |
| **E5 — Executors** | M1, M2, M3 | Executor port; shell → mock → real providers |
| **E6 — Verification** | M1 | Validator port, deterministic validators, gates |
| **E7 — Budgets & Repair** | M1 | Budget enforcement, bounded repair loops |
| **E8 — Skill System** | M2 | Skill manifest/contract, invocation, packs |
| **E9 — Context** | M2, M4 | Context port; static → full deterministic-first engine |
| **E10 — Provider SDK & Router** | M3 | Capability negotiation, routing, failover, secrets |
| **E11 — Knowledge Engine** | M4 | Graph, Derived/Authored split, extractors, knowledge-update |
| **E12 — VCS & CI Integration** | M5 | Worktree isolation, GitHub, grounded PR, CI mode |
| **E13 — Observability** | M5 | OTel traces, cost/metrics, local dashboard/TUI |
| **E14 — Extensibility** | M6 | Plugin SDK, versioned ports, sandbox, registry, conformance |
| **E15 — Hardening & Enterprise** | M7 | Semver freeze, security, air-gapped, remote ledger, RBAC |

---

## 4. Pull Request Plan

### M0 — Walking Skeleton (fully specified)

---

#### PR-001 — Repository bootstrap & "Foundry initialized."
- **Goal:** A buildable, CI-green Go module whose binary runs and reports itself alive.
- **Motivation:** Establish the working-state baseline every later PR must preserve. Nothing can be reviewed against a repo that doesn't compile.
- **Scope:** `go.mod`; `cmd/foundry/main.go` printing `Foundry initialized.`; `Makefile` (`build`, `test`, `lint`, `run`); `LICENSE` (decision dep, see Risks); `.gitignore`; one trivial passing test; CI workflow that builds + tests + lints on push/PR.
- **Out of scope:** Any command parsing, config, YAML, runtime. No flags.
- **Technical design:** Single `main()`. CI: GitHub Actions, `go build ./... && go test ./... && golangci-lint run`. Pin Go version in `go.mod` and CI.
- **Dependencies:** ADR-0001 (language) ratified.
- **Acceptance criteria:**
  - `make build && ./bin/foundry` prints exactly `Foundry initialized.` and exits 0.
  - `make test` passes; `make lint` is clean.
  - CI is green on the PR.
- **Testing strategy:** One unit test asserting the banner string (extract to a `func banner() string` so it's testable without capturing stdout).
- **Documentation changes:** `README.md` quickstart (build + run); `CONTRIBUTING.md` stub (build/test/lint commands).
- **Risks:** License choice is a governance decision (ties to RFC-0001 review P0-2) — do not let it block; pick a placeholder Apache-2.0 with a note that governance RFC may revisit.
- **Complexity:** **S**

---

#### PR-002 — CLI command framework (`version`, `help`, root)
- **Goal:** A real command tree so every future capability has a home and is demonstrable.
- **Motivation:** Vertical slices need a user-facing surface. Establishing the CLI framework now prevents each later PR from reinventing argument handling.
- **Scope:** Adopt `cobra`; root command (`foundry` with no args prints the init banner + usage hint); `foundry version` (embeds build version/commit via ldflags); global flags `--config`, `--verbose`, `--json`. Structured logging skeleton (text + `--json`).
- **Out of scope:** `run`, `config`, `pipeline` subcommands (later PRs add them).
- **Technical design:** `internal/surfaces/cli` holds the tree; `cmd/foundry` only calls `cli.Execute()`. Version injected at build time in the Makefile.
- **Dependencies:** PR-001.
- **Acceptance criteria:**
  - `foundry version` prints a semver + commit.
  - `foundry --help` lists commands; unknown command exits non-zero with a helpful message.
  - `--json` switches log output to JSON.
- **Testing strategy:** Command-level tests using cobra's command execution with captured output; table-driven for flag parsing.
- **Documentation changes:** README command list section (kept current every PR thereafter).
- **Risks:** Over-investing in CLI ergonomics early; keep it minimal.
- **Complexity:** **S**

---

#### PR-003 — YAML + JSON-schema validation foundation
- **Goal:** A reusable "parse YAML → validate against a schema → typed struct" primitive with precise, line-numbered errors.
- **Motivation:** Pipelines, skills, and config are all YAML. One rigorous, well-tested parse/validate layer prevents divergent, sloppy YAML handling across the codebase. Good errors here are a major DX lever.
- **Scope:** `internal/schema`: YAML decode (strict, unknown-field rejection), JSON-schema validation, error type carrying file + line/column + JSON-pointer path. A `foundry schema validate --kind <k> <file>` debug command wired to a trivial sample schema.
- **Out of scope:** The actual pipeline/skill schemas (defined where those types are introduced).
- **Technical design:** `sigs.k8s.io/yaml` (or `goccy/go-yaml` for position info) + a JSON-schema validator. Decode to `map`, validate, then strict-decode to typed struct. Errors implement a common `SchemaError` with location.
- **Dependencies:** PR-002.
- **Acceptance criteria:**
  - Valid documents parse; invalid ones produce errors naming the file, line, and offending field.
  - Unknown fields are rejected (strict mode), not silently dropped.
  - Golden-file tests cover ≥6 representative error shapes.
- **Testing strategy:** Golden/snapshot tests on error messages (high value for DX); table-driven valid/invalid pairs in `testdata/schema/`.
- **Documentation changes:** `docs/authoring/yaml-conventions.md` (strictness, error format).
- **Risks:** YAML library choice affects whether we get line numbers — evaluate before merging; line numbers are worth a heavier dependency.
- **Complexity:** **M**

---

#### PR-004 — Configuration loader (`foundry config show`)
- **Goal:** Layered configuration (defaults → `.foundry/config.yaml` → env → flags) with introspection.
- **Motivation:** Everything downstream (providers, budgets, paths) needs resolved config. Introspection (`config show`) makes config debuggable from day one.
- **Scope:** `internal/config`; precedence resolution; `.foundry/` discovery (walk up from cwd); `foundry config show` (resolved values + their source), `--json`. Redaction of secret-shaped keys in output.
- **Out of scope:** Secrets *storage* (M3 keychain); provider config (M3).
- **Technical design:** Consumes PR-003's schema primitive. Each setting records its origin layer for `show`. Env prefix `FOUNDRY_`.
- **Dependencies:** PR-003.
- **Acceptance criteria:**
  - `config show` reports each value and where it came from.
  - Env overrides file; flags override env (verified by test).
  - Secret-shaped values are redacted in output.
- **Testing strategy:** Table-driven precedence tests; fixture `.foundry/` trees; redaction test.
- **Documentation changes:** `docs/configuration.md`.
- **Risks:** Config sprawl — keep the v0.1 schema tiny; add keys only when a feature needs them.
- **Complexity:** **M**

---

#### PR-005 — Pipeline domain model + graph load (`foundry pipeline validate|show`)
- **Goal:** Parse a pipeline YAML into a typed, validated DAG of stages; detect cycles; render it.
- **Motivation:** The pipeline is the source of truth and the central domain object. The runtime (PR-006) needs a validated graph to execute.
- **Scope:** `internal/domain` pipeline/stage types; pipeline JSON-schema; DAG construction from stage dependencies; cycle + dangling-reference detection; `foundry pipeline validate <file>` and `foundry pipeline show <file>` (ASCII graph / `--json`).
- **Out of scope:** Execution (PR-006); skills (M2); gates/validators (M1 later).
- **Technical design:** Stages declare `id`, `op` (built-in) or placeholder for `skill` (rejected with "not yet supported" until M2), and `needs: [ids]`. Topological sort with cycle detection. Pure domain — no I/O beyond reading the file via the surface.
- **Dependencies:** PR-003.
- **Acceptance criteria:**
  - A valid pipeline validates; a cyclic one fails with the cycle path.
  - A stage referencing an unknown `needs` id fails with a clear message.
  - `pipeline show` renders stages in dependency order.
- **Testing strategy:** Unit tests on graph construction + cycle detection (property test: random DAGs validate, random graphs-with-cycles fail); golden tests on `show`.
- **Documentation changes:** `docs/authoring/pipelines.md` (schema, examples in `examples/`).
- **Risks:** Schema churn later (M2 adds skills/gates). Mitigate with explicit schema versioning field from the start (`version: 1`).
- **Complexity:** **M**

---

#### PR-006 — Deterministic runtime + built-in ops (`foundry run`) — **M0 exit**
- **Goal:** Execute an all-deterministic pipeline end-to-end and report per-stage results.
- **Motivation:** This is the walking skeleton's spine: proves the orchestrator, stage sequencing, and result reporting against real (if trivial) work. Everything after is "add an executor / add a gate."
- **Scope:** `internal/kernel/orchestrator`; sequential execution in topological order; built-in deterministic ops `echo`, `noop`, `fail` (for testing failure paths); per-stage status (pending/running/ok/failed); `foundry run <pipeline>` with a rendered summary + non-zero exit on failure.
- **Out of scope:** Ledger (PR-007), artifacts (PR-010), parallelism, executors, gates.
- **Technical design:** Orchestrator walks the DAG; each stage maps to a registered built-in op `func(ctx, inputs) (result, error)`. Synchronous, single-threaded (parallelism deferred — see Tech Debt). State held in memory.
- **Dependencies:** PR-005.
- **Acceptance criteria:**
  - `foundry run examples/hello.yaml` runs an `echo` stage and prints its output.
  - A pipeline with a `fail` stage exits non-zero and reports which stage failed.
  - Stages execute in dependency order (verified by an ordering test).
- **Testing strategy:** Integration test running real example pipelines from `testdata/`; unit tests per op; failure-path test.
- **Documentation changes:** README quickstart updated to `foundry run`; `examples/hello.yaml`.
- **Risks:** Temptation to add parallelism/executors here — resist; keep M0 a true skeleton.
- **Complexity:** **M**

> **M0 demo:** clone → `make build` → `foundry run examples/hello.yaml` → deterministic stages execute and report. Foundry is a (tiny) working runtime.

---

### M1 — Deterministic Runtime (fully specified)

---

#### PR-007 — Run ledger (event-sourced) + run inspection
- **Goal:** Every run emits an ordered, immutable event log persisted locally; runs are listable and inspectable.
- **Motivation:** The ledger is the backbone for replay, resume, audit, and observability (`ARCHITECTURE.md §7.4`). Building it now means those four features are projections, not rewrites.
- **Scope:** `internal/kernel/ledger`; append-only event store (embedded — SQLite or append-only file, ratify in ADR-0004); event types (RunStarted, StageStarted, StageFinished, RunFinished, …); `foundry runs` (list) and `foundry inspect <run>` (event timeline).
- **Out of scope:** Replay (PR-008), remote ledger (M7).
- **Technical design:** Orchestrator emits events; store appends with monotonic sequence + content hashes. Run state is a *projection* over events, not a separate mutable record.
- **Dependencies:** PR-006.
- **Acceptance criteria:**
  - A run produces a persisted event sequence; `runs` lists it; `inspect` shows the timeline.
  - Events are append-only (no update/delete API).
  - Killing mid-run leaves a valid partial event log.
- **Testing strategy:** Unit tests on event ordering/projection; integration test asserting a run's full event sequence; crash-simulation test (truncate + reopen).
- **Documentation changes:** `docs/concepts/run-ledger.md`.
- **Risks:** Event schema churn. Mitigate: version every event; never repurpose a field; additive evolution only (ties to Tech Debt: event migration).
- **Complexity:** **L**

---

#### PR-008 — Replay from ledger (`foundry replay <run>`)
- **Goal:** Re-execute a recorded deterministic run from its ledger and produce identical results.
- **Motivation:** This *is* the process-determinism guarantee (`RFC-0001 §6.5`) and the test harness for everything that follows. Once replay exists, regressions are detectable by diffing run outputs.
- **Scope:** Replay mode that reconstructs run state from events and re-runs deterministic stages; result equality check; `foundry replay <run>` reporting identical/divergent.
- **Out of scope:** Provider record/replay (M3 builds on this).
- **Technical design:** Deterministic ops re-run live; their outputs are compared to recorded artifacts/hashes. Divergence is a first-class, reported outcome.
- **Dependencies:** PR-007.
- **Acceptance criteria:**
  - Replaying a deterministic run reports identical output and exit status.
  - Tampering with the pipeline then replaying reports divergence at the right stage.
- **Testing strategy:** Property test — for arbitrary deterministic pipelines, run-then-replay is identical (this is a flagship invariant test).
- **Documentation changes:** `docs/concepts/determinism.md` (process vs output determinism, honest framing per RFC §6.5).
- **Risks:** "Deterministic" ops that aren't (clock, env, randomness). Mitigate: built-in ops are pure; shell executor (PR-011) is explicitly marked non-deterministic and excluded from the identity guarantee.
- **Complexity:** **L**

---

#### PR-009 — Resume & safe interruption (`foundry resume <run>`)
- **Goal:** Interrupted runs continue from the last completed stage instead of restarting.
- **Motivation:** Trust + UX: `Ctrl-C` must never corrupt state (`ARCHITECTURE.md §16.2`). Long AI runs (later) make this essential.
- **Scope:** Durable run state reconstruction from events; resume entrypoint; signal handling (graceful stop emits a checkpoint event).
- **Out of scope:** Mid-stage resumption (stages are the resume granularity in v0.1).
- **Technical design:** On start, if a run is incomplete, rebuild state from events and continue at the first non-finished stage. SIGINT → finish current stage or mark it interrupted, then exit cleanly.
- **Dependencies:** PR-007.
- **Acceptance criteria:**
  - Kill a run after stage 2 of 4; `resume` completes stages 3–4 without redoing 1–2.
  - SIGINT during a run leaves a resumable, valid ledger.
- **Testing strategy:** Integration test with simulated interruption; idempotency test (resume of a completed run is a no-op).
- **Documentation changes:** `docs/concepts/run-ledger.md` (resume section).
- **Risks:** Partial side effects from non-deterministic stages on resume — documented limitation until worktree isolation (M5).
- **Complexity:** **M**

---

#### PR-010 — Artifact model (content-addressed) + store
- **Goal:** Stage outputs become first-class, hashed, stored, referenceable artifacts.
- **Motivation:** Provenance and replay (`ARCHITECTURE.md §3.2`) require content-addressed artifacts. Validators (PR-012) operate on them.
- **Scope:** `domain.Artifact` (hash = identity); local content-addressed store under `.foundry/`; stages declare/produce artifacts; events reference artifacts by hash; `foundry artifacts <run>`.
- **Out of scope:** Remote artifact store; large-binary optimization.
- **Technical design:** Artifact = `{hash, kind, metadata, content-ref}`. Store is `hash → bytes`. Ledger events carry artifact hashes, not bodies.
- **Dependencies:** PR-007.
- **Acceptance criteria:**
  - A stage producing output yields an artifact retrievable by hash.
  - Identical content → identical hash → single stored copy (dedup verified).
  - `artifacts <run>` lists artifacts with hashes + producing stage.
- **Testing strategy:** Unit tests on hashing/dedup; integration test threading an artifact from producer to consumer stage.
- **Documentation changes:** `docs/concepts/artifacts.md`.
- **Risks:** Hash algorithm choice (stability across versions) — pin and document; treat as a compatibility surface.
- **Complexity:** **M**

---

#### PR-011 — Executor port + shell executor
- **Goal:** Formalize the Executor interface and ship the first real adapter: run a shell command as a stage, capturing output as an artifact.
- **Motivation:** Introduces the port abstraction *with a concrete adapter* (no speculative interface). Makes Foundry immediately useful as a deterministic-ish task runner (build/test orchestration) before any AI.
- **Scope:** `internal/ports` Executor interface; `internal/adapters/executor/shell`; stage `op: shell` with `cmd`, captured stdout/stderr/exit-code → artifacts; timeout via budget hook (placeholder until PR-014).
- **Out of scope:** Mock/AI executors (M2/M3); sandboxing (M6).
- **Technical design:** `Executor.Execute(ctx, ExecRequest) (ExecResult, error)`. Shell executor marked `deterministic: false` so replay (PR-008) excludes it from identity guarantees but still records its outputs.
- **Dependencies:** PR-006, PR-010.
- **Acceptance criteria:**
  - A `shell` stage runs `go test ./...` (or `echo`) and captures output as an artifact.
  - Non-zero exit fails the stage.
  - Replay of a shell stage records (does not falsely guarantee) its output.
- **Testing strategy:** Integration tests with deterministic commands; timeout test; non-zero-exit test.
- **Documentation changes:** `docs/authoring/executors.md` (shell); update pipeline schema docs.
- **Risks:** Shell nondeterminism leaking into determinism claims — enforced by the `deterministic` flag and tested.
- **Complexity:** **M**

---

#### PR-012 — Validator port + deterministic validators + ValidationReport
- **Goal:** Run checks over artifacts and produce structured findings, without mutating anything.
- **Motivation:** Verification is the core of trust (`RFC-0001 §6.1`); deterministic-first (`§8.3`). Gates (PR-013) consume reports.
- **Scope:** `ports.Validator`; `domain.ValidationReport`/`Finding{severity,location,rule_id,message,fixable}`; adapters: `exitcode` (wrap any command; non-zero = finding) and a structured `shell` validator; `foundry validate <pipeline>` to run validators standalone.
- **Out of scope:** LLM-judge validators (deferred; require providers, M3+); ecosystem-specific validators (community/M6).
- **Technical design:** `Validator.Validate(artifacts, profile) ValidationReport`. Validators are **pure w.r.t. the codebase** — read & report only (auto-fix is a future *skill*, not a validator side effect).
- **Dependencies:** PR-010.
- **Acceptance criteria:**
  - A failing build/test/lint produces findings with location + rule id.
  - Validators never modify artifacts or the working tree (enforced by test).
  - Reports are deterministic for identical inputs.
- **Testing strategy:** Table-driven on report construction; integration with a real linter on a fixture repo; purity test (no FS writes).
- **Documentation changes:** `docs/concepts/validation.md`.
- **Risks:** Validator output format churn — version the report schema.
- **Complexity:** **M**

---

#### PR-013 — Gate evaluation
- **Goal:** Turn validation reports into deterministic pass/fail/repair verdicts that drive the runtime.
- **Motivation:** Gates are where validation becomes decision (`ARCHITECTURE.md §12.3`); a gate verdict is reproducible — a cornerstone of process determinism.
- **Scope:** `domain.Gate` + `kernel/gate`; `all/any/none` over named conditions; `on_fail: fail | repair | warn`; stage-level and pipeline-level gates; runtime honors verdicts (fail stops with non-zero; warn continues).
- **Out of scope:** Repair execution (PR-015 wires `repair` to a loop).
- **Technical design:** Gate is a pure function `(reports, signals) → verdict`. Pipeline schema gains `gate:` on stages. `repair` verdict is recorded but, until PR-015, treated as `fail` with a clear "repair not yet enabled" note.
- **Dependencies:** PR-012.
- **Acceptance criteria:**
  - A pipeline with `gate: {all: [tests.pass]}` fails when tests fail, passes when they pass.
  - Gate verdicts are identical for identical reports (reproducibility test).
- **Testing strategy:** Table-driven gate logic; integration: build→test→gate pipeline.
- **Documentation changes:** `docs/concepts/gates.md`; pipeline schema docs.
- **Risks:** Gate expression complexity creep — keep to `all/any/none` in v0.1.
- **Complexity:** **M**

---

#### PR-014 — Budget accounting & enforcement
- **Goal:** Runs and stages carry enforceable ceilings (time, iterations; tokens/cost as placeholders for M3).
- **Motivation:** Budget is a first-class constraint, not a report (`ARCHITECTURE.md §4`). Repair loops (PR-015) and AI executors (M3) are unsafe without it.
- **Scope:** `kernel/budget`; budget declared in pipeline (`max_iterations`, `max_duration`); enforcement aborts over-budget runs with a clear reason; token/cost fields defined but inert until providers exist.
- **Out of scope:** Cost estimation (lives in provider adapters, M3).
- **Technical design:** Budget tracked in the orchestrator; checked before each stage and each repair iteration; over-budget emits a ledger event and stops.
- **Dependencies:** PR-006 (enforcement point), PR-007 (events).
- **Acceptance criteria:**
  - A pipeline exceeding `max_duration` aborts with a budget-exceeded event.
  - Budget state is visible in `inspect`.
- **Testing strategy:** Unit tests on accounting; integration test tripping each ceiling.
- **Documentation changes:** `docs/concepts/budgets.md`.
- **Risks:** None significant; keep token/cost inert until real.
- **Complexity:** **S/M**

---

#### PR-015 — Bounded repair loops (with deterministic fixer demo) — **M1 exit**
- **Goal:** A failed gate can feed findings back to a fixer stage and re-verify, bounded by iterations + budget + must-make-progress.
- **Motivation:** Repair is the workhorse of quality (`ARCHITECTURE.md §7.5`). Proving it *deterministically* (e.g., a formatter fixing lint findings) validates the mechanism before AI fixers exist.
- **Scope:** Repair-loop construct in the orchestrator: `gate on_fail: repair` re-invokes a designated fixer stage with the findings as input; bounded by `max_iterations` and budget; aborts unless findings strictly decrease.
- **Out of scope:** AI-based repair (M3 plugs in naturally).
- **Technical design:** Loop = {run fixer → re-validate → re-gate}. Progress invariant: finding count must strictly decrease per iteration or abort. All iterations recorded in the ledger.
- **Dependencies:** PR-013, PR-014, PR-011 (a deterministic fixer, e.g., `gofmt`/`eslint --fix` via shell).
- **Acceptance criteria:**
  - A pipeline with a lint gate + formatter fixer converges: lint fails → fixer runs → lint passes, within bound.
  - A non-converging case aborts at `max_iterations` with a clear, recorded reason.
  - Repair never loops unbounded (enforced by test).
- **Testing strategy:** Integration with a real formatter on a deliberately-misformatted fixture; non-convergence test; ledger assertion that every iteration is recorded.
- **Documentation changes:** `docs/concepts/repair-loops.md`.
- **Risks:** Infinite/expensive loops — directly mitigated by the dual bound + progress invariant.
- **Complexity:** **L**

> **M1 demo:** a deterministic pipeline that builds, tests, lints, **gates** on the results, **repairs** lint failures with a formatter, records everything to a **ledger**, can be **replayed identically** and **resumed** after interruption — with **zero providers configured**. Foundry is now a genuinely useful, rigorous, AI-free runtime.

---

### M2 — Skills + Mock Executor (medium detail)

> Goal of milestone: author declarative skills and run AI-*shaped* pipelines fully deterministically against a fixture/mock executor — so the entire skill/repair/context machinery is proven before a single real token is spent.

| PR | Title | Goal | Key acceptance | Cx |
|---|---|---|---|---|
| PR-016 | Skill manifest + contract | Declarative skill (`inputs`, `context`, `executor req`, `outputs`, `validators`, `gates`, `repair`); `foundry skill validate/list` | Manifest-only skill validates; missing required field errors with location | M |
| PR-017 | Skill invocation in stages | Stage `skill: <name>` resolves + runs via a generic skill executor | A pipeline composed of skills runs end-to-end | M |
| PR-018 | Context port + static resolver | `ports.Context`; deterministic file-glob/explicit resolver → immutable, hashed `ContextBundle` with provenance | Bundle is content-addressed; provenance lists every chunk + source | M |
| PR-019 | Mock executor (fixture/record-replay) | Executor returning scripted outputs keyed by request hash | Same request hash → same output; unknown request fails loudly (no silent fabrication) | M |
| PR-020 | Skill/pipeline packs + version pinning | Bundle + distribute skills/pipelines locally; pin versions for replay | A pipeline pins skill versions; replay uses pinned versions | M |

Milestone exit: `foundry run implement-feature.yaml` runs a realistic AI-shaped pipeline (context → skill(mock) → validate → gate → repair) **deterministically**, proving the full shape with no provider.

---

### M3 — Provider SDK & Real Executors (epic-level)

> The first milestone where real AI appears — and only *after* the runtime is complete. Built on the mock executor's contract so the swap is a drop-in.

- **PR-021** Provider port + `CapabilityDescriptor` (context window, supported features, cost, limits, locality). _M_
- **PR-022** Secrets port + keychain/env adapters; secret redaction in logs/ledger. _M_
- **PR-023** Router: capability match, routing policy (cost/latency/quality/privacy), **budget gate** refusing over-budget calls. _L_
- **PR-024** Failover chains + circuit breakers + health. _M_
- **PR-025** Anthropic adapter (normalized ↔ native; capabilities incl. caching/tools/reasoning; cost descriptor). _L_
- **PR-026** OpenAI adapter. _M_
- **PR-027** Ollama adapter (local; enables privacy-constrained/`must_be_local` routing). _M_
- **PR-028** Provider record/replay (cassettes): real runs recorded; replay re-checks gates without spending tokens. _L_
- **PR-029** Graceful capability degradation (emulate structured output via tool-calling, etc.). _M_

Milestone exit: a real `implement-feature` run against a live provider, recorded and replayable; switching providers via policy requires no pipeline change.

---

### M4 — Knowledge & Context Engine (epic-level)

> Foundry's differentiator. Deterministic-first throughout; semantic retrieval is a *backstop*, explicitly deferred within the milestone.

- **PR-030** Knowledge graph schema + store; **Derived/Authored split** enforced at the type level. _L_
- **PR-031** Extractors: AST (structural), git (history/ownership), markdown (ADRs/docs). _L_
- **PR-032** Context engine v1: deterministic-first retrieval (structural → lexical → decision → historical), ranking, budget knapsack, compaction tiers, mandatory provenance. _XL_
- **PR-033** Content-addressed bundle caching + **diff-driven incremental invalidation**. _L_
- **PR-034** Gated, reviewable **knowledge-update** workflow (propose ADR / state-of-impl as a diff; human approves). _L_
- **PR-035** Relevance feedback loop (which loaded context was actually used) — recorded, observable. _L_
- **PR-036** Semantic retrieval tier (embeddings as recall backstop) — *intentionally last*. _L_

Milestone exit: context-aware runs whose every context chunk is attributable; knowledge updates land as reviewable diffs, never silent rewrites (RFC §V2).

---

### M5 — Integration & Visibility (epic-level)

- **PR-037** Git worktree isolation — all mutation off the user's working tree (`ARCHITECTURE.md §7.3`). _L_
- **PR-038** Diff review + approve flow (`foundry review last`, `foundry approve last`). _M_
- **PR-039** VCS port + GitHub adapter. _M_
- **PR-040** Ledger-grounded commit + PR generation (message/body are projections of the run, not post-hoc summaries). _L_
- **PR-041** Saga/rollback: compensating actions; irreversible actions escalate, never auto-undo. _L_
- **PR-042** CI mode (same binary) + native annotations (GitHub Checks) + replay-based verification. _L_
- **PR-043** Observability: OTel spans (Run→Stage→Skill→Provider/Validator), cost/token metrics. _M_
- **PR-044** Local dashboard/TUI (bubbletea): runs, costs, knowledge health — no cloud. _L_

Milestone exit: local-authored workflow runs unchanged in CI, produces a grounded PR, fully observable.

---

### M6 — Extensibility (epic-level)

- **PR-045** Public SDK (`pkg/sdk`) + gRPC plugin protocol (`api/proto`). _L_
- **PR-046** Out-of-process plugin runtime (trusted, subprocess/gRPC). _L_
- **PR-047** Sandbox + **capability permission model** (default-deny; fs/network/secrets grants). _XL_
- **PR-048** Versioned ports + compatibility negotiation at load. _M_
- **PR-049** Minimal registry/discovery (git+manifest). _M_
- **PR-050** Per-port **conformance test suites** (third-party adapters self-certify). _L_

Milestone exit: a community-authored provider/validator/extractor installs and runs sandboxed, against a stable, versioned contract.

### M7 — Stability & Hardening (v1.0) (epic-level)

- Port contracts frozen under semver; deprecation policy.
- Security: air-gapped mode, PII/secret egress scanning, signing/trust policy.
- Enterprise: remote ledger backend, RBAC/SSO reference, audit export.
- Governance, contributor docs, documented release process, conformance certification.

---

## 5. Dependency Graph

```
PR-001 bootstrap
   └─ PR-002 CLI framework
        └─ PR-003 YAML+schema ──────────────┐
             ├─ PR-004 config                │
             └─ PR-005 pipeline model ◀───────┘
                  └─ PR-006 runtime (M0 exit)
                       ├─ PR-007 ledger
                       │    ├─ PR-008 replay
                       │    ├─ PR-009 resume
                       │    └─ PR-010 artifacts
                       │         ├─ PR-011 executor port + shell
                       │         └─ PR-012 validators
                       │              └─ PR-013 gates
                       └─ PR-014 budget
                            └─ PR-015 repair loops (M1 exit)
                                 │
   ┌─────────────────────────────┘
M2: PR-016 skill manifest → PR-017 invocation → PR-018 context(static)
                                              → PR-019 mock executor → PR-020 packs
M3: PR-021 provider port → PR-022 secrets, PR-023 router → PR-024 failover
        → PR-025/026/027 adapters → PR-028 cassettes → PR-029 degradation
        (PR-019 mock executor is the drop-in seam for real executors)
M4: PR-030 graph → PR-031 extractors → PR-032 context engine → PR-033 cache
        → PR-034 knowledge-update → PR-035 feedback → PR-036 semantic
        (PR-018 context port is the seam the engine slots behind)
M5: PR-037 worktree → PR-038 review → PR-039 vcs → PR-040 PR-gen → PR-041 saga
        → PR-042 CI → PR-043 otel → PR-044 TUI
M6: PR-045 SDK → PR-046 runtime → PR-047 sandbox → PR-048 versioning
        → PR-049 registry → PR-050 conformance
```

**Critical path to first usable runtime:** 001 → 002 → 003 → 005 → 006 (M0), then → 007 → 010 → 011/012 → 013 → 015 (M1). Config (004), replay (008), resume (009), budget (014) are parallelizable off the critical path.

**Key architectural seams (built once, leveraged forever):** the **Executor port** (PR-011/019) is where shell → mock → real providers all plug in; the **Context port** (PR-018) is where static → full engine slots in; the **ledger** (PR-007) is what makes replay/resume/audit/observability projections rather than features.

---

## 6. Testing Strategy

Layered, deterministic-first, and CI-enforced.

- **Unit (every PR).** Pure domain logic, table-driven. `internal/domain` and `kernel/*` aim for high coverage because they're pure and cheap to test.
- **Golden/snapshot.** YAML parse errors, CLI rendering, `pipeline show`, PR-body generation. High DX leverage; regenerate intentionally.
- **Integration (from PR-006).** Run real example pipelines from `testdata/`; assert ledger event sequences and artifacts. The example pipelines double as documentation.
- **Property/invariant tests.** Flagship: *run-then-replay is identical for deterministic pipelines* (PR-008); *repair loops always terminate* (PR-015); *gate verdicts are pure functions of reports* (PR-013).
- **Contract/conformance tests (ports).** Each port ships a reusable conformance suite any adapter must pass. Introduced with the port's second adapter; formalized as a public suite in M6 (PR-050) so third parties self-certify.
- **Record/replay cassettes (providers, M3+).** No live provider calls in CI; cassettes give deterministic, free, fast AI tests. A nightly (opt-in, gated) job exercises real providers to catch drift.
- **E2E CLI tests.** `testscript`-style (golden command transcripts) for the user-facing surface.
- **No network in PR CI.** Anything needing a network is mocked, cassetted, or moved to a nightly gated workflow.

**Coverage policy:** ratchet, not absolute — coverage may not *decrease* on a PR touching tested packages; pure domain packages held to a high bar. Don't chase coverage on glue/adapters where conformance + integration tests are the real signal.

---

## 7. CI Strategy

- **Platform:** GitHub Actions (public repo).
- **Per-PR (required, must be green for merge):** `go build ./...`, `go vet`, `golangci-lint`, `go test -race ./...`, coverage ratchet, build the single binary on the matrix (linux/macos/windows × amd64/arm64 — at least linux+macos from day one).
- **Branch protection:** PRs only; required green CI; ≥1 review; linear history. `main` is always releasable.
- **Conventional commits + semantic PR titles** (enables changelog generation — dogfooded later by the `generate-changelog` skill).
- **Conformance job (from M3):** runs each port's conformance suite.
- **Nightly (gated, not on PRs):** real-provider cassette refresh + drift detection; long/integration suites.
- **Release (from ~M1):** `goreleaser`-style cross-compiled, **signed** binaries + checksums; SBOM. Pre-1.0 releases may break port contracts (documented); post-1.0 frozen under semver.
- **Dogfooding milestone:** once M5 lands, Foundry runs Foundry's own `review-changes` / `generate-pr` in its CI — the strongest possible integration test and credibility signal.

---

## 8. Technical Debt Intentionally Postponed

Named deliberately, with the trigger that ends the deferral:

| Postponed | Until | Why it's safe to defer |
|---|---|---|
| **Stage parallelism** (runtime is sequential in M0–M1) | Performance becomes a real complaint (likely M4 on big repos) | DAG is already modeled; parallel execution is an orchestrator-internal change behind a stable interface |
| **Semantic/vector retrieval** | M4 PR-036 (last in the milestone) | Deterministic-first retrieval covers most cases, is cheaper and explainable; embeddings are a recall backstop |
| **Remote/distributed ledger** | M7 | Local embedded ledger is correct for local-first v0.1–v0.5; remote is an additive port, not a redesign |
| **Real sandboxing (WASM)** | M6; subprocess/gRPC isolation first | Out-of-process already contains crashes & enables language-agnostic plugins; WASM is a hardening step (ADR-0002) |
| **Event-log schema migrations** | When the first breaking event-shape change is unavoidable | Events are versioned + additive-only from PR-007, buying time |
| **TUI richness** | M5 PR-044 | CLI is the primary surface; TUI is a thin client over the same engine |
| **Multi-repo / cross-repo orchestration** | Post-1.0 | One Project at a time is the right v1 scope |
| **Marketplace / ratings / OCI distribution** | Post-1.0 | git+manifest registry suffices until there are packages |
| **Windows edge cases** (paths, signals) | Tracked per-PR; hardened in M7 | macOS/Linux cover the early audience; don't let Windows block velocity, but keep CI building it |

The rule: **debt is a logged decision with a trigger, never an accident.** Each item above should have a tracking issue referencing this table.

---

## 9. Risks During Implementation

1. **Context Engine (M4) is the make-or-break and is genuinely unsolved.** *Mitigation:* deterministic-first, mandatory provenance (diagnosable), feedback loop (improving), downstream gates (containment). Treat M4 as research-flavored; timebox PR-032 and ship a deliberately simple v1.
2. **Ledger schema churn breaks replay/audit.** *Mitigation:* version events from PR-007; additive-only; never repurpose fields; a replay-compat test in CI.
3. **Premature port abstraction (architecture astronautics).** *Mitigation:* hard rule — introduce a port only with its first adapter, generalize only with its second. The kernel stays thin.
4. **"Deterministic" stages that aren't.** *Mitigation:* built-in ops are pure; shell/AI executors are flagged non-deterministic and excluded from the identity guarantee; the run-then-replay property test guards regressions.
5. **Mock-vs-real provider drift (M2→M3).** *Mitigation:* the mock implements the *same Executor contract*; nightly real-provider cassettes detect divergence.
6. **Repair-loop cost/time runaway.** *Mitigation:* dual bound (iterations + budget) + must-make-progress invariant, enforced and tested in PR-015.
7. **Plugin security near credentials/repos (M6).** *Mitigation:* out-of-process + capability default-deny + signing; conformance + sandbox tests.
8. **Governance/licensing unresolved (RFC-0001 review P0-2).** *Mitigation:* don't let it block PR-001 (placeholder Apache-2.0), but resolve before opening community contributions (M6) — flagged as a dependency, not a code task.
9. **YAML schema evolution breaking users' pipelines.** *Mitigation:* `version:` field from PR-005; schema changes are additive or major-versioned with a migration note.
10. **Scope creep inside milestones.** *Mitigation:* the per-PR "Out of Scope" sections are contractual; anything not listed in-scope is a follow-up PR.

---

## 10. Recommended First PR (start coding tomorrow)

**PR-001 — Repository bootstrap & "Foundry initialized."**

Start here, exactly as specified in §4. It is **S** complexity, has one external dependency (ratify ADR-0001 language choice — Go), and establishes the working-state baseline that every subsequent PR must preserve.

**Concrete definition of done for tomorrow:**
- `go.mod` initialized; `cmd/foundry/main.go` prints `Foundry initialized.`
- `Makefile` with `build`, `test`, `lint`, `run`.
- One unit test (assert the banner via an extracted `banner()` function).
- GitHub Actions CI: build + test + lint, green on the PR.
- `README.md` (build/run) + `CONTRIBUTING.md` stub + `LICENSE` (Apache-2.0 placeholder, noted as governance-pending).
- Branch protection on `main` configured.

**Two things to settle *before* the PR merges (not code, but blocking):**
1. **ADR-0001 (language).** This roadmap assumes Go; ratify or revise.
2. **License placeholder decision.** Apache-2.0 is the safe default; the governance RFC (RFC-0001 review P0-2) may revisit, which is fine — but `main` needs a license file before it's public.

After PR-001 is green, PR-002 (CLI framework) unblocks the entire vertical-slice cadence, and the critical path 003 → 005 → 006 reaches the first usable runtime (M0) quickly.

---

_End of roadmap. This is a living plan: re-detail each milestone's PRs at that milestone's start, keep the "Out of Scope" sections honest, and let `main` always run. Build the engine first; let AI be the executor it was always meant to be — one of several._
