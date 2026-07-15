package engine

import (
	"encoding/json"
	"fmt"

	"foundry/domain"
)

// PipelineDocument is the declarative, wire-format shape a Pipeline is
// authored as, decoded by DecodePipelineDocument into the engine.Pipeline
// PipelineStrategy actually executes. It is deliberately a distinct type
// from Pipeline, not a JSON-tagged Pipeline itself: the document's schema
// and its own versioning belong to an as-yet-unwritten ADR ("Reusable-Act
// template format & evolution policy", docs/03-adrs/README.md backlog).
// Reusing Pipeline's Go field names and tags as the wire format would
// silently pre-decide that ADR's question; keeping the two shapes
// separate, joined by one explicit translation, is what lets the document
// evolve independently of the runtime type later
// (docs/01-rfcs/RFC-0002-pipeline-execution-runtime.md §6).
//
// JSON is the only decodable form today — the canonical form RFC-0002 §6
// recommends even for the eventual YAML-authored end state ("YAML source,
// canonical JSON"). Nothing here discovers, walks, or reads a document
// from the filesystem or network; a document is a []byte a caller already
// has. Today that caller is always BuiltinProvider, embedding the one
// built-in document (builtin_provider.go); a future filesystem or remote
// PipelineProvider would read its own bytes and call the same decoder.
type PipelineDocument struct {
	Name   string               `json:"name"`
	Steps  []StepDocument       `json:"steps"`
	Repair RepairPolicyDocument `json:"repair"`
}

// StepDocument is one Step's declarative shape within a PipelineDocument.
// Capability, Executor, and FeedsForward are optional (RFC-0004 §2, Piece 1
// of docs/04-guides/multi-executor-router-implementation-plan.md): a
// document that omits them decodes to Step's zero values for all three,
// identical to every document written before they existed.
type StepDocument struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Capability   map[string]string `json:"capability,omitempty"`
	Executor     string            `json:"executor,omitempty"`
	FeedsForward bool              `json:"feeds_forward,omitempty"`
}

// RepairPolicyDocument is a Pipeline's declarative repair bound. A document
// that omits "repair" decodes to the zero value (MaxAttempts: 0, Target: ""
// — no repair, restart from the top), matching RepairPolicy's own zero-value
// default.
type RepairPolicyDocument struct {
	MaxAttempts int    `json:"max_attempts"`
	Target      string `json:"target"`
}

// DecodePipelineDocument parses data as a PipelineDocument and translates
// it into an engine.Pipeline. It is the loader RFC-0002 §9 Phase 3 calls
// for: the one place a declarative document becomes the data
// PipelineStrategy walks. Decode validates the document's own shape —
// required fields present, each Step's Kind one of RFC-0002 §4.2's closed
// five-kind vocabulary (domain.StepKindGenerate, domain.StepKindVerify,
// domain.StepKindApprove, domain.StepKindApply, domain.StepKindRecord),
// RepairPolicy.MaxAttempts non-negative, and a non-empty
// RepairPolicy.Target naming a Step ID declared somewhere in the same
// document — and returns a clear, named error for the first violation
// found, rather than handing PipelineStrategy a Pipeline it would only fail
// on much later, at execution time.
func DecodePipelineDocument(data []byte) (Pipeline, error) {
	var doc PipelineDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Pipeline{}, fmt.Errorf("engine: decode pipeline document: %w", err)
	}
	return doc.toPipeline()
}

// toPipeline validates doc and translates it into the Pipeline it
// declares.
func (doc PipelineDocument) toPipeline() (Pipeline, error) {
	if doc.Name == "" {
		return Pipeline{}, fmt.Errorf("engine: pipeline document: name is required")
	}
	if len(doc.Steps) == 0 {
		return Pipeline{}, fmt.Errorf("engine: pipeline document %q: at least one step is required", doc.Name)
	}
	if doc.Repair.MaxAttempts < 0 {
		return Pipeline{}, fmt.Errorf("engine: pipeline document %q: repair.max_attempts must not be negative, got %d", doc.Name, doc.Repair.MaxAttempts)
	}

	steps := make([]Step, len(doc.Steps))
	ids := make(map[string]bool, len(doc.Steps))
	for i, s := range doc.Steps {
		if s.ID == "" {
			return Pipeline{}, fmt.Errorf("engine: pipeline document %q: step %d: id is required", doc.Name, i)
		}
		switch s.Kind {
		case domain.StepKindGenerate, domain.StepKindVerify, domain.StepKindApprove, domain.StepKindApply, domain.StepKindRecord:
		default:
			return Pipeline{}, fmt.Errorf("engine: pipeline document %q: step %q: unrecognized kind %q", doc.Name, s.ID, s.Kind)
		}
		steps[i] = Step{
			ID:           s.ID,
			Kind:         s.Kind,
			Capability:   s.Capability,
			Executor:     s.Executor,
			FeedsForward: s.FeedsForward,
		}
		ids[s.ID] = true
	}
	if doc.Repair.Target != "" && !ids[doc.Repair.Target] {
		return Pipeline{}, fmt.Errorf("engine: pipeline document %q: repair.target %q does not name any declared step", doc.Name, doc.Repair.Target)
	}

	return Pipeline{
		Name:  doc.Name,
		Steps: steps,
		Repair: RepairPolicy{
			MaxAttempts: doc.Repair.MaxAttempts,
			Target:      doc.Repair.Target,
		},
	}, nil
}
