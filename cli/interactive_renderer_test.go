package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"foundry/cli"
)

func TestInteractiveRenderer_Banner(t *testing.T) {
	var out bytes.Buffer
	cli.NewInteractiveRenderer(&out).Banner("/path/to/project")

	if !strings.Contains(out.String(), "/path/to/project") {
		t.Errorf("output = %q, want it to name the project root", out.String())
	}
}

func TestInteractiveRenderer_Prompt(t *testing.T) {
	var out bytes.Buffer
	cli.NewInteractiveRenderer(&out).Prompt()

	if strings.Contains(out.String(), "\n") {
		t.Errorf("Prompt() wrote %q, want no trailing newline", out.String())
	}
	if out.Len() == 0 {
		t.Error("Prompt() wrote nothing")
	}
}

func TestInteractiveRenderer_Info(t *testing.T) {
	var out bytes.Buffer
	cli.NewInteractiveRenderer(&out).Info("initialized .foundry/pipelines")

	if !strings.Contains(out.String(), "initialized .foundry/pipelines") {
		t.Errorf("output = %q, want it to contain the message", out.String())
	}
}

func TestInteractiveRenderer_Error(t *testing.T) {
	var out bytes.Buffer
	cli.NewInteractiveRenderer(&out).Error(errors.New("unknown command \"bogus\""))

	if !strings.Contains(out.String(), "unknown command \"bogus\"") {
		t.Errorf("output = %q, want it to contain the error message", out.String())
	}
}
