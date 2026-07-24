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
func SupportedModels() []model.Info {
	return []model.Info{
		{ID: "gpt-5.1", Executor: "openai", Provider: "OpenAI", DisplayName: "GPT-5.1"},
		{ID: "gpt-5.1-mini", Executor: "openai", Provider: "OpenAI", DisplayName: "GPT-5.1 Mini"},
		{ID: "llama3", Executor: "openai-compatible", Provider: "Meta (via Ollama)", DisplayName: "Llama 3 (local, via Ollama)"},
		{ID: "openai/gpt-4.1", Executor: "openai-compatible", Provider: "OpenAI (via GitHub Models)", DisplayName: "GPT-4.1 (GitHub Models free tier)"},
	}
}
