package claude

import "github.com/SahidAyala/Foundry/model"

// SupportedModels returns Claude Code's known models as Model Registry
// catalog metadata (ADR-0013). Since ADR-0013's post-ratification "Claude
// Code model selection" note, ClaudeExecutor does have a real
// model-selection parameter (NewClaudeExecutor's model argument, passed
// through to the CLI's own --model flag) — but every entry below still
// shares one Executor value ("claude"), the same static-catalog shape
// executor/geminicli's own SupportedModels() already uses. A Step naming
// `model`/`preferred` only resolves correctly if the project's own
// `.foundry/executors.json` names an entry literally "claude" *and* that
// entry's own configured model happens to match the specific ID the Step
// named — a named executor entry is bound to one fixed model at
// construction, so a project wanting more than one Claude model
// selectable this way would need one distinctly-named entry per model,
// with the catalog's shared "claude" Executor value unable to
// distinguish between them. The safe, unambiguous way to pin a specific
// Claude model per Step today is a direct `executor` pin naming that
// project-configured entry (e.g. `{"executor": "claude-fable"}`), not
// `model`/`preferred` — see pipelines.md's "Pinning a Step to a Model"
// section for the same caveat as it already applies to Gemini.
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
