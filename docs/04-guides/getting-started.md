# Getting Started

> Install Foundry and produce your first **Act** against a real repository. Concepts used below — **Act**, **Executor**, **Pipeline**, **Step**, **Authored Knowledge** — are defined once in [../05-reference/terminology.md](../05-reference/terminology.md); this guide only shows how to invoke them. For what's actually shipped vs. still planned, see [../00-overview/roadmap.md](../00-overview/roadmap.md)'s current-status table.

## Requirements

| Dependency | Needed for | Notes |
|---|---|---|
| [Go](https://go.dev/dl/) 1.21+ | Building the binary | Build-time only — not required after `foundry` is installed. |
| `git` | Every Act | Foundry isolates each Act's patch on a throwaway branch (`workspace.NewWorkspace`). The target directory must already be a git repository with at least one commit — Foundry never runs `git init` for you. |
| [Claude Code CLI](https://github.com/anthropics/claude-code) (`claude`), installed and authenticated | The default Executor | `foundry do` and the interactive session call `claude -p` as a subprocess. Foundry reads no API key for it — authentication is Claude Code's own. This is the only Executor available with zero extra configuration. |
| An OpenAI API key *(optional)* | A second, named Executor | Only needed if you want to pin specific Pipeline Steps to OpenAI instead of Claude Code — see [Configuring a second Executor](#configuring-a-second-executor-optional) below. |
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

Each Pipeline's `verify` Step(s) must pass before you're ever asked to approve anything — a failing verification stops the attempt (and retries once, bounded, if the Pipeline declares repair) rather than reaching approval. When it does reach an `approve` Step, you'll see the proposed patch and its verdict, then a `y/n` prompt; declining leaves your repository untouched. On approval, the patch is applied and the Act is recorded immutably under `.foundry/acts/` in your repository.

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
  "gpt": { "vendor": "openai", "model": "gpt-4.1", "api_key_env": "OPENAI_API_KEY" }
}
```

Then reference the name (`"gpt"` above) from a Step's `executor` field in a Pipeline document — see [pipelines.md](pipelines.md). A missing `executors.json` means only the default Executor exists; nothing above is required to use Foundry at all. `openai` is the only supported vendor today ([ADR-0005](../03-adrs/ADR-0005-executor-contract-and-capability-model.md)); an unrecognized vendor is a clear configuration error, not a silent fallback.

## Project configuration (optional)

`.foundry/config.json` controls a few opt-in behaviors, all defaulting to "off":

```json
{
  "docs_path": "docs/CHANGELOG.md",
  "require_approval_before_remote_publish": true,
  "remote_publish_token_env": "GH_TOKEN"
}
```

- `docs_path` — enables the `project-doc` apply target, appending an Act's output to this file.
- `require_approval_before_remote_publish` / `remote_publish_token_env` — enable the `remote-pr` apply target ([ADR-0010](../03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md)), which pushes a branch and opens a pull request via `gh`. A Pipeline declaring `remote-pr` with no preceding `approve` Step is refused when it's loaded, not silently allowed to skip human approval.

## Authored Knowledge (optional)

A Pipeline's `apply` Step can declare `"target": "knowledge-note"` to write its output as a plain Markdown file under `.foundry/knowledge/`, one per contributing Act. A later Act's naive lexical retrieval automatically surfaces relevant notes back into its own considered Evidence — no separate indexing step needed. See [../02-architecture/knowledge.md](../02-architecture/knowledge.md) and [RFC-0005](../01-rfcs/RFC-0005-authored-knowledge-retrieval.md) (still Draft — Proposed, not yet ratified).

## What "usable for testing" means today, honestly

Real: it builds a real patch through a real Executor against a real repository, verifies it, requires human approval, and records every Act immutably; replay and resume both work. Not yet real: multi-user use, true capability-based Executor routing (today's Router is explicit-pin-only), Derived Knowledge, or any observability surface. [../00-overview/roadmap.md](../00-overview/roadmap.md)'s current-status table is the honest, up-to-date list of what's shipped per milestone.

## Troubleshooting

- **`claude: executable "claude" not found in PATH`** — install the Claude Code CLI, or configure a named OpenAI Executor above and pin every `generate` Step to it.
- **Foundry refuses to start in a directory** — the target must already be a git repository with at least one commit; `foundry` does not initialize one for you.
- **A Pipeline with a `remote-pr` apply Step fails to load** — if `require_approval_before_remote_publish` is `true` in `.foundry/config.json`, that Pipeline must declare an `approve` Step before the `remote-pr` one, or registration refuses it outright.
