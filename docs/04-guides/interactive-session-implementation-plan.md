# Interactive Session — Component Design & Implementation Plan

> **Executable roadmap** for the product-shape decision proposed in [RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md): `foundry` becomes an interactive, slash-command-driven session instead of a flag-parsed one-shot CLI. This guide is the "how, in order"; RFC-0003 is the "why". Each PR below leaves the repository compiling with tests green, mirroring [M0-IMPLEMENTATION-BACKLOG.md](../archive/obsolete/M0-IMPLEMENTATION-BACKLOG.md)'s discipline (M0 is now complete; that document is historical).
>
> **Scope guard, confirmed by audit (§1):** `engine.Engine`, `engine.PipelineStrategy`, `engine.PipelineRegistry`, and `engine.PipelineProvider` require **zero modifications**. Every component below is either a new adapter satisfying an existing port, or new code in new packages one layer above `engine`. Where this guide would have needed an Engine change, it says so explicitly (it doesn't happen).

---

## 1. Audit — what exists today, what's reused, what should eventually disappear

Confirmed by running the built binary (not just reading source):

```
$ foundry
Usage: foundry <command> [arguments]
Commands:
  do    Run the Act lifecycle for an Intent against a repository
  log   List recorded Acts for a repository
  show  Show one recorded Act in full
```

| Piece | Today | Verdict |
|---|---|---|
| `cmd/foundry/main.go` — literal `switch` on `args[0]` | One-shot dispatch, exits after one action | **Add a new branch** (no args → start a Session); existing branches stay until slash-command parity is proven (§5) |
| `cli.ParseArgs`, `parseLogArgs`, `parseShowArgs` — three near-duplicate `--repo`/positional parsers | Each subcommand reimplements its own flag scanner | **Eventually disappears.** In a session, the project root is the current directory (matching Claude Code / aider / opencode) — `--repo` never needs to exist on the interactive surface. This is the concrete answer to "no quiero un CLI con docenas de flags." |
| `cli.CLI.Do/Log/Show` | Engine.Run → approval → apply → record; read history | **Reused unchanged.** This is the single biggest reuse point: a slash-command handler is a thin adapter that ends by calling straight into `cli.CLI`'s existing methods. |
| `cli.PromptForApproval` | Blocking `y/n` read from `io.Reader` | **Reused unchanged.** It only assumes an `io.Reader`/`io.Writer`; a REPL reading the *next* line for approval is the same call, not a new mechanism. |
| `cli.ProgressReporter` (satisfies `engine.Reporter`) | Narrates Gathering/Executing/Verifying/Repairing to an `io.Writer` | **Reused unchanged** as the Engine-facing progress renderer (§3, "ProgressRenderer"). |
| `engine.NewDefaultRegistry`, `PipelineRegistry.RegisterMany` | Composes `BuiltinProvider`'s output once | **Reused unchanged**, generalized by `ProjectLoader` (§3) to also compose a `FilesystemPipelineProvider` — the exact registration mechanism already twice validated. |
| `record.FileStore`, `workspace.Workspace`/`StagedVerifier`, `gatherer.NaiveGatherer`, `verify.Gate`, `executor/claude.ClaudeExecutor` | Engine ports/adapters | **Reused unchanged.** None of these know or care whether they're wired once per process or once per session. |
| `engine.Engine` bound to one `Pipeline` at construction | `NewEngine(gatherer, executor, verifier, workspace, pipeline)` | **Not a limitation.** `NewEngine` is cheap (pointer wiring, no I/O). A session running `/feature`, then `/bug`, then `/review` constructs a fresh `*Engine` per command, selecting the Pipeline by name each time — exactly what `cmd/foundry/commands/do.go` already does once per process, just called more than once within one process. **No Engine change proposed or needed.** |

**Conclusion of the audit's mandate ("no debe modificarse salvo que encuentres una limitación arquitectónica real"): no real limitation was found.** `Engine`, `PipelineStrategy`, `PipelineRegistry`, and `PipelineProvider` are all used exactly as already designed.

---

## 2. Design decisions this plan is built on

1. **Project root = current working directory.** No `--repo` flag on the interactive surface, ever. `Session` captures `os.Getwd()` once at startup.
2. **Slash-command name = Pipeline name, by default.** `/feature "…"` resolves the Pipeline named `"feature"`; `/bug "…"` resolves `"bugfix"`; `/review "…"` resolves the built-in `"review"` Pipeline (already shipped); `/release "…"` resolves `"release"`. An unresolved name is a **clear, named error pointing at `/init`** — never a silent fallback to `"default"`. This follows the codebase's own established value ("fix(engine): Fail Pipeline execution loudly instead of silently or by panic").
3. **One generic command handler, not four bespoke types.** `/feature`, `/bug`, `/review`, `/release` are the *same* handler (`RunPipelineCommand{PipelineName}`) instantiated four times — the "capa superior que traduce slash commands hacia Pipelines" the request asks for, made literal: the handler contains no per-command logic at all.
4. **`/init` is the one genuinely different handler.** It scaffolds `.foundry/pipelines/` instead of running a Pipeline.
5. **`ProgressRenderer` is not a new type.** It is `cli.ProgressReporter`, reused as-is. A visually richer renderer later is a *new* implementation of the same unchanged `engine.Reporter` port — never an Engine change.

---

## 3. Component design

| Component | Package (new) | Responsibility | Interface (indicative) | Depends on | Reuses from today | Fully decoupled from |
|---|---|---|---|---|---|---|
| **FilesystemPipelineProvider** | `project` | Reads `.foundry/pipelines/*.json` in the project root, decodes each with the unchanged `engine.DecodePipelineDocument` | `Load(ctx) ([]engine.Pipeline, error)` — satisfies `engine.PipelineProvider` | `engine.DecodePipelineDocument`, `engine.Pipeline` | `DecodePipelineDocument`'s existing validation, unchanged | Slash commands, rendering, `BuiltinProvider` |
| **ProjectLoader** | `project` | Composes `BuiltinProvider` + `FilesystemPipelineProvider` into one `*engine.PipelineRegistry` at session start; scaffolds `.foundry/pipelines/` for `/init` | `LoadRegistry(root string) (*engine.PipelineRegistry, error)`; `Scaffold(root string) error` | `engine.NewPipelineRegistry`, `RegisterMany`, `BuiltinProvider`, `FilesystemPipelineProvider` | `PipelineRegistry`'s registration mechanism, unchanged | Session lifecycle, rendering, slash-command syntax |
| **SlashCommandParser** | `session` | Parses one input line into a `Command{Name, Args}` or "plain text" | `Parse(line string) (Command, isSlash bool)` | nothing (pure function) | — (new, trivial) | Everything — no I/O, no Engine knowledge |
| **CommandRegistry** | `session` | Maps a command name to a `CommandHandler`; refuses duplicate registration; errors on unknown name | `Register(name string, h CommandHandler) error`; `Dispatch(ctx, name, args string) error` | `CommandHandler` | The *pattern* of `PipelineRegistry` (register-once, lookup-by-name) — not its code | Individual command implementations |
| **CommandHandler** (interface) | `session` | The contract every slash command satisfies | `Run(ctx context.Context, s *Session, args string) error` | `Session` | — | Rendering specifics (delegated) |
| **RunPipelineCommand** | `session` | The one handler backing `/feature`, `/bug`, `/review`, `/release`: resolves `PipelineName` from `s.Registry`, builds an `Intent` from `args`, constructs a fresh `engine.Engine` + `cli.CLI`, calls `cli.CLI.Do` | implements `CommandHandler`; `type RunPipelineCommand struct{ PipelineName string }` | `Session`, `engine.NewEngine`, `cli.NewCLI`, `cli.CLI.Do` | `cli.CLI.Do`'s entire approve/apply/record body, unchanged | Which Pipeline it runs (data, not code) |
| **InitCommand** | `session` | Backs `/init`: calls `ProjectLoader.Scaffold` | implements `CommandHandler` | `project.ProjectLoader` | — | Pipeline execution entirely |
| **Session** | `session` | Owns session-lifetime state: project root, the composed `PipelineRegistry`, and the reusable Gatherer/Verifier/Executor/Recorder factories (relocated from `cmd/foundry/commands/do.go`, called once instead of once-per-process) | Plain struct + constructor; a method to build a fresh `*engine.Engine` for a resolved Pipeline | `project.ProjectLoader`, `gatherer`, `verify`, `workspace`, `executor/claude`, `record` | Every existing factory function (`gatherer.NewNaiveGatherer`, `verify.NewGate`, `workspace.NewStagedVerifier`, `claude.NewClaudeExecutor`, `record.NewFileStore`) | Slash-command syntax, terminal rendering |
| **REPL** | `session` | The read loop: reads a line from `io.Reader`, renders a prompt, parses, dispatches, repeats until `/exit` or EOF | `Run(ctx context.Context) error` | `SlashCommandParser`, `CommandRegistry`, `InteractiveRenderer` | Nothing engine-side — pure I/O loop, tested exactly like `cli_test.go`'s existing golden tests (`strings.Reader` in, `bytes.Buffer` out) | `engine`, `cli.CLI`'s internals |
| **InteractiveRenderer** | `cli` (new file, beside `progress.go`/`render.go`) | REPL-level chrome: prompt, banner, session info/error messages — *not* inside-an-Act narration | `Prompt()`, `Info(msg string)`, `Error(err error)`, `Banner()` | `io.Writer` | `cli`'s existing unexported ANSI/color helpers directly (same package) | `engine.Reporter`, Pipeline execution |
| **ProgressRenderer** | `cli` (existing) | Narrates one Act's Engine-driven lifecycle | *(no new type — see §2.5)* | — | `cli.ProgressReporter`, unchanged | Session/REPL entirely |

---

## 4. Sequencing rationale

Bottom-up: pure/leaf components first (no I/O, no Engine wiring), then composition (`ProjectLoader`), then the session-facing layer (`Session`, handlers), then the read loop (`REPL`), and only last, the one-line change to `main.go` that makes any of it reachable. Each step is independently testable without the steps after it existing — the same discipline RFC-0002's own phased migration used.

---

## 5. Commit plan

Each commit compiles and passes `go vet`, `go test ./...`, `go test -race ./...` on its own — no commit depends on a later one to build.

### Commit 1 — `feat(project): Add FilesystemPipelineProvider`
New package `project`. Reads every `*.json` in `<root>/.foundry/pipelines/` (flat directory, no recursion — same constraint the built-in provider work was scoped under), decodes each via the unchanged `engine.DecodePipelineDocument`. Tests: missing directory → empty slice, not an error; one valid document decodes; a malformed document surfaces `DecodePipelineDocument`'s existing named error; multiple documents load in a deterministic (filename-sorted) order.

### Commit 2 — `feat(project): Add ProjectLoader.LoadRegistry`
Composes `engine.BuiltinProvider{}` and `project.FilesystemPipelineProvider{Dir: ...}` into one `*engine.PipelineRegistry` via `NewPipelineRegistry`/`RegisterMany` — the exact mechanism `engine.NewDefaultRegistry` already uses, generalized by one extra provider. Tests: only built-ins when no project directory exists; built-ins + project-local Pipelines when it does; a project-local Pipeline whose name collides with a built-in surfaces `PipelineRegistry.Register`'s existing duplicate-name error, not a silent overwrite.

### Commit 3 — `feat(project): Add ProjectLoader.Scaffold for /init`
Writes `.foundry/pipelines/` with starter document(s) a user is expected to edit. Idempotent and safe to re-run: never overwrites a file that already exists (mirrors `git init`'s re-run safety). Tests: fresh project gets scaffolded; re-running after a user has edited a scaffolded file leaves that edit untouched.

### Commit 4 — `feat(session): Add SlashCommandParser`
Pure parsing, package `session`. `"/feature do X"` → `{Name: "feature", Args: "do X"}`; a line not starting with `/` is plain text; blank lines and whitespace are handled explicitly. Zero dependencies on `engine` or `cli`.

### Commit 5 — `feat(session): Add CommandRegistry`
Register-by-name, dispatch-by-name; duplicate registration and unknown-name lookup both fail with named errors — mirroring (not importing) `PipelineRegistry`'s already-tested shape. Tests use fake handlers; no real command logic yet.

### Commit 6 — `feat(session): Add Session`
The composition-root object: captures the project root, calls `ProjectLoader.LoadRegistry` once, and holds the reusable Gatherer/Verifier/Executor/Recorder factories relocated from `cmd/foundry/commands/do.go`. Exposes a method to build a fresh `*engine.Engine` for a named, resolved Pipeline. Tests prove two different named Pipelines (e.g. `"default"` and `"review"`) run back-to-back through the same `Session` without cross-contamination — directly extending the coexistence tests already proven for `engine.NewDefaultRegistry` in `engine/review_pipeline_test.go`, one layer up.

### Commit 7 — `feat(session): Add RunPipelineCommand`
Implements `CommandHandler`: resolves `PipelineName` via `Session`, builds an `Intent` from the command's args, constructs a fresh `cli.CLI` over a fresh `Session`-built `*engine.Engine`, and delegates to the unchanged `cli.CLI.Do`. This is the commit that proves the core claim: the same handler, instantiated with `PipelineName: "feature"` / `"bugfix"` / `"review"` / `"release"`, backs all four slash commands with no per-command branching. Tests reuse the scripted-executor fixtures `cli_test.go` already established.

### Commit 8 — `feat(session): Add InitCommand`
Thin wrapper over Commit 3's `ProjectLoader.Scaffold`. Trivial once Commit 3 exists.

### Commit 9 — `feat(cli): Add InteractiveRenderer`
New file beside `progress.go`/`render.go` in package `cli`, reusing the existing unexported ANSI/color helpers directly. Prompt, banner, info, and error rendering only — no Engine-facing narration (that stays `ProgressReporter`'s job, untouched).

### Commit 10 — `feat(session): Add REPL`
Wires `Session` + `SlashCommandParser` + `CommandRegistry` (with Commits 7–8's handlers registered) + `InteractiveRenderer` into the actual read loop over `io.Reader`, exiting on `/exit`/`/quit` or EOF. First end-to-end test of the full vertical slice: feed a scripted multi-line `strings.Reader` (`"/init\n/feature add x\ny\n/exit\n"`), assert on the output buffer *and* on side effects (a Pipeline document was scaffolded, an Act was recorded) — the same golden-test discipline `cli/repair_flow_test.go` already established, one layer up.

### Commit 11 — `feat(cmd/foundry): Start an interactive Session when invoked with no subcommand`
The only commit touching `main.go`'s top-level dispatch: `len(args) == 0` now builds a `Session` rooted at `os.Getwd()` and runs `REPL.Run` instead of printing usage and exiting 2. Deliberately last — every layer beneath it is already independently proven.

---

## 6. Explicitly out of scope for this plan

- **Removing `do`/`log`/`show` and their flag parsers.** They stay until slash-command parity (Commits 7–8 cover `/feature`-family and `/init`; `/history` and `/show <id>` wrapping `cli.CLI.Log`/`Show` are the same pattern as Commit 7 and are one more small commit, not included above to keep this plan's scope to what was asked) is proven in real use. Removal is a separate, later decision requiring its own explicit approval — never a big-bang swap.
- **The Router/Capability model** (RFC-0002 Phases 6–7) — `/feature`, `/bug`, etc. resolve a *Pipeline*, not yet a per-Step Executor. Naming Opus/Sonnet/Haiku/Copilot per Step is still gated behind those phases, unaffected by this plan.
- **Commit + PR automation** (RFC-0003 §4.1) — its own unresolved trust-boundary question; nothing here touches `Apply`.
