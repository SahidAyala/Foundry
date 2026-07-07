package cli

import (
	"io"
	"os"
	"strings"
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
