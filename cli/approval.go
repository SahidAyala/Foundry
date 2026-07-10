package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"foundry/domain"
)

// PromptForApproval shows act's proposed patch and machine verdict on out,
// then reads a yes/no decision from in. On acceptance it returns the
// accountable Authority (the current user) and approved=true; otherwise it
// returns approved=false. This is the trust boundary: nothing is applied or
// recorded unless an Authority accepts.
//
// If in is already a *bufio.Reader, PromptForApproval reads directly from
// it instead of wrapping it in a new bufio.Reader. This matters the
// moment a caller issues more than one approval prompt over the same
// underlying stream (an interactive session, one prompt per slash
// command): bufio.NewReader's first Read greedily drains everything
// currently available from the underlying reader into its own buffer,
// so wrapping anew on every call silently discards every byte after the
// first line — the caller's *next* read (whether the next approval
// prompt or the next REPL line) would see EOF even though more input was
// typed. Wrapping only once, by the one caller who owns the stream for
// its whole lifetime, avoids that. The one-shot CLI (which calls this
// exactly once per process) is unaffected either way.
func PromptForApproval(in io.Reader, out io.Writer, act *domain.Act) (authority string, approved bool, err error) {
	color := colorEnabled(out)
	fmt.Fprintf(out, "\nProposed patch:\n%s\n", renderDiff(act.Patch, color))
	fmt.Fprintf(out, "Verdict: %s\n", renderVerdict(act.JudgmentVerdict, color))
	fmt.Fprint(out, "Approve and apply? (y/n): ")

	reader, ok := in.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(in)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, fmt.Errorf("cli: read approval: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return currentUser(), true, nil
	default:
		return "", false, nil
	}
}

// currentUser identifies the accountable Authority: the USER environment
// variable, falling back to the `whoami` command.
func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if out, err := exec.Command("whoami").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	return "unknown"
}
