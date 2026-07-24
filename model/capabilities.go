package model

// capabilityFlags maps the string names a Step's RequireCapabilities list
// uses (ADR-0013, Proposed, seventh increment) to the Capabilities field
// they correspond to. Names mirror Capabilities' own field names exactly
// (ADR-0013's third increment: "supports: tool_use, thinking, streaming,
// multimodal, structured_output").
var capabilityFlags = map[string]func(Capabilities) bool{
	"tool_use":          func(c Capabilities) bool { return c.ToolUse },
	"thinking":          func(c Capabilities) bool { return c.Thinking },
	"streaming":         func(c Capabilities) bool { return c.Streaming },
	"multimodal":        func(c Capabilities) bool { return c.Multimodal },
	"structured_output": func(c Capabilities) bool { return c.StructuredOutput },
}

// Supports reports whether c satisfies every named capability in
// required, and returns the subset of required that it does not (empty
// when ok is true). An unrecognized capability name is treated as
// unsatisfied — never silently ignored — so a typo'd requirement fails
// clearly rather than passing vacuously; it is included in missing like
// any other unsatisfied name.
func (c Capabilities) Supports(required []string) (ok bool, missing []string) {
	for _, name := range required {
		check, known := capabilityFlags[name]
		if !known || !check(c) {
			missing = append(missing, name)
		}
	}
	return len(missing) == 0, missing
}
