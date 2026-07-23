# ADR-0012 — Interactive Terminal UX and Foundry's First Third-Party Dependency

| | |
|---|---|
| **Status** | **Proposed** — drafted 2026-07-22, awaiting the maintainer's ratification under [ADR-0000](ADR-0000-governance-and-ratification-process.md). Unlike ADR-0007/ADR-0011/ADR-0008, this one is deliberately **not** self-ratified: it introduces Foundry's first external Go dependency, a real, binding precedent, and [ADR-0000](ADR-0000-governance-and-ratification-process.md)'s lightweight process still requires the maintainer's own act of ratifying, not an AI agent's. |
| **Date** | Drafted 2026-07-22 |
| **Deciders** | The project's sole maintainer, under [ADR-0000](ADR-0000-governance-and-ratification-process.md); drafted AI-assisted |
| **Ratifies** | A new roadmap item, not an existing backlog row: a richer interactive-session terminal experience (autocomplete for slash commands, live suggestions, styled output) comparable to Claude Code / Codex / OpenCode's own CLIs — and, as a direct consequence, whether Foundry may depend on external Go modules at all. |
| **Gates** | [roadmap.md](../00-overview/roadmap.md)'s new "Interactive terminal UX" parallel track (see Migration Strategy); [ADR-0001](ADR-0001-language-and-toolchain.md)'s R1 (single static binary) and its own flagged, not-yet-decided "Dependency & supply-chain policy" item (Future ADR Dependencies, point 3). |

---

## Context

**Today's interactive session has zero third-party dependencies and a line-based, non-interactive read loop.** `go.mod` declares no `require` block at all — every package Foundry imports today is the Go standard library. `session/repl.go`'s `REPL.Run` reads one line at a time via a plain `*bufio.Reader.ReadString('\n')` (`r.session.In`), with no raw terminal mode, no line editing, no history, and no completion of any kind. A user typing `/fea` and pressing Tab gets a literal tab character inserted into the line — `ParseLine` then fails to recognize `/fea\t...` as a known command.

**The maintainer's ask, verbatim in spirit:** an interactive terminal experience closer to what Claude Code, Codex, and OpenCode's own CLIs already offer — autocomplete over available slash commands (`/feature`, `/bug`, `/review`, `/release`, `/init`, `/help`, plus whatever a project's own `CommandRegistry` adds), live suggestions as the user types, and a more visually considered prompt/output than today's plain `fmt.Fprint` calls in `cli/progress.go`/`cli/render.go`. This is explicitly **not** a request to build a web or GUI surface — Foundry stays a terminal program; the ask is to make the terminal experience itself richer.

**Building this with the standard library alone is not realistic.** Raw terminal mode (disabling line buffering and local echo to read keystrokes one at a time), cursor/line redraw for live suggestion rendering, and a completion-aware line editor are exactly the kind of complex, easy-to-get-subtly-wrong terminal-handling code (`termios`-equivalent per-OS syscalls, ANSI escape sequence parsing/emission, Unicode-aware cursor math) that "no premature abstraction" does not mean "reimplement `readline` from scratch." This is the first time in Foundry's history that a real capability requires reaching outside the standard library.

**This is therefore the trigger [ADR-0001](ADR-0001-language-and-toolchain.md) itself flagged and left undecided**: its own Future ADR Dependencies section names a "Possible new ADR (Dependency & supply-chain policy): vendoring, module-proxy, and dependency-vetting policy is implied by choosing Go's module ecosystem but is not decided here; flag for later if the dependency surface grows." The surface is growing now, for the first time — this ADR is that flag firing, scoped narrowly to the concrete case in front of it rather than writing a general policy speculatively.

**R1 (single static binary, `CGO_ENABLED=0` on the default build path) must survive whatever is chosen.** Any candidate library is disqualified outright if it requires cgo on the default build.

## Decision

1. **Foundry may depend on external, pure-Go modules for the interactive session's presentation layer — its first such dependency ever — scoped narrowly to this concrete need, not as a general "dependencies are now fine" precedent.** Specifically: [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) (the terminal event loop and raw-mode input handling), [`github.com/charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) (its `textinput`/`list` components, which already implement Tab-completion-style suggestion UX), and [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) (terminal styling — color, borders, layout) — three modules from the same actively-maintained ecosystem, already the de facto standard for exactly this kind of Go CLI (used by `gh`'s own extensions, `glow`, `gum`, and numerous other production terminal tools). All three are pure Go, no cgo, fully compatible with `CGO_ENABLED=0` static builds, preserving R1.

