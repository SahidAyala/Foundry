package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrPromptEOF is ReadInteractiveLine's signal that the user ended input
// at the prompt (Ctrl-C or Ctrl-D) rather than submitting a line — the
// same "stop the session" meaning session/repl.go's Run already gives a
// plain io.EOF from its non-interactive fallback path.
var ErrPromptEOF = errors.New("cli: interactive prompt ended")

// CommandCandidate is one slash command ReadInteractiveLine can offer in
// its "/"-triggered dropdown menu: its full "/name" form (including the
// leading slash) and one-line description, both shown side by side in
// the menu exactly as /help already lists them (session.CommandInfo).
type CommandCandidate struct {
	Name        string
	Description string
}

// PromptHistory holds the lines a REPL has already submitted, most-recent-
// last, so ReadInteractiveLine can offer arrow-key recall (ADR-0012
// Decision 3). NewPromptHistory's history is in-memory only, scoped to
// one process; LoadPromptHistory's also persists to disk, so a later
// session's arrow-key recall survives this process ending.
type PromptHistory struct {
	entries []string
	path    string // "" means in-memory only, no persistence
}

// NewPromptHistory returns an empty, in-memory-only PromptHistory.
func NewPromptHistory() *PromptHistory {
	return &PromptHistory{}
}

// LoadPromptHistory returns a PromptHistory pre-populated from path's
// existing lines (one submitted line per line, oldest first), and
// remembers path so Add appends every new line back to it — ADR-0012's
// own v1 Decision 3 named persistent cross-session history as a later
// increment; this is that increment. A missing file, or one this process
// cannot read, is not fatal: history is a convenience that must never
// block a session from starting, so both cases just return an empty,
// path-remembering history instead of an error.
func LoadPromptHistory(path string) *PromptHistory {
	h := &PromptHistory{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			h.entries = append(h.entries, line)
		}
	}
	return h
}

// Add appends line to the history, in submission order. A blank line (or
// one that is only whitespace) is not recorded — there is nothing useful
// to recall from it. If this PromptHistory was built via
// LoadPromptHistory, line is also appended to its file immediately
// (append-only, never rewritten wholesale, so a crash mid-session loses
// at most nothing already flushed) — best-effort: a write failure (a
// missing/unwritable directory, a full disk) is silently ignored rather
// than surfaced, since losing history persistence must never break the
// interactive session itself over a purely cosmetic convenience.
func (h *PromptHistory) Add(line string) {
	if h == nil || strings.TrimSpace(line) == "" {
		return
	}
	h.entries = append(h.entries, line)
	if h.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

// at returns the entry i steps back from the most recent submission (0
// is the most recent) and whether that index exists.
func (h *PromptHistory) at(i int) (string, bool) {
	if h == nil || i < 0 || i >= len(h.entries) {
		return "", false
	}
	return h.entries[len(h.entries)-1-i], true
}

// ReadInteractiveLine collects one line of input from a real terminal via
// bubbletea's raw-mode line editor: a full, arrow-navigable dropdown menu
// of every candidate whose name has the current input as a prefix —
// opening the moment the line starts with "/", filtering as the user
// keeps typing, closing once the line no longer looks like a bare command
// name (a space, or no longer a "/" prefix) — plus Up/Down arrow-key
// recall through history when the menu isn't showing. It owns the
// terminal only for the duration of one line — bubbletea restores cooked
// mode before Run returns — so a caller's own subsequent blocking reads
// (e.g. an approval prompt reading from the same *bufio.Reader over
// os.Stdin) behave exactly as they did before this existed.
//
// Returns ErrPromptEOF on Ctrl-C or Ctrl-D instead of the submitted line.
func ReadInteractiveLine(prompt string, candidates []CommandCandidate, history *PromptHistory) (string, error) {
	m := newPromptModel(prompt, candidates, history)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("cli: interactive prompt: %w", err)
	}

	fm := final.(promptModel)
	if fm.eof {
		return "", ErrPromptEOF
	}
	history.Add(fm.submitted)
	return fm.submitted, nil
}

var (
	menuRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	menuSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237")).Bold(true)
)

// promptModel is the bubbletea model backing ReadInteractiveLine: a
// bubbles/textinput field for the line itself, this session's own
// Up/Down history browsing, and a "/"-triggered dropdown menu over every
// registered slash command. Menu navigation and history browsing share
// the Up/Down keys — whichever is active at the moment intercepts them
// before textinput's own Update ever sees the keystroke, since
// textinput's default KeyMap binds Up/Down to its own (single-line,
// non-full-list) suggestion cycling, which this dropdown replaces
// entirely rather than competing with.
type promptModel struct {
	input      textinput.Model
	history    *PromptHistory
	historyIdx int // -1 = editing the live draft, not browsing history
	draft      string
	submitted  string
	eof        bool

	menu       []CommandCandidate // every offerable candidate, unfiltered
	filtered   []CommandCandidate // menu, narrowed to the current input's prefix
	menuCursor int
	menuOpen   bool
	nameWidth  int // longest candidate Name, for the menu's column alignment
}

