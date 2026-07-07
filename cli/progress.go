package cli

import (
	"fmt"
	"io"

	"foundry/engine"
)

// ProgressReporter narrates an Act's lifecycle to out as the Engine runs it:
// gathering, each Execute/Verify round, and repair. It satisfies
// engine.Reporter, which is telemetry-only — a ProgressReporter only
// describes what the Engine already decided, never influences it (I1).
type ProgressReporter struct {
	out   io.Writer
	color bool
}

// NewProgressReporter returns a Reporter that writes human-readable progress
// lines to out, colored when out is an interactive terminal.
func NewProgressReporter(out io.Writer) *ProgressReporter {
	return &ProgressReporter{out: out, color: colorEnabled(out)}
}

var _ engine.Reporter = (*ProgressReporter)(nil)

func (p *ProgressReporter) Gathering() {
	p.line(ansiCyan, "→ Gathering repository context...")
}

func (p *ProgressReporter) Executing(iteration int) {
	label := "Asking Claude Code for a patch..."
	if iteration > 1 {
		label = fmt.Sprintf("Asking Claude Code to repair the failed attempt (round %d)...", iteration)
	}
	p.line(ansiCyan, "→ "+label)
}

func (p *ProgressReporter) Verifying(iteration int) {
	p.line(ansiCyan, "→ Verifying the proposed patch...")
}

func (p *ProgressReporter) Verified(iteration int, verdict string) {
	fmt.Fprintf(p.out, "  %s\n", renderVerdict(verdict, p.color))
}

func (p *ProgressReporter) Repairing() {
	p.line(ansiYellow, "↻ Verification failed — attempting one bounded repair...")
}

func (p *ProgressReporter) RepairSkipped(reason string) {
	p.line(ansiRed, "✗ Repair skipped: "+reason)
}

func (p *ProgressReporter) BudgetExceeded(reason string) {
	p.line(ansiRed, "✗ Budget exceeded: "+reason)
}

// line writes s to out, tinted with code when color is enabled.
func (p *ProgressReporter) line(code, s string) {
	if p.color {
		s = code + s + ansiReset
	}
	fmt.Fprintln(p.out, s)
}
