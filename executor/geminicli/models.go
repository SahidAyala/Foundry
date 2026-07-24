package geminicli

import "foundry/model"

// SupportedModels returns Gemini's known models as Model Registry catalog
// metadata (ADR-0013), registered under the "gemini" Executor — the same
// vendor string cmd/foundry/main.go's namedExecutor resolves to this
// package (Foundry's preferred, CLI-based Gemini entry point). The same
// model IDs are also reachable through executor/gemini's HTTP API
// ("gemini-api"), but since each model belongs to exactly one Executor in
// this catalog, they are registered once here rather than duplicated
// under both vendor strings.
//
// Capabilities/Limits/Quality are hand-curated best-effort metadata
// (model.Info's own doc comment) — not fetched from any live Google API,
// and expected to drift as Google ships new models.
func SupportedModels() []model.Info {
	return []model.Info{
		{
			ID: "gemini-3.6-flash", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.6 Flash",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 1000000},
			Quality:      model.Quality{Reasoning: 4, Coding: 4, Review: 3},
		},
		{
			ID: "gemini-3.5-flash", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.5 Flash",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 1000000},
			Quality:      model.Quality{Reasoning: 3, Coding: 3, Review: 3},
		},
		{
			ID: "gemini-3.5-flash-lite", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.5 Flash Lite",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 128000},
			Quality:      model.Quality{Reasoning: 2, Coding: 2, Review: 2},
		},
		{
			ID: "gemini-3.1-flash-lite", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.1 Flash Lite",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 128000},
			Quality:      model.Quality{Reasoning: 2, Coding: 2, Review: 2},
		},
		{
			ID: "gemini-3.1-pro", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.1 Pro",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 1000000},
			Quality:      model.Quality{Reasoning: 5, Coding: 4, Review: 4},
		},
	}
}
