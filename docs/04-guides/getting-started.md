# Getting Started

> Install Foundry and produce your first **Act** against a real repository. Concepts used below — **Act**, **Executor**, **Pipeline**, **Step**, **Authored Knowledge** — are defined once in [../05-reference/terminology.md](../05-reference/terminology.md); this guide only shows how to invoke them. For what's actually shipped vs. still planned, see [../00-overview/roadmap.md](../00-overview/roadmap.md)'s current-status table.

## Requirements

| Dependency | Needed for | Notes |
|---|---|---|
| [Go](https://go.dev/dl/) 1.21+ | Building the binary | Build-time only — not required after `foundry` is installed. |
| `git` | Every Act | Foundry isolates each Act's patch on a throwaway branch (`workspace.NewWorkspace`). The target directory must already be a git repository with at least one commit — Foundry never runs `git init` for you. |
| [Claude Code CLI](https://github.com/anthropics/claude-code) (`claude`), installed and authenticated | The default Executor | `foundry do` and the interactive session call `claude -p` as a subprocess. Foundry reads no API key for it — authentication is Claude Code's own. This is the only Executor available with zero extra configuration. |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) (`gemini`), installed and signed in *(optional)* | A second, named Executor, no API key | Only needed if you want to pin specific Pipeline Steps to Gemini. Run `gemini` once and pick "Sign in with Google" — Foundry never reads an API key for it, the same way it never reads one for Claude Code. Gemini's free tier needs no credit card. |
| An OpenAI or Gemini API key *(optional, last resort)* | A second, named Executor, no CLI/browser login | Only needed for `openai` or `gemini-api` (see [Configuring a second Executor](#configuring-a-second-executor-optional) below) — CI or any environment where a one-time browser login is never possible. Prefer the Gemini CLI row above when you can. |
| `gh` CLI, authenticated *(optional)* | The `remote-pr` apply target | Only needed if a Pipeline opens a pull request directly ([ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md)). Not required for local use. |

## Install

```
curl -fsSL https://raw.githubusercontent.com/SahidAyala/Foundry/main/install.sh | bash
```

Or from a local clone: `git clone git@github.com:SahidAyala/Foundry.git && cd Foundry && ./install.sh`. Either way this builds `foundry` and installs it to `/usr/local/bin` (override with `FOUNDRY_INSTALL_DIR`). See the repository [README](../../README.md#install) for the full install script behavior.

## First steps

**Foundry's primary interface is the interactive session** — a persistent, slash-command-driven assistant resident in your project, not a flag-parsed shell command ([RFC-0003](../01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md)). The one-shot `foundry do` command below exists specifically for CI and automation scripting; it is not a second, equally-preferred way to use Foundry day to day.

### Interactive session — the primary way to use Foundry

```
cd /path/to/your/repo   # must already be a git repository, at least one commit
foundry
```

Running `foundry` with no arguments opens an interactive session rooted at the current directory. It understands a small set of slash commands:

| Command | Effect |
|---|---|
| `/init` | Scaffolds `.foundry/pipelines/{feature,bugfix,release}.json` starter Pipeline documents. Safe to re-run — never overwrites a file that already exists. |
| `/feature "<intent>"` | Runs the `feature` Pipeline: `plan → approve-plan → implement → verify → approve-outcome → apply → record`. |
| `/bug "<intent>"` | Runs the `bugfix` Pipeline: `implement → verify → approve → apply → record`. |
| `/review "<intent>"` | Runs the built-in `review` Pipeline: two independent `verify` Steps against one Outcome. |
| `/release "<intent>"` | Runs the `release` Pipeline: `prepare → verify → verify-checklist → approve → apply → record`, no repair. |
| `/help` | Lists every slash command this session understands, by name and one-line description. |
| `/exit` or `/quit` | End the session. |

When run from a real terminal ([ADR-0012](../03-adrs/ADR-0012-interactive-terminal-ux-and-first-dependency.md)), the prompt opens a full, arrow-navigable menu of the table above the moment you type `/`, filtering as you keep typing; Up/Down also recalls previously submitted lines, remembered across runs in `.foundry/history` (see below). Piped or redirected input/output (scripting, tests) falls back to a plain prompt with no menu or recall.

Each Pipeline's `verify` Step(s) must pass before you're ever asked to approve anything — a failing verification stops the attempt (and retries once, bounded, if the Pipeline declares repair) rather than reaching approval. When it does reach an `approve` Step, you'll see the proposed patch and its verdict, then a `y/n` prompt; declining leaves your repository untouched. On approval, the patch is applied and the Act is recorded immutably under `.foundry/acts/` in your repository. A patch longer than 40 lines is piped through `$PAGER` (falling back to `less -R`) so it doesn't scroll past what you can read before deciding — `foundry show`'s output gets the same treatment.

Per [ADR-0002](../03-adrs/ADR-0002-persistence-content-addressing-and-on-disk-layout.md): commit `.foundry/acts/` to your project's own repository — it is durable audit history, the same way `.foundry/pipelines/` already is, not a disposable cache. `.foundry/acts/*/checkpoint.json` (an interrupted Act's in-progress state, `foundry resume`'s own bookkeeping) is the one exception — it has no audit value once superseded or deleted, so you may gitignore `**/checkpoint.json` if you prefer.

The session also remembers what you've typed across runs, in `.foundry/history` (ADR-0012) — arrow-key recall survives closing and reopening `foundry`. Unlike the paths above, this one is personal, ephemeral convenience state, not audit-worthy Evidence or Knowledge; gitignore `.foundry/history` rather than committing it.

Plain text typed at the prompt (not a slash command) is not yet supported — use one of the commands above.

### One-shot command — for CI and automation scripting only

```
foundry do "<intent>" --repo /path/to/your/repo
```

Runs the same lifecycle through the built-in `default` Pipeline (`generate → verify`, one bounded repair). This exists so Foundry can be driven from a pipeline or script where nothing is present to answer an interactive prompt — reach for the interactive session above for anything you're doing yourself.

### Inspecting history

```
foundry log --repo /path/to/your/repo              # list recorded Acts
foundry show <act-id> --repo /path/to/your/repo    # show one Act in full
foundry replay <act-id> --repo /path/to/your/repo  # re-run verification, check reproducibility
foundry resume --repo /path/to/your/repo           # continue an Act interrupted mid-Pipeline
```

## Configuring a second Executor (optional)

To pin a Pipeline Step to a vendor other than the default Claude Code Executor, add `.foundry/executors.json` to your project:

```json
{
  "gpt": { "vendor": "openai", "model": "gpt-4.1", "api_key_env": "OPENAI_API_KEY" },
  "flash": { "vendor": "gemini", "model": "gemini-3.5-flash" },
  "flash-ci": { "vendor": "gemini-api", "model": "gemini-3.5-flash", "api_key_env": "GEMINI_API_KEY" },
  "local-llama": { "vendor": "openai-compatible", "model": "llama3", "base_url": "http://localhost:11434/v1/chat/completions" },
  "gh-copilot": { "vendor": "copilot" }
}
```

Then reference the name (`"gpt"`/`"flash"`/`"flash-ci"`/`"local-llama"`/`"gh-copilot"` above) from a Step's `executor` field in a Pipeline document — see [pipelines.md](pipelines.md). A missing `executors.json` means only the default Executor exists; nothing above is required to use Foundry at all. `openai`, `gemini`, `gemini-api`, `copilot`, and `openai-compatible` are the supported vendors today ([ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md)); an unrecognized vendor is a clear configuration error, not a silent fallback.

`copilot` runs the [GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli) (`copilot`, GA since 2026-02-25) the same "delegate auth to the vendor's own CLI" way `gemini` does — it reads no key Foundry manages; the CLI reads its own `COPILOT_GITHUB_TOKEN` or reuses an interactive `gh auth login` session. Unlike Claude Code and the Gemini CLI, the Copilot CLI is genuinely agentic (it ships its own file/shell tools) — this Executor deliberately grants it none and additionally checks the workspace's `git status` before and after every call, refusing outright if anything changed, so a real or future change in the CLI's own default behavior can never bypass Foundry's own approval gate silently. **This vendor has not been validated against a real `copilot` CLI** (none installed in this environment) — try it cautiously, and watch the first few runs closely, before trusting it in an unattended Pipeline.

`openai-compatible` reuses the same client as `openai` against any endpoint that speaks the same Chat Completions request/response shape — `base_url` is required. This covers, among others:

| Provider | `base_url` | Notes |
|---|---|---|
| [Ollama](https://ollama.com) | `http://localhost:11434/v1/chat/completions` | Fully local and free — no signup, no key, no rate limit; needs the model already pulled (`ollama pull llama3`) and reasonable local hardware. `api_key_env` can be left unset. |
| [Groq](https://groq.com) | `https://api.groq.com/openai/v1/chat/completions` | Free tier, very fast inference, open (Llama-class) models. |
| [DeepSeek](https://platform.deepseek.com) | `https://api.deepseek.com/chat/completions` | Not free, but inexpensive, and a genuinely strong coding model. |
| [GitHub Models](https://docs.github.com/en/github-models) | check current docs for the exact endpoint | Free tier; authenticates with a GitHub PAT — the same credential class `remote_publish_token_env` already uses for `gh`. |

Amazon Q Developer was considered and deliberately skipped: AWS announced its end-of-support in 2026, closing new signups (including its free tier) — not a fit to build against.

`gemini` (recommended) runs the Gemini CLI as a subprocess, the same way the default Claude Code Executor does — it needs no `api_key_env` at all, since the CLI's own cached "Sign in with Google" login (`gemini`, run once interactively) is reused for every later headless call. `gemini-api` calls Gemini's REST API directly with a raw key instead — kept available deliberately as a last resort for environments where a one-time browser login is never possible (e.g. some CI runners), not as the default path. Gemini's free tier needs no credit card for either — [ai.google.dev/gemini-api/docs/pricing](https://ai.google.dev/gemini-api/docs/pricing) has current limits.

## Project configuration (optional)

`.foundry/config.json` controls a few opt-in behaviors, all defaulting to "off":

```json
{
  "docs_path": "docs/CHANGELOG.md",
  "require_approval_before_remote_publish": true,
  "remote_publish_token_env": "GH_TOKEN",
  "request_copilot_review": true
}
```

- `docs_path` — enables the `project-doc` apply target, appending an Act's output to this file.
- `require_approval_before_remote_publish` / `remote_publish_token_env` — enable the `remote-pr` apply target ([ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md)), which pushes a branch and opens a pull request via `gh`. A Pipeline declaring `remote-pr` with no preceding `approve` Step is refused when it's loaded, not silently allowed to skip human approval.
- `request_copilot_review` — after `remote-pr` opens a pull request, also ask GitHub Copilot to review it (`gh pr edit --add-reviewer @copilot`). Has no effect unless `remote_publish_token_env` is set too. Requires a paid Copilot plan on the repository/organization — Foundry can't detect whether that's available, so this defaults to off. A failure to request the review (no such plan, the feature not enabled) is printed as a warning but never fails the Act — the pull request itself has already been opened by that point.

## Authored Knowledge (optional)

A Pipeline's `apply` Step can declare `"target": "knowledge-note"` to write its output as a plain Markdown file under `.foundry/knowledge/`, one per contributing Act. A later Act's naive lexical retrieval automatically surfaces relevant notes back into its own considered Evidence — no separate indexing step needed. See [../02-architecture/knowledge.md](../02-architecture/knowledge.md) and [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) (still Draft — Proposed as an RFC, though the store's own format and durability questions are ratified by [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md)).

Per [ADR-0007](../03-adrs/ADR-0007-knowledge-and-semantic-store.md): commit `.foundry/knowledge/` to your project's own repository, the same way `.foundry/acts/` and `.foundry/pipelines/` already are — a Knowledge note is durable audit-adjacent history, not a disposable cache.

## Structured logging (optional)

Setting the `FOUNDRY_LOG` environment variable to any non-empty value adds structured, leveled JSON log lines to stderr for every Act — one line per lifecycle event (gather start, each execute/verify round, repair, budget exceeded) — alongside the normal human-readable progress on stdout. Unset (the default), nothing changes: only the existing human narration runs. This is a diagnostic/observability stream, not a machine-readable *output* contract — `foundry show`/`log`'s own output format is unaffected (no `--json` mode exists; see [ADR-0009](../03-adrs/ADR-0009-cli-and-output-contract.md)).

```
FOUNDRY_LOG=1 foundry do "<intent>" --repo /path/to/your/repo 2>foundry.jsonl
```

## What "usable for testing" means today, honestly

Real: it builds a real patch through a real Executor against a real repository, verifies it, requires human approval, and records every Act immutably; replay and resume both work; `foundry show` renders a colored, per-Step trace of what happened. Not yet real: multi-user use, true capability-based Executor routing (today's Router is explicit-pin-only), Derived Knowledge. [../00-overview/roadmap.md](../00-overview/roadmap.md)'s current-status table is the honest, up-to-date list of what's shipped per milestone.

## Troubleshooting

- **`claude: executable "claude" not found in PATH`** — install the Claude Code CLI, or configure a named OpenAI or Gemini Executor above and pin every `generate` Step to it.
- **Foundry refuses to start in a directory** — the target must already be a git repository with at least one commit; `foundry` does not initialize one for you.
- **A Pipeline with a `remote-pr` apply Step fails to load** — if `require_approval_before_remote_publish` is `true` in `.foundry/config.json`, that Pipeline must declare an `approve` Step before the `remote-pr` one, or registration refuses it outright.
