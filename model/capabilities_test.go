package model_test

import (
	"reflect"
	"testing"

	"github.com/SahidAyala/Foundry/model"
)

func TestCapabilities_Supports_AllSatisfied(t *testing.T) {
	c := model.Capabilities{ToolUse: true, Thinking: true, StructuredOutput: true}

	ok, missing := c.Supports([]string{"tool_use", "thinking", "structured_output"})
	if !ok {
		t.Errorf("ok = false, want true; missing = %v", missing)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
}

func TestCapabilities_Supports_MissingOneReportsIt(t *testing.T) {
	c := model.Capabilities{ToolUse: true, Thinking: true, StructuredOutput: false}

	ok, missing := c.Supports([]string{"tool_use", "thinking", "structured_output"})
	if ok {
		t.Error("ok = true, want false (structured_output is unsupported)")
	}
	if !reflect.DeepEqual(missing, []string{"structured_output"}) {
		t.Errorf("missing = %v, want [structured_output]", missing)
	}
}

func TestCapabilities_Supports_UnrecognizedNameCountsAsMissing(t *testing.T) {
	c := model.Capabilities{ToolUse: true, Thinking: true, StructuredOutput: true}

	ok, missing := c.Supports([]string{"tool_use", "telepathy"})
	if ok {
		t.Error("ok = true, want false (an unrecognized capability name must never pass vacuously)")
	}
	if !reflect.DeepEqual(missing, []string{"telepathy"}) {
		t.Errorf("missing = %v, want [telepathy]", missing)
	}
}

func TestCapabilities_Supports_EmptyRequiredAlwaysSatisfied(t *testing.T) {
	ok, missing := model.Capabilities{}.Supports(nil)
	if !ok {
		t.Error("ok = false, want true for an empty requirement list")
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
}
