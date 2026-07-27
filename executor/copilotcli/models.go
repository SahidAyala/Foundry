package copilotcli

import "github.com/SahidAyala/Foundry/model"

// SupportedModels returns a single informational entry for the "copilot"
// Executor (ADR-0013, Model Registry). Unlike executor/openai's or
// executor/geminicli's own price tables, this package has no confirmed,
// documented list of selectable model IDs for the Copilot CLI's --model
// flag (checked against GitHub's own CLI documentation before writing
// this — no such reference was found), so this entry names the fact
// that an unconfigured default exists rather than fabricating specific
// model names. For the same reason, Capabilities/Limits/Quality are left
// at their zero value ("not rated," per model.Info's own doc comment)
// rather than guessed — the underlying model is not confirmed, so neither
// is anything about what it supports.
func SupportedModels() []model.Info {
	return []model.Info{
		{ID: "copilot-default", Executor: "copilot", Provider: "GitHub Copilot", DisplayName: "GitHub Copilot CLI (default model)"},
	}
}