func newPromptModel(prompt string, candidates []CommandCandidate, history *PromptHistory) promptModel {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.Focus()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	nameWidth := 0
	for _, c := range candidates {
		if len(c.Name) > nameWidth {
			nameWidth = len(c.Name)
		}
	}

	return promptModel{input: ti, history: history, historyIdx: -1, menu: candidates, nameWidth: nameWidth}
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.eof = true
			return m, tea.Quit
		case tea.KeyEsc:
			if m.menuOpen {
				m.menuOpen = false
				return m, nil
			}
		case tea.KeyEnter:
			if m.menuOpen {
				m.acceptMenuSelection()
				return m, nil
			}
			m.submitted = m.input.Value()
			return m, tea.Quit
		case tea.KeyTab:
			if m.menuOpen {
				m.acceptMenuSelection()
				return m, nil
			}
		case tea.KeyUp:
			if m.menuOpen {
				m.moveMenuCursor(-1)
			} else {
				m.browseHistory(1)
			}
			return m, nil
		case tea.KeyDown:
			if m.menuOpen {
				m.moveMenuCursor(1)
			} else {
				m.browseHistory(-1)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refreshMenu()
	return m, cmd
}

// refreshMenu recomputes which candidates the current input matches,
// called after every keystroke that reaches textinput itself (typing,
// deleting, pasting). The menu shows only while the line still looks like
// a bare command name being typed: it must start with "/" and must not
// yet contain a space — the moment the user is past the command name
// into its arguments, or has erased the leading "/", the menu closes.
func (m *promptModel) refreshMenu() {
	value := m.input.Value()
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, " \t") {
		m.menuOpen = false
		m.filtered = nil
		return
	}

	m.filtered = m.filtered[:0]
	for _, c := range m.menu {
		if strings.HasPrefix(c.Name, value) {
			m.filtered = append(m.filtered, c)
		}
	}

	// Once the line already spells out a candidate's name exactly, there
	// is nothing left to complete or disambiguate — close the menu so a
	// single Enter submits the line immediately. Without this, finishing
	// a command by typing it out in full (rather than picking it from the
	// menu) would still need two Enters: one to "accept" a selection that
	// was already spelled out, and only the second to actually submit.
	for _, c := range m.filtered {
		if c.Name == value {
			m.menuOpen = false
			m.filtered = nil
			return
		}
	}

	m.menuOpen = len(m.filtered) > 0
	if m.menuCursor >= len(m.filtered) {
		m.menuCursor = 0
	}
}

// acceptMenuSelection completes the input to the highlighted candidate's
// full name plus a trailing space — ready for the user to keep typing
// its arguments — and closes the menu, mirroring how Claude Code's own
// "/" command menu behaves on Tab or Enter.
func (m *promptModel) acceptMenuSelection() {
	if len(m.filtered) == 0 {
		m.menuOpen = false
		return
	}
	m.input.SetValue(m.filtered[m.menuCursor].Name + " ")
	m.input.CursorEnd()
	m.menuOpen = false
	m.filtered = nil
}

// moveMenuCursor moves delta steps through the filtered candidate list,
// wrapping at either end.
func (m *promptModel) moveMenuCursor(delta int) {
	n := len(m.filtered)
	if n == 0 {
		return
	}
	m.menuCursor = ((m.menuCursor+delta)%n + n) % n
}

// browseHistory moves delta steps through history (positive = older,
// negative = newer), saving the in-progress draft the first time history
// browsing starts so returning past the newest entry restores it exactly,
// the same behavior a shell's own history recall gives.
func (m *promptModel) browseHistory(delta int) {
	if m.historyIdx == -1 {
		if delta < 0 {
			return // already at the live draft; nothing newer to go to
		}
		m.draft = m.input.Value()
	}

	next := m.historyIdx + delta
	if next == -1 {
		m.input.SetValue(m.draft)
		m.historyIdx = -1
		m.input.CursorEnd()
		return
	}
	line, ok := m.history.at(next)
	if !ok {
		return
	}
	m.historyIdx = next
	m.input.SetValue(line)
	m.input.CursorEnd()
}

func (m promptModel) View() string {
	if !m.menuOpen || len(m.filtered) == 0 {
		return m.input.View()
	}

	var b strings.Builder
	b.WriteString(m.input.View())
	for i, c := range m.filtered {
		b.WriteByte('\n')
		row := fmt.Sprintf("%-*s  %s", m.nameWidth, c.Name, c.Description)
		if i == m.menuCursor {
			b.WriteString(menuSelectedStyle.Render("▸ " + row))
		} else {
			b.WriteString(menuRowStyle.Render("  " + row))
		}
	}
	return b.String()
}
