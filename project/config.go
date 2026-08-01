package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFile is the conventional location, relative to a project root,
// where LoadConfig reads a project's own settings (RFC-0004 §2.6, Piece 4;
// ADR-0010, Piece 6 — both of
// docs/04-guides/multi-executor-router-implementation-plan.md).
const ConfigFile = ".foundry/config.json"

// Config is a project's own settings, read once at composition-root wiring
// time.
type Config struct {
	// DocsPath is the project-relative file a "project-doc" apply Target
	// (RFC-0004 §2.6) writes a Knowledge-lite capture Step's Produced
	// prose into; empty means the project has not opted into that apply
	// target.
	DocsPath string `json:"docs_path"`

	// RequireApprovalBeforeRemotePublish gates PipelineRegistry's
	// SetPublishPolicy (ADR-0010,
	// docs/03-adrs/ADR-0010-vcs-pr-integration-and-apply-targets.md
	// Decision 3): true refuses, at Pipeline-registration time, any
	// Pipeline declaring a "remote-pr" apply Step with no preceding
	// approve Step. false (or the field's absence) means the project has
	// explicitly opted out of requiring one.
	RequireApprovalBeforeRemotePublish bool `json:"require_approval_before_remote_publish"`

	// RemotePublishTokenEnv names the environment variable
	// vcs.GitHubPRApplier reads its GitHub credential from at Apply time
	// (ADR-0010 Decision 7) — never persisted, logged, or passed through
	// domain.Intent or any recorded Evidence, mirroring
	// ExecutorConfig.APIKeyEnv's pattern. Empty means the project has not
	// configured a "remote-pr" apply target at all.
	RemotePublishTokenEnv string `json:"remote_publish_token_env"`

	// RequestCopilotReview, when true, asks GitHub Copilot to review every
	// pull request vcs.GitHubPRApplier opens (`gh pr edit --add-reviewer
	// @copilot`). Requires a paid Copilot plan on the repository/
	// organization and has no effect at all unless RemotePublishTokenEnv is
	// also set — false (or the field's absence) means a project has not
	// opted into this supplementary, best-effort request.
	RequestCopilotReview bool `json:"request_copilot_review"`

	// Validators, when non-empty, replaces verify.DefaultValidators' own
	// root-only auto-detection entirely with this project-declared list —
	// for a project DefaultValidators can't detect correctly (a polyglot
	// monorepo with no root go.mod/package.json, where its fallback
	// "repo-sanity" validator checks nothing real about the code). Empty
	// means auto-detection, exactly as before this field existed. Kept as
	// plain data here (not verify.Validator directly) so project never
	// needs to import verify — the composition root (session.Session,
	// cmd/foundry/commands) converts each entry at Gate-construction time.
	Validators []ValidatorConfig `json:"validators"`

	// PRSummaryModel, when set, has vcs.GitHubPRApplier ask Claude Code
	// (via a claude.Summarizer using this model — an alias like "haiku",
	// or a full model name) for a short PR title and body from an Act's
	// Intent and Patch, instead of its own mechanical "echo the Intent
	// verbatim" default. Empty means the mechanical default — has no
	// effect at all unless RemotePublishTokenEnv is also set (the
	// "remote-pr" apply target). A summarization failure is never fatal:
	// it falls back to the mechanical default, the same best-effort
	// treatment RequestCopilotReview's own failures already get.
	PRSummaryModel string `json:"pr_summary_model"`

	// TicketProvider names which external ticketing system the
	// interactive session's /issue command fetches from — "github",
	// "jira", "gitlab", "asana", or "backlog" today (see
	// docs/00-overview/implementation-status.md). Empty means /issue is
	// not configured; it reports a clear, named error if invoked rather
	// than guessing a provider.
	TicketProvider string `json:"ticket_provider"`

	// BacklogPath is the project-relative path ticket/backlog reads its
	// local JSON feature list from — ticket/backlog.DefaultPath
	// (.foundry/backlog.json) if empty. Ignored unless TicketProvider is
	// "backlog".
	BacklogPath string `json:"backlog_path"`

	// JiraBaseURL is a Jira Cloud site's own base URL (e.g.
	// "https://yourcompany.atlassian.net") — required when TicketProvider
	// is "jira"; ignored otherwise.
	JiraBaseURL string `json:"jira_base_url"`

	// JiraEmail is the Atlassian account email ticket/jira authenticates
	// as (Basic Auth's username half) — required when TicketProvider is
	// "jira"; ignored otherwise.
	JiraEmail string `json:"jira_email"`

	// JiraAPITokenEnv names the environment variable ticket/jira reads its
	// Atlassian API token from at Fetch time (id.atlassian.com/manage/
	// api-tokens) — never persisted, logged, or passed through
	// domain.Intent or any recorded Evidence, mirroring
	// ExecutorConfig.APIKeyEnv's pattern. Required when TicketProvider is
	// "jira"; ignored otherwise.
	JiraAPITokenEnv string `json:"jira_api_token_env"`

	// AsanaAPITokenEnv names the environment variable ticket/asana reads
	// its Personal Access Token from at Fetch time
	// (developers.asana.com/docs/personal-access-token) — never
	// persisted, logged, or passed through domain.Intent or any recorded
	// Evidence, mirroring ExecutorConfig.APIKeyEnv's pattern. Required
	// when TicketProvider is "asana"; ignored otherwise. Unlike Jira,
	// Asana needs no separate base URL — its API has one fixed endpoint
	// (app.asana.com) regardless of workspace.
	AsanaAPITokenEnv string `json:"asana_api_token_env"`

	// AIReviewModel, when set, adds a supplementary, non-deterministic
	// verify.aireview.Verifier alongside the deterministic Gate every
	// Pipeline already runs (composed via verify.Compose, never
	// replacing it — docs/02-architecture/trust.md's stated preference
	// for deterministic checks first). Empty means no AI review layer is
	// added at all, exactly as if this feature did not exist.
	AIReviewModel string `json:"ai_review_model"`

	// AIReviewBaseURL is the OpenAI-Chat-Completions-compatible endpoint
	// verify/aireview calls (the same shape the "openai-compatible"
	// Executor vendor already establishes — OpenAI, Gemini's API,
	// Ollama, Groq, DeepSeek, or any other endpoint speaking it).
	// Required whenever AIReviewModel is set; there is no single
	// "default" endpoint across vendors to fall back to.
	AIReviewBaseURL string `json:"ai_review_base_url"`

	// AIReviewAPIKeyEnv names the environment variable verify/aireview
	// reads its credential from at Verify time — never persisted, logged,
	// or passed through domain.Intent or any recorded Evidence, mirroring
	// ExecutorConfig.APIKeyEnv's pattern. May be left empty for an
	// endpoint with no auth of its own (e.g. a local Ollama instance).
	AIReviewAPIKeyEnv string `json:"ai_review_api_key_env"`
}

