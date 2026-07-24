package main

import (
	"foundry/executor/claude"
	"foundry/executor/copilotcli"
	"foundry/executor/geminicli"
	"foundry/executor/openai"
	"foundry/model"
)

// buildModelRegistry assembles a model.Registry from every concrete
// Executor package's own SupportedModels() (ADR-0013, Model Registry).
// This is purely additive metadata: nothing in Foundry's execution path
// (engine.Router, session.Session, wireEngine) constructs or consumes a
// Registry today — no Pipeline, Step, or Executor behavior changes as a
// result of this function's existence. It exists so a later, separately
// decided PR (e.g. a `foundry models` listing command) has a Registry to
// build from without a first migration to invent the shape.
//
// executor/gemini (the HTTP API vendor "gemini-api") is deliberately not
// included: it shares the exact same underlying models as executor/
// geminicli's own SupportedModels(), already registered under the
// "gemini" Executor — since each model belongs to exactly one Executor in
// this catalog, they are not duplicated under "gemini-api" too.
func buildModelRegistry() (*model.Registry, error) {
	registry := model.NewRegistry()
	sources := [][]model.Info{
		claude.SupportedModels(),
		geminicli.SupportedModels(),
		openai.SupportedModels(),
		copilotcli.SupportedModels(),
	}
	for _, infos := range sources {
		for _, info := range infos {
			if err := registry.Register(info); err != nil {
				return nil, err
			}
		}
	}
	return registry, nil
}
