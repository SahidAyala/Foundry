package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

var (
	bannerTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	bannerDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bannerReadyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	bannerNotYetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	bannerBoxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
)

const (
	bannerReadyText  = "Initialized"
	bannerNotYetText = "Not initialized — type /init to get started"
)

// Banner writes a one-time startup panel naming the project root a
// session is running against, this build's real version identity
// (BuildVersion — never a fabricated semantic version, ADR-0012), and
// whether /init has already scaffolded this project (initialized) —
// real, disk-derived facts, not decorative chrome. Piped or redirected
// output (r.color false, e.g. every existing test) gets the same three
// facts as plain lines, with no border or ANSI codes at all — the same
// "plain when non-interactive" rule renderDiff/renderVerdict already
// follow.
func (r *InteractiveRenderer) Banner(root string, initialized bool) {
	status := bannerNotYetText
	if initialized {
		status = bannerReadyText
	}

	if !r.color {
		fmt.Fprintf(r.out, "Foundry %s\n%s\n%s\n", BuildVersion(), root, status)
		return
	}

	title := bannerTitleStyle.Render("⚒ Foundry") + "  " + bannerDimStyle.Render(BuildVersion())
	path := bannerDimStyle.Render(root)
	statusStyle := bannerReadyStyle
	if !initialized {
		statusStyle = bannerNotYetStyle
	}
	body := strings.Join([]string{title, path, statusStyle.Render(status)}, "\n")
	fmt.Fprintln(r.out, bannerBoxStyle.Render(body))
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
