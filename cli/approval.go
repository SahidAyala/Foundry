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
func PromptForApproval(in io.Reader, out io.Writer, act *domain.Act) (authority string, approved bool, err error) {
	color := colorEnabled(out)
	fmt.Fprintf(out, "\nProposed patch:\n%s\n", renderDiff(act.Patch, color))
	fmt.Fprintf(out, "Verdict: %s\n", renderVerdict(act.JudgmentVerdict, color))
	fmt.Fprint(out, "Approve and apply? (y/n): ")

	line, err := bufio.NewReader(in).ReadString('\n')
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
