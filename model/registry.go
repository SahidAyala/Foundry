// Package model defines a small, additive catalog of which named model an
// Executor vendor can run (ADR-0013, Model Registry). It is metadata only:
// nothing in Foundry's execution path (engine.Router, session.Session,
// engine.Engine) constructs, selects, or otherwise consumes a Registry
// today — Pipeline decoding, Step routing, and every existing Executor's
// behavior are all unchanged by this package's existence. It exists so a
// later, separately-decided PR (e.g. a `foundry models` listing command, or
// capability-aware routing once ADR-0006's own named trigger fires) has
// somewhere to build from, without a first migration to invent the shape.
package model

import (
	"fmt"
	"sort"
)

// Info is one named model's catalog metadata.
//
// ID is the string a caller passes to that Executor's own model parameter
// (project.ExecutorConfig.Model, or an Executor constructor's model
// argument) — the Registry does not itself construct or select an
// Executor; it only records that this ID is known to exist for this
// Executor.
//
// Executor names which Executor vendor invokes this model — the same
// vendor strings project.ExecutorConfig.Vendor and
// cmd/foundry/main.go's namedExecutor switch already use ("openai",
// "gemini", "gemini-api", "copilot", "openai-compatible"), plus "claude"
// for Foundry's default Executor (which has no vendor string of its own,
// since it is never constructed through that named-Executor registry).
//
// Provider is the upstream model provider (e.g. "Anthropic", "Google",
// "OpenAI") — distinct from Executor, since one Executor (notably
// "openai-compatible") can front models from more than one Provider.
//
// Each model belongs to exactly one Executor: Register refuses a second
// entry for an ID already registered, even under a different Executor,
// rather than silently overwriting or duplicating it.
type Info struct {
	ID          string
	Executor    string
	Provider    string
	DisplayName string
}

// Registry is an in-memory catalog of Info entries, keyed by ID.
type Registry struct {
	models map[string]Info
}

// NewRegistry returns an empty Registry, ready for Register calls.
func NewRegistry() *Registry {
	return &Registry{models: make(map[string]Info)}
}

// Register adds info to the Registry. It refuses an empty ID, an empty
// Executor, or a duplicate ID with a named error rather than silently
// accepting incomplete metadata or overwriting an existing entry —
// mirroring engine.ExecutorRegistry.Register's own duplicate-refusal
// pattern.
func (r *Registry) Register(info Info) error {
	if info.ID == "" {
		return fmt.Errorf("model: registry: model has no ID")
	}
	if info.Executor == "" {
		return fmt.Errorf("model: registry: model %q has no executor", info.ID)
	}
	if _, exists := r.models[info.ID]; exists {
		return fmt.Errorf("model: registry: model %q already registered", info.ID)
	}
	r.models[info.ID] = info
	return nil
}

// Get returns the registered Info for id, or a named error if id was
// never registered.
func (r *Registry) Get(id string) (Info, error) {
	info, ok := r.models[id]
	if !ok {
		return Info{}, fmt.Errorf("model: registry: model %q not registered", id)
	}
	return info, nil
}

// List returns every registered Info, sorted by ID for deterministic
// output — a future caller (e.g. a listing command) needs stable
// ordering, not Go's randomized map iteration order.
func (r *Registry) List() []Info {
	out := make([]Info, 0, len(r.models))
	for _, info := range r.models {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByExecutor returns every registered Info whose Executor equals executor,
// sorted by ID. An unknown or unregistered executor returns an empty
// slice, not an error — this mirrors List's "just query the data" shape
// rather than treating an empty result as exceptional.
func (r *Registry) ByExecutor(executor string) []Info {
	var out []Info
	for _, info := range r.models {
		if info.Executor == executor {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
