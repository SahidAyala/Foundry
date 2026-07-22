package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"foundry/domain"
	"foundry/engine"
)

// PromptForApproval shows act's proposed patch and machine verdict on out,
// then reads a yes/no decision from in. On acceptance it returns the
// accountable Authority (the current user) and approved=true; otherwise it
// returns approved=false. This is the trust boundary: nothing is applied or
// recorded unless an Authority accepts.
//
// A patch longer than pagerThresholdLines is piped through $PAGER (see
// maybePage) when out is an interactive terminal, so a large diff doesn't
// scroll past what a human can review before the y/n prompt appears —
// exactly the review surface this trust boundary depends on actually being
// readable.
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
	if err := maybePage(out, color, "\nProposed patch:\n"+renderDiff(act.Patch, color)+"\n"); err != nil {
		return "", false, fmt.Errorf("cli: write patch: %w", err)
	}
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

// InteractiveAuthority implements engine.Authority by prompting a human on
// Out and reading their decision from In — the same PromptForApproval CLI.Do
// already used after Engine.Run returned, now callable from inside a
// Pipeline run so an approve Step can seek approval mid-run instead of only
// at the very end (RFC-0002 §9 Phase 4).
type InteractiveAuthority struct {
	In  io.Reader
	Out io.Writer
}

func (a InteractiveAuthority) Decide(ctx context.Context, act *domain.Act) (string, bool, error) {
	return PromptForApproval(a.In, a.Out, act)
}

var _ engine.Authority = InteractiveAuthority{}

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