2. **This dependency is confined to the presentation layer — `session/` and `cli/` — and must never leak into `domain/`, `engine/`, `record/`, `verify/`, `gatherer/`, `knowledge/`, `replay/`, `workspace/`, or `executor/`.** Those packages remain exactly as dependency-free as they are today. This mirrors [I12](../05-reference/invariants.md) (the model is substrate, never the domain center): a terminal-rendering library is substrate for *how a human sees and types*, with the same discipline applied — it must never become load-bearing for what an Act *is* or how it is judged. `session.CommandRegistry.Dispatch`, `ParseLine`, and every domain/engine port are unchanged by this decision; only `REPL.Run`'s input-acquisition loop and `cli`'s rendering helpers are replaced.

3. **Scope of the interactive experience, v1:** Tab-completion over the current `CommandRegistry`'s registered slash-command names (already known statically at prompt time — no new port or discovery mechanism needed) and arrow-key command history within a single session. Live suggestions render inline as the user types, styled via `lipgloss`. Explicitly **not** in this first slice: completion of Pipeline *arguments* (e.g. suggesting file paths or Act IDs inside a command's own arguments), persistent cross-session history, or a themeable/configurable prompt — each is a separate, later increment once the base line-editing loop is real, not a reason to over-build this one.

4. **This ADR does not establish a general dependency-vetting or vendoring policy.** It licenses exactly the three named modules for exactly the named purpose. A future, unrelated dependency proposal (for any other package, for any other reason) still needs its own justification — this ADR is not a blanket "third-party dependencies are now unreviewed." If the dependency surface keeps growing, a genuine "Dependency & supply-chain policy" ADR remains a real, separate, not-yet-written decision — named here, not resolved here.

## Consequences

- **Positive:** the interactive session — [ADR-0009](ADR-0009-cli-and-output-contract.md)'s ratified *primary* interface — gets materially easier to use daily, closing a real gap against the terminal UX of comparable tools. Confining the dependency to `session/`+`cli/` keeps the deterministic core (`domain/`, `engine/`, `record/`, `verify/`) exactly as auditable and dependency-free as [ADR-0001](ADR-0001-language-and-toolchain.md) intended.
- **Cost:** `go.mod` gains its first `require` block; `go build`/`go mod download` now fetch external code, and Foundry's supply-chain surface is no longer literally zero. `go.sum` must be committed and reviewed like any other change per the repo's normal contribution rules — no special new process invented here, since point 4 above declines to write one.
- **Harder:** none identified against R1 — all three modules are confirmed pure Go with no cgo requirement.

## Migration Strategy

1. Add `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss` to `go.mod`/`go.sum`.
2. Replace `session/repl.go`'s `REPL.Run` read loop with a `bubbletea` program wrapping a `bubbles/textinput` model (completion candidates sourced from `r.commands`'s registered names), keeping `handleLine`/`dispatchRecovered`/`ParseLine` as the same dispatch path underneath — only how a line is *acquired and rendered* changes.
3. Restyle `cli/progress.go`/`cli/render.go`'s existing output helpers with `lipgloss` where they already emit ANSI color today (`colorEnabled`, `renderVerdict`, `renderDiff`) rather than introducing a second, competing styling mechanism alongside them.
4. Update [roadmap.md](../00-overview/roadmap.md) with the new "Interactive terminal UX" parallel track (added alongside this ADR) and [implementation-status.md](../00-overview/implementation-status.md) once code lands.
5. This is a genuinely new implementation PR, not a ratify-only ADR like ADR-0007/ADR-0003 — code changes follow ratification, not before.

## Review Checklist

- [ ] Maintainer has confirmed `charmbracelet/bubbletea`/`bubbles`/`lipgloss` (vs. an alternative, e.g. a hand-rolled `golang.org/x/term`-based line editor, or `chzyer/readline`) as the preferred dependency.
- [ ] R1 (static binary, `CGO_ENABLED=0`) re-confirmed against the actual `go build` output once the dependency is vendored.
- [ ] Confirmed no import of `bubbletea`/`bubbles`/`lipgloss` leaks outside `session/`+`cli/`.
