package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// pagerThresholdLines is how many lines of content trigger paging, so a
// short diff or findings dump — the common case — never launches a
// subprocess at all.
const pagerThresholdLines = 40

// pagerRunner runs a pager command, feeding it stdin (the content to page)
// and connecting its own stdout directly to out — the same "content on
// stdin, keyboard control from the terminal directly" shape every real
// pager (less, more) already expects when used as the receiving end of a
// pipe (`foo | less`). The real implementation requires out to be a
// terminal (*os.File): a pager needs direct terminal control it cannot get
// from an arbitrary io.Writer. Tests substitute a fake that only records
// what it was asked to run, so maybePage itself stays testable without a
// real terminal or subprocess — the same discipline renderDiff/
// renderVerdict already established by taking an explicit color bool
// instead of re-deriving it from an io.Writer internally.
var pagerRunner = func(name string, args []string, stdin string, out io.Writer) error {
	f, ok := out.(*os.File)
	if !ok {
		return fmt.Errorf("cli: pager requires a terminal, got %T", out)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = f
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// maybePage writes content to out directly, or — when page is true and
// content is longer than pagerThresholdLines — pipes it through $PAGER
// (falling back to "less -R" to preserve the ANSI color codes renderDiff
// already applies) so a long diff or findings dump doesn't scroll past a
// human's terminal before they can read it. Callers decide page exactly
// the way they already decide color (colorEnabled(out)) — this function
// never inspects out's concrete type itself for that decision. If the
// pager fails for any reason (not installed, non-zero exit, $PAGER set to
// something broken), content is still written directly afterward — a
// broken pager must never eat output the caller needs to see.
func maybePage(out io.Writer, page bool, content string) error {
	if !page || countLines(content) <= pagerThresholdLines {
		_, err := io.WriteString(out, content)
		return err
	}

	pagerCmd := os.Getenv("PAGER")
	if pagerCmd == "" {
		pagerCmd = "less -R"
	}
	parts := strings.Fields(pagerCmd)
	if len(parts) == 0 {
		_, err := io.WriteString(out, content)
		return err
	}

	if err := pagerRunner(parts[0], parts[1:], content, out); err != nil {
		_, err := io.WriteString(out, content)
		return err
	}
	return nil
}

// countLines returns how many lines s spans (a trailing newline does not
// count as a further, empty line).
func countLines(s string) int {
	return strings.Count(strings.TrimRight(s, "\n"), "\n") + 1
}
