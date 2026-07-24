// Command foundry is the CLI entry point.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"foundry/cmd/foundry/commands"
	"foundry/engine"
	"foundry/executor/claude"
	"foundry/executor/copilotcli"
	"foundry/executor/gemini"
	"foundry/executor/geminicli"
	"foundry/executor/openai"
	"foundry/project"
	"foundry/session"
	"foundry/ticket"
	asanaticket "foundry/ticket/asana"
	githubticket "foundry/ticket/github"
	gitlabticket "foundry/ticket/gitlab"
	jiraticket "foundry/ticket/jira"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, claudeExecutor, namedExecutor))
}

// claudeExecutor is the production Executor factory: it invokes the Claude
// Code CLI against the Act's workspace. It is injected at the composition root
// so that main's tests can substitute a deterministic fixture and never depend
// on Claude Code being installed.
func claudeExecutor(workspace string) engine.Executor {
	return claude.NewClaudeExecutor(workspace)
}

// namedExecutor is the production vendor-dispatch factory (ADR-0005
// Decision 5, docs/03-adrs/ADR-0005-executor-contract-and-capability-model.md):
// it constructs a named, project-configured Executor from a
// project.ExecutorConfig decoded out of a project's .foundry/executors.json,
// and the project's workspace directory (needed by a subprocess-based
// vendor like executor/geminicli — a pure HTTP vendor like executor/openai
// or executor/gemini simply ignores it). This is the one place in the whole
// binary that knows which concrete vendor packages exist — project,
// session, and cmd/foundry/commands stay vendor-agnostic, calling only this
// function through the project.ExecutorConstructor seam.
//
// "gemini" resolves to executor/geminicli, not executor/gemini's own HTTP
// API-key path — the maintainer's own framing was that a raw API key
// should be a last resort, not the default: the Gemini CLI's "Sign in with
// Google" login (cached to disk, reused headlessly on later runs — see
// executor/geminicli's own package doc) needs no key Foundry ever reads.
// "gemini-api" names that HTTP path explicitly, for environments where no
// browser is ever available to complete that one-time login.
//
// "openai-compatible" reuses executor/openai's own Chat-Completions client
// against a caller-named base_url instead of writing a near-duplicate
// package per vendor: Ollama (free, local, no key), Groq, DeepSeek, and
// several other providers all document an explicit OpenAI-compatible
// endpoint, so one client already covers all of them.
//
// "copilot" resolves to executor/copilotcli, delegating generate Steps (not
// just PR review — see vcs.GitHubPRApplier's own RequestCopilotReview) to
// the GitHub Copilot CLI, the same "delegate auth to the vendor's own CLI"
// pattern as "gemini". Unlike Claude Code and the Gemini CLI, the Copilot
// CLI is genuinely agentic — see executor/copilotcli's own package doc for
// the specific safeguard (no tool grants, plus a git-status check) this
// demands, and why it has not been validated live in this environment.
//
// Adding any of this needed no new architectural decision: ADR-0005's
// Executor contract and ADR-0006's explicit-pin routing already cover any
// number of named vendors. An unrecognized vendor is a clear, named
// configuration error rather than a silent no-op.
func namedExecutor(cfg project.ExecutorConfig, workspace string) (engine.Executor, error) {
	switch cfg.Vendor {
	case "openai":
		return openai.NewExecutor(cfg.Model, os.Getenv(cfg.APIKeyEnv)), nil
	case "gemini":
		return geminicli.NewExecutor(workspace, cfg.Model), nil
	case "gemini-api":
		return gemini.NewExecutor(cfg.Model, os.Getenv(cfg.APIKeyEnv)), nil
	case "copilot":
		return copilotcli.NewExecutor(workspace, cfg.Model), nil
	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("foundry: vendor %q requires base_url in .foundry/executors.json (e.g. Ollama, Groq, DeepSeek — any endpoint speaking the Chat Completions shape)", cfg.Vendor)
		}
		return openai.NewExecutorWithEndpoint(cfg.Model, os.Getenv(cfg.APIKeyEnv), cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("foundry: unsupported executor vendor %q (supported: %q, %q, %q, %q, %q)", cfg.Vendor, "openai", "gemini", "gemini-api", "copilot", "openai-compatible")
	}
}