// ValidatorConfig is one custom Validator a project declares in its own
// "validators" list (Config.Validators) — the project-local escape hatch
// from verify.DefaultValidators' root-only auto-detection. Mirrors
// verify.Validator's own {Name, Cmd} shape without this package importing
// verify directly.
type ValidatorConfig struct {
	// Name labels this Validator's own line in a recorded Judgment's
	// Checked findings (verify.Result's own Name field) — must be unique
	// within a project's own Validators list, the same uniqueness
	// verify.NewGate itself does not enforce but a human reading recorded
	// Evidence needs to tell entries apart.
	Name string `json:"name"`

	// Cmd is the shell command Validator.Run executes in the Act's
	// workspace — a non-zero exit fails this Validator, exactly like
	// verify.DefaultValidators' own auto-detected commands (e.g.
	// "go build ./...", "go test ./..."). Runs via the same shell
	// invocation verify.Validator.Run already uses, so a monorepo's
	// per-subdirectory command (e.g. "cd events && go test ./...") works
	// unmodified.
	Cmd string `json:"cmd"`
}

// LoadConfig reads and decodes root's conventional configuration file
// (ConfigFile). A missing file is not an error — it decodes to the zero
// Config, mirroring LoadExecutorConfig's "missing file → empty" pattern: a
// project that never opts in sees every field's default (no DocsPath,
// today's only field), exactly as if this file's format didn't exist.
func LoadConfig(root string) (Config, error) {
	path := filepath.Join(root, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("project: read config %q: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("project: decode config %q: %w", path, err)
	}
	return config, nil
}
