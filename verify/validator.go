// Package verify runs deterministic checks against an Outcome and renders a verdict.
package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// defaultValidatorTimeout bounds how long a single Validator command may
// run when Timeout is unset. M0 policy: hardcoded, matching Budget's
// hardcoded caps (engine/budget.go) — a validator is most often a build or
// test suite acting on Executor-produced code, and must never be allowed
// to hang the whole Act indefinitely (e.g. an AI-generated infinite loop).
// A timeout is reported as an ordinary failed Validator, feeding the
// repair loop exactly like any other failure — not a fatal error.
const defaultValidatorTimeout = 2 * time.Minute

// Result is the outcome of running a single Validator.
type Result struct {
	Name   string
	Passed bool
	Output string
}

// Validator is one check (e.g. "run tests").
type Validator struct {
	Name    string
	Cmd     string        // shell command to run
	Timeout time.Duration // zero uses defaultValidatorTimeout
}

// Run executes the validator's command in workspace. A non-zero exit code
// or a timeout fails the validator without returning an error; an error is
// returned only if the command could not be run at all (e.g. invalid
// workspace).
func (v *Validator) Run(ctx context.Context, workspace string) (*Result, error) {
	timeout := v.Timeout
	if timeout <= 0 {
		timeout = defaultValidatorTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", v.Cmd)
	cmd.Dir = workspace

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return &Result{
			Name:   v.Name,
			Passed: false,
			Output: fmt.Sprintf("timed out after %s", timeout),
		}, nil
	}

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
