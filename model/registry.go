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
//
// Capabilities, Limits, and Quality are pure catalog metadata (ADR-0013,
// Proposed, third increment): they describe a model, nothing more.
// Nothing in Foundry reads them yet — no routing decision, no validation,
// no execution-path behavior consults these fields today. They exist so a
// future, separately-decided PR (e.g. capability-aware routing, once
// ADR-0006's own named trigger fires) has real data to build from, the
// same "expose now, consume later" shape ADR-0013's original Executor/
// Model split already established. Every field defaults to its zero value
// (false, 0) when omitted — an Info literal written before these fields
// existed decodes and behaves identically.
type Info struct {
	ID          string
	Executor    string
	Provider    string
	DisplayName string

	Capabilities Capabilities
	Limits       Limits
	Quality      Quality
}

// Capabilities records which named abilities a model is known to support.
// Hand-curated per model, the same "will drift as vendors ship new
// versions" caveat SupportedModels()'s own price-table-style entries
// already carry — not fetched from any vendor's live capability-listing
// API. Zero value (all false) means "not confirmed to support this,"
// never "confirmed not to."
type Capabilities struct {
	// ToolUse: the model can call caller-provided functions/tools
	// (function calling).
	ToolUse bool
	// Thinking: the model has an extended, distinct reasoning/"thinking"
	// mode, separate from its default response generation.
	Thinking bool
	// Streaming: the model's Executor can receive its response
	// incrementally rather than only as one complete response.
	Streaming bool
	// Multimodal: the model accepts non-text input (e.g. images).
	Multimodal bool
	// StructuredOutput: the model can be constrained to emit a specific
	// output shape (e.g. JSON mode / a declared schema).
	StructuredOutput bool
}

// Limits records a model's known operating limits.
type Limits struct {
	// MaxContext is the model's documented context window, in tokens.
	// Zero means "not recorded," never "no limit."
	MaxContext int
}

// Quality is a hand-assigned, relative rating on a fixed 1 (weakest) to 5
// (strongest) scale for each named dimension — a coarse, subjective
// judgment call recorded once per model, not a benchmark score, and not
// comparable across a model's own Provider's other product lines with any
// precision. Zero means "not rated," never "lowest quality."
type Quality struct {
	Reasoning int
	Coding    int
	Review    int
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
