package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"foundry/domain"
)

// PipelineDocument is the declarative, wire-format shape a Pipeline is
// authored as, decoded by DecodePipelineDocument into the engine.Pipeline
// PipelineStrategy actually executes. It is deliberately a distinct type
// from Pipeline, not a JSON-tagged Pipeline itself: ADR-0004
// ("Reusable-Act template format & evolution policy") ratifies this
// separation as the permanent mechanism that lets the document evolve
// independently of the runtime type — new fields must be optional and
// omitempty (ADR-0004 Decision 3); there is no version field yet, by the
// same decision, since no second reader of this format exists.
// Reusing Pipeline's Go field names and tags as the wire format would have
// pre-decided that question implicitly instead of deliberately
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
// Capability, Executor, FeedsForward, and Target are optional (RFC-0004 §2
// and §2.6, Pieces 1 and 4 of
// docs/04-guides/multi-executor-router-implementation-plan.md): a document
// that omits them decodes to Step's zero values for all four, identical to
// every document written before they existed.
type StepDocument struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Capability   map[string]string `json:"capability,omitempty"`
	Executor     string            `json:"executor,omitempty"`
	FeedsForward bool              `json:"feeds_forward,omitempty"`
	Target       string            `json:"target,omitempty"`
}

// RepairPolicyDocument is a Pipeline's declarative repair bound. A document
// that omits "repair" decodes to the zero value (MaxAttempts: 0, Target: ""
// — no repair, restart from the top), matching RepairPolicy's own zero-value
// default.
type RepairPolicyDocument struct {
	MaxAttempts int    `json:"max_attempts"`
	Target      string `json:"target"`
}

// pipelineSchemaDocRef is where DecodePipelineDocument points an author who
// hits an unknown-field error (ADR-0004 Decision 4) to fix it against.
const pipelineSchemaDocRef = "docs/04-guides/pipelines.md"

// unknownFieldPattern extracts the field name from encoding/json's own
// DisallowUnknownFields error ("json: unknown field \"x\""), which
// otherwise says nothing about which Step or section it occurred in.
var unknownFieldPattern = regexp.MustCompile(`unknown field "([^"]+)"`)

// DecodePipelineDocument parses data as a PipelineDocument and translates
// it into an engine.Pipeline. It is the loader RFC-0002 §9 Phase 3 calls
// for: the one place a declarative document becomes the data
// PipelineStrategy walks. Decode validates the document's own shape —
// required fields present, no unrecognized field anywhere in the document
// (ADR-0004 Decision 4 — a typo'd or stray key is a decode-time error, not
// silently dropped), each Step's Kind one of RFC-0002 §4.2's closed
// five-kind vocabulary (domain.StepKindGenerate, domain.StepKindVerify,
// domain.StepKindApprove, domain.StepKindApply, domain.StepKindRecord),
// RepairPolicy.MaxAttempts non-negative, and a non-empty
// RepairPolicy.Target naming a Step ID declared somewhere in the same
// document — and returns a clear, named error for the first violation
// found, rather than handing PipelineStrategy a Pipeline it would only fail
// on much later, at execution time.
func DecodePipelineDocument(data []byte) (Pipeline, error) {
	var doc PipelineDocument
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		if unknownFieldPattern.MatchString(err.Error()) {
			return Pipeline{}, describeUnknownField(data, err)
		}
		return Pipeline{}, fmt.Errorf("engine: decode pipeline document: %w", err)
	}
	return doc.toPipeline()
}

// describeUnknownField re-decodes data one level at a time — top-level,
// then each Step, then repair — to locate which section DisallowUnknownFields
// rejected, so the returned error names the field, the enclosing Step (by
// id, when there is one), and pipelineSchemaDocRef. Go's own error gives
// none of that context, only the bare field name.
func describeUnknownField(data []byte, cause error) error {
	field := "(unrecognized)"
	if m := unknownFieldPattern.FindStringSubmatch(cause.Error()); m != nil {
		field = fmt.Sprintf("%q", m[1])
	}

	var loose struct {
		Name   string            `json:"name"`
		Steps  []json.RawMessage `json:"steps"`
		Repair json.RawMessage   `json:"repair"`
	}
	if err := json.Unmarshal(data, &loose); err != nil {
		// Doesn't even parse leniently as a PipelineDocument shape; report
		// Go's own error rather than guessing further.
		return fmt.Errorf("engine: decode pipeline document: unknown field %s — see %s for the current schema: %w", field, pipelineSchemaDocRef, cause)
	}
	prefix := "engine: decode pipeline document:"
	if loose.Name != "" {
		prefix = fmt.Sprintf("engine: pipeline document %q:", loose.Name)
	}

	if strictlyRejects(data, &struct {
		Name   json.RawMessage `json:"name"`
		Steps  json.RawMessage `json:"steps"`
		Repair json.RawMessage `json:"repair"`
	}{}) {
		return fmt.Errorf("%s unknown top-level field %s — see %s for the current schema", prefix, field, pipelineSchemaDocRef)
	}

	for i, raw := range loose.Steps {
		if strictlyRejects(raw, &StepDocument{}) {
			return fmt.Errorf("%s step %s: unknown field %s — see %s for the current schema", prefix, stepLabel(raw, i), field, pipelineSchemaDocRef)
		}
	}

	if len(loose.Repair) > 0 && strictlyRejects(loose.Repair, &RepairPolicyDocument{}) {
		return fmt.Errorf("%s repair: unknown field %s — see %s for the current schema", prefix, field, pipelineSchemaDocRef)
	}

	return fmt.Errorf("%s unknown field %s — see %s for the current schema: %w", prefix, field, pipelineSchemaDocRef, cause)
}

// strictlyRejects reports whether decoding raw into v with unknown fields
// disallowed fails on an unknown field specifically.
func strictlyRejects(raw []byte, v any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	return err != nil && unknownFieldPattern.MatchString(err.Error())
}

// stepLabel identifies a Step for an error message: its declared id when
// present, or its position when the id itself is missing or unparsable.
func stepLabel(raw json.RawMessage, index int) string {
	var peek struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &peek); err == nil && peek.ID != "" {
		return fmt.Sprintf("%q", peek.ID)
	}
	return fmt.Sprintf("at index %d", index)
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
			Target:       s.Target,
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
