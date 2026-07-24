package claude

import "foundry/model"

// SupportedModels returns Claude Code's known models as Model Registry
// catalog metadata (ADR-0013). This is informational only:
// ClaudeExecutor has no model-selection parameter of its own today (see
// this package's doc comment — Execute always runs the Claude Code CLI's
// own default) — registering these entries does not yet let a caller
// select among them; it only records that the underlying CLI is capable
// of running them.
func SupportedModels() []model.Info {
	return []model.Info{
		{ID: "claude-opus-4-8", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Opus 4.8"},
		{ID: "claude-sonnet-5", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Sonnet 5"},
		{ID: "claude-haiku-4-5-20251001", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Haiku 4.5"},
		{ID: "claude-fable-5", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Fable 5"},
	}
}
