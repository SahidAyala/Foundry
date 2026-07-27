package claude

import "github.com/SahidAyala/Foundry/model"

// SupportedModels returns Claude Code's known models as Model Registry
// catalog metadata (ADR-0013). This is informational only:
// ClaudeExecutor has no model-selection parameter of its own today (see
// this package's doc comment — Execute always runs the Claude Code CLI's
// own default) — registering these entries does not yet let a caller
// select among them; it only records that the underlying CLI is capable
// of running them.
//
// Capabilities/Limits/Quality are hand-curated best-effort metadata
// (model.Info's own doc comment) — not fetched from any live Anthropic
// API, and expected to drift as Anthropic ships new models.
func SupportedModels() []model.Info {
	return []model.Info{
		{
			ID: "claude-opus-4-8", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Opus 4.8",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 200000},
			Quality:      model.Quality{Reasoning: 5, Coding: 5, Review: 5},
		},
		{
			ID: "claude-sonnet-5", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Sonnet 5",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 200000},
			Quality:      model.Quality{Reasoning: 4, Coding: 5, Review: 4},
		},
		{
			ID: "claude-haiku-4-5-20251001", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Haiku 4.5",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 200000},
			Quality:      model.Quality{Reasoning: 3, Coding: 3, Review: 3},
		},
		{
			ID: "claude-fable-5", Executor: "claude", Provider: "Anthropic", DisplayName: "Claude Fable 5",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 200000},
			Quality:      model.Quality{Reasoning: 3, Coding: 2, Review: 3},
		},
	}
}
