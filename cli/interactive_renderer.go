package cli

import (
	"fmt"
	"io"
)

// InteractiveRenderer renders the REPL-level chrome around an
// interactive session — the prompt, a startup banner, and
// informational or error messages between slash commands. It is
// deliberately separate from ProgressReporter (progress.go), which
// narrates one Act's Engine-driven lifecycle: InteractiveRenderer never
// touches an Act, a Judgment, or engine.Reporter at all.
type InteractiveRenderer struct {
	out   io.Writer
	color bool
}

// NewInteractiveRenderer returns a renderer that writes to out, colored
// when out is an interactive terminal — the same detection
// ProgressReporter already uses.
func NewInteractiveRenderer(out io.Writer) *InteractiveRenderer {
	return &InteractiveRenderer{out: out, color: colorEnabled(out)}
}

// Banner writes a one-time startup message naming the project root a
// session is running against.
func (r *InteractiveRenderer) Banner(root string) {
	fmt.Fprintf(r.out, "foundry — interactive session in %s\n", root)
	fmt.Fprintln(r.out, "Type /init to get started, or /help for a list of commands.")
}

// Prompt writes the input prompt, without a trailing newline — the
// REPL's next read is expected to appear on the same line.
func (r *InteractiveRenderer) Prompt() {
	fmt.Fprint(r.out, "foundry> ")
}

// Info writes a plain informational message, one line.
func (r *InteractiveRenderer) Info(msg string) {
	r.line(ansiCyan, msg)
}

// Error writes err as a single line, tinted red when color is enabled.
func (r *InteractiveRenderer) Error(err error) {
	r.line(ansiRed, "✗ "+err.Error())
}

func (r *InteractiveRenderer) line(code, s string) {
	if r.color {
		s = code + s + ansiReset
	}
	fmt.Fprintln(r.out, s)
}
