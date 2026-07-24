package openai

import "foundry/model"

// SupportedModels returns this package's known models as Model Registry
// catalog metadata (ADR-0013). executor/openai backs two vendor strings
// (project.ExecutorConfig.Vendor): "openai" (OpenAI's own API, via
// NewExecutor) and "openai-compatible" (any other endpoint speaking the
// same Chat Completions shape, via NewExecutorWithEndpoint — Ollama,
// Groq, DeepSeek, GitHub Models, ...). The two OpenAI-hosted models below
// are registered under "openai"; the "openai-compatible" entries name a
// couple of already-referenced, real examples (this repository's own
// .foundry/executors.json/.foundry/config.json) rather than an
// exhaustive catalog of every compatible endpoint.
//
// Capabilities/Limits/Quality are hand-curated best-effort metadata
// (model.Info's own doc comment) — not fetched from any live API, and
// expected to drift as vendors ship new models.
func SupportedModels() []model.Info {
	return []model.Info{
		{
			ID: "gpt-5.1", Executor: "openai", Provider: "OpenAI", DisplayName: "GPT-5.1",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: true, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 128000},
			Quality:      model.Quality{Reasoning: 5, Coding: 5, Review: 4},
		},
		{
			ID: "gpt-5.1-mini", Executor: "openai", Provider: "OpenAI", DisplayName: "GPT-5.1 Mini",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 128000},
			Quality:      model.Quality{Reasoning: 3, Coding: 3, Review: 3},
		},
		{
			ID: "llama3", Executor: "openai-compatible", Provider: "Meta (via Ollama)", DisplayName: "Llama 3 (local, via Ollama)",
			Capabilities: model.Capabilities{ToolUse: false, Thinking: false, Streaming: true, Multimodal: false, StructuredOutput: false},
			Limits:       model.Limits{MaxContext: 8192},
			Quality:      model.Quality{Reasoning: 2, Coding: 2, Review: 2},
		},
		{
			ID: "openai/gpt-4.1", Executor: "openai-compatible", Provider: "OpenAI (via GitHub Models)", DisplayName: "GPT-4.1 (GitHub Models free tier)",
			Capabilities: model.Capabilities{ToolUse: true, Thinking: false, Streaming: true, Multimodal: true, StructuredOutput: true},
			Limits:       model.Limits{MaxContext: 1000000},
			Quality:      model.Quality{Reasoning: 4, Coding: 4, Review: 4},
		},
	}
}
