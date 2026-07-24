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
func SupportedModels() []model.Info {
	return []model.Info{
		{ID: "gemini-3.6-flash", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.6 Flash"},
		{ID: "gemini-3.5-flash", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.5 Flash"},
		{ID: "gemini-3.5-flash-lite", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.5 Flash Lite"},
		{ID: "gemini-3.1-flash-lite", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.1 Flash Lite"},
		{ID: "gemini-3.1-pro", Executor: "gemini", Provider: "Google", DisplayName: "Gemini 3.1 Pro"},
	}
}
