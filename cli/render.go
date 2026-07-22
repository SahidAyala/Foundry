package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"foundry/replay"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// colorEnabled reports whether out is an interactive terminal (a character
// device) — the only case where ANSI codes are emitted, so piped and test
// output stays plain.
func colorEnabled(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// renderVerdict formats a machine verdict for human review: "✓ pass" for a
// pass, "✗ <verdict>" for anything else, tinted when color is true.
func renderVerdict(verdict string, color bool) string {
	symbol, tint := "✗", ansiRed
	if verdict == "pass" {
		symbol, tint = "✓", ansiGreen
	}
	s := symbol + " " + verdict
	if !color {
		return s
	}
	return tint + s + ansiReset
}

// renderDiff formats a unified diff for review: file headers bold, hunk
// headers cyan, additions green, deletions red — when color is true; the
// patch is returned verbatim otherwise.
func renderDiff(patch string, color bool) string {
	if !color {
		return patch
	}

	lines := strings.Split(patch, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"),
			strings.HasPrefix(line, "+++"),
			strings.HasPrefix(line, "---"):
			lines[i] = ansiBold + line + ansiReset
		case strings.HasPrefix(line, "@@"):
			lines[i] = ansiCyan + line + ansiReset
		case strings.HasPrefix(line, "+"):
			lines[i] = ansiGreen + line + ansiReset
		case strings.HasPrefix(line, "-"):
			lines[i] = ansiRed + line + ansiReset
		}
	}
	return strings.Join(lines, "\n")
}

// renderStepVerdictLabel renders one StepRecord's own verdict for the
// per-Step trace formatAct prints: a verify Step's pass/fail through
// renderVerdict (identical tinting to the live approval prompt), an
// approve Step's accept/reject through a matching tick/cross (renderVerdict
// itself would mis-tint "accept" as a failure, since it only special-cases
// the literal string "pass"), and no label at all for a generate/apply/
// record Step, which carries no verdict.
func renderStepVerdictLabel(kind, verdict string, color bool) string {
	switch kind {
	case "verify":
		if verdict == "" {
			return ""
		}
		return renderVerdict(verdict, color)
	case "approve":
		symbol, tint := "✗", ansiRed
		if verdict == "accept" {
			symbol, tint = "✓", ansiGreen
		}
		s := symbol + " " + verdict
		if !color {
			return s
		}
		return tint + s + ansiReset
	default:
		return ""
	}
}

// formatReplayResult renders a replay.Result for human review: one line per
// verify Step comparing its recorded and replayed verdict, then an overall
// reproduced/diverged summary. This is a same-version replay report
// (replay/replay.go's package doc) — it says nothing about a future Engine
// version.
func formatReplayResult(result replay.Result, color bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Act:     %s\n", result.ActID)

	if len(result.Steps) == 0 {
		b.WriteString("No verify Steps recorded for this Act — nothing to replay.\n")
		return b.String()
	}

	b.WriteString("\nVerify Steps:\n")
	for _, step := range result.Steps {
		mark := "✓ reproduced"
		tint := ansiGreen
		if !step.Reproduced {
			mark = "✗ diverged"
			tint = ansiRed
		}
		if color {
			mark = tint + mark + ansiReset
		}
		fmt.Fprintf(&b, "  %-8s %s  (recorded: %s, replayed: %s)\n",
			step.StepID, mark, step.RecordedVerdict, step.ReplayedVerdict)
	}

	b.WriteString("\n")
	if result.Reproduced() {
		fmt.Fprintln(&b, renderVerdict("pass", color)+" — verification reproduced under this Engine build")
	} else {
		fmt.Fprintln(&b, renderVerdict("fail", color)+" — verification diverged from the recorded Act; see Verify Steps above")
	}
	return b.String()
}
