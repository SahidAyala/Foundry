package cli

import (
	"fmt"
	"io"
	"strings"

	"foundry/domain"
	"foundry/engine"
)

// maxFindingLines bounds how many lines of a failed Judgment's Checked
// findings ProgressReporter prints live, so one verbose validator (a full
// compiler dump) cannot flood the terminal during a demo. The recorded Act
// always carries the findings in full (`foundry show`).
const maxFindingLines = 12

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

// Executed is intentionally silent: ADR-0011's actual-cost signal is
// reported Evidence for `foundry show`/FOUNDRY_LOG, not live human
// narration — ProgressReporter's own progress lines are unchanged.
func (p *ProgressReporter) Executed(iteration int, actualCostUSD *float64) {}

func (p *ProgressReporter) Verifying(iteration int) {
	p.line(ansiCyan, "→ Verifying the proposed patch...")
}

func (p *ProgressReporter) Verified(iteration int, judgment *domain.Judgment) {
	fmt.Fprintf(p.out, "  %s\n", renderVerdict(judgment.Verdict, p.color))
	if judgment.Verdict == "pass" {
		return
	}

	lines := findingLines(judgment.Checked)
	shown, remaining := lines, 0
	if len(lines) > maxFindingLines {
		shown, remaining = lines[:maxFindingLines], len(lines)-maxFindingLines
	}
	for _, line := range shown {
		fmt.Fprintf(p.out, "    %s\n", line)
	}
	if remaining > 0 {
		fmt.Fprintf(p.out, "    ... (%d more lines; see `foundry show` for the full findings)\n", remaining)
	}
}

// findingLines flattens a Judgment's Checked entries into individual lines,
// in order, for a compact live rendering.
func findingLines(checked []string) []string {
	var lines []string
	for _, c := range checked {
		lines = append(lines, strings.Split(strings.TrimRight(c, "\n"), "\n")...)
	}
	return lines
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