// newTicketFetcher is the production ticket-fetcher vendor-dispatch
// factory (mirroring namedExecutor's own shape) for /issue's own external
// system boundary (docs/02-architecture/system-context.md), covering all
// four providers the maintainer asked for, in the order named as most
// common: "github" resolves to ticket/github, reusing the exact same
// already-authenticated gh CLI session vcs.GitHubPRApplier's own
// PR-opening already requires — Foundry reads no separate credential for
// it. "jira" resolves to ticket/jira, a pure HTTP call authenticating
// with Basic Auth (cfg.JiraEmail plus an API token resolved from
// cfg.JiraAPITokenEnv) — Jira has no equivalent already-authenticated CLI
// session to piggyback on. "gitlab" resolves to ticket/gitlab, mirroring
// GitHub's own approach by shelling out to the glab CLI's own
// already-authenticated session instead of a raw token. "asana" resolves
// to ticket/asana, a pure HTTP call like Jira's (a Bearer Personal Access
// Token resolved from cfg.AsanaAPITokenEnv) — Asana has no CLI
// convention either, and unlike Jira needs no separate base URL, since
// its API has one fixed global endpoint. Only runSession calls this, and
// only when project.Config.TicketProvider is set — /issue is entirely
// opt-in, exactly like RequestCopilotReview.
func newTicketFetcher(cfg project.Config, workspace string) (ticket.Fetcher, error) {
	switch cfg.TicketProvider {
	case "github":
		return githubticket.NewFetcher(workspace), nil
	case "jira":
		return jiraticket.NewFetcher(cfg.JiraBaseURL, cfg.JiraEmail, os.Getenv(cfg.JiraAPITokenEnv)), nil
	case "gitlab":
		return gitlabticket.NewFetcher(workspace), nil
	case "asana":
		return asanaticket.NewFetcher(os.Getenv(cfg.AsanaAPITokenEnv)), nil
	default:
		return nil, fmt.Errorf("foundry: unsupported ticket provider %q (supported: %q, %q, %q, %q)", cfg.TicketProvider, "github", "jira", "gitlab", "asana")
	}
}

// run's newNamedExecutor is variadic (pass zero or one) so every existing
// caller — production and test — that never configures a named Executor
// keeps compiling and behaving identically; only main's own call above
// passes the real dispatch.
func run(args []string, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor ...project.ExecutorConstructor) int {
	var construct project.ExecutorConstructor
	if len(newNamedExecutor) > 0 {
		construct = newNamedExecutor[0]
	}

	if len(args) == 0 {
		return runSession(context.Background(), stdin, stdout, newExecutor, construct)
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage())
		return 0
	case "do":
		return commands.Do(context.Background(), args[1:], stdin, stdout, newExecutor, construct)
	case "log":
		return commands.Log(context.Background(), args[1:], stdout)
	case "show":
		return commands.Show(context.Background(), args[1:], stdout)
	case "replay":
		return commands.Replay(context.Background(), args[1:], stdout)
	case "resume":
		return commands.Resume(context.Background(), args[1:], stdin, stdout, newExecutor, construct)
	default:
		fmt.Fprintf(stdout, "foundry: unknown command %q\n\n", args[0])
		fmt.Fprint(stdout, usage())
		return 2
	}
}

// runSession starts an interactive session rooted at the current
// working directory — this is what `foundry` with no subcommand at all
// now does, replacing the old "print usage, exit 2" behavior. The
// one-shot do/log/show subcommands below remain available unchanged for
// scripting and CI; the interactive session is additive, not a
// replacement
// (docs/01-rfcs/RFC-0003-interactive-assistant-and-multi-executor-pipelines.md
// §3.1).
func runSession(ctx context.Context, stdin io.Reader, stdout io.Writer, newExecutor func(workspace string) engine.Executor, newNamedExecutor project.ExecutorConstructor) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	s, err := session.NewSession(ctx, root, stdin, stdout, newExecutor, newNamedExecutor)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}

	cfg, err := project.LoadConfig(root)
	if err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	if cfg.TicketProvider != "" {
		fetcher, err := newTicketFetcher(cfg, root)
		if err != nil {
			fmt.Fprintln(stdout, err)
			return 1
		}
		s.SetTicketFetcher(fetcher)
	}

	repl := session.NewREPL(s, session.DefaultCommandRegistry())
	if err := repl.Run(ctx); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	return 0
}

// usage lists foundry's subcommands. Each subcommand's own --help gives
// its full usage.
func usage() string {
	return `Usage: foundry <command> [arguments]

Commands:
  do      Run the Act lifecycle for an Intent against a repository
  log     List recorded Acts for a repository
  show    Show one recorded Act in full
  replay  Re-run verification against a recorded Act and report reproducibility
  resume  Continue an Act interrupted mid-Pipeline, or list interrupted acts

Run 'foundry <command> --help' for details on a specific command.
`
}
