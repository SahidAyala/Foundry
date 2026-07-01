// Package verify runs deterministic checks against an Outcome and renders a verdict.
package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Result is the outcome of running a single Validator.
type Result struct {
	Name   string
	Passed bool
	Output string
}

// Validator is one check (e.g. "run tests").
type Validator struct {
	Name string
	Cmd  string // shell command to run
}

// Run executes the validator's command in workspace. A non-zero exit code
// fails the validator without returning an error; an error is returned only
// if the command could not be run at all (e.g. invalid workspace).
func (v *Validator) Run(ctx context.Context, workspace string) (*Result, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", v.Cmd)
	cmd.Dir = workspace

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, fmt.Errorf("verify: run validator %q: %w", v.Name, err)
	}

	return &Result{
		Name:   v.Name,
		Passed: err == nil,
		Output: output.String(),
	}, nil
}
