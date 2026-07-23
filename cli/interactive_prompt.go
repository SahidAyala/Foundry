package cli

import (
	"errors"
	"fmt"
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

// PromptHistory holds the lines a REPL has already submitted during one
// process's lifetime, most-recent-last, so ReadInteractiveLine can offer
// arrow-key recall (ADR-0012 Decision 3, v1 scope). It is not persisted
// across process runs — cross-session history is explicitly a later
// increment, not this one.
type PromptHistory struct {
	entries []string
}

// NewPromptHistory returns an empty PromptHistory.
func NewPromptHistory() *PromptHistory {
	return &PromptHistory{}
}

// Add appends line to the history, in submission order. A blank line (or
// one that is only whitespace) is not recorded — there is nothing useful
// to recall from it.
func (h *PromptHistory) Add(line string) {
	if h == nil || strings.TrimSpace(line) == "" {
		return
	}
	h.entries = append(h.entries, line)
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
// bubbletea's raw-mode line editor: Tab-completion (bubbles/textinput's
// built-in inline suggestion) over candidates, and Up/Down arrow-key
// recall through history. It owns the terminal only for the duration of
// one line — bubbletea restores cooked mode before Run returns — so a
// caller's own subsequent blocking reads (e.g. an approval prompt reading
// from the same *bufio.Reader over os.Stdin) behave exactly as they did
// before this existed.
//
// Returns ErrPromptEOF on Ctrl-C or Ctrl-D instead of the submitted line.
func ReadInteractiveLine(prompt string, candidates []string, history *PromptHistory) (string, error) {
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

// promptModel is the bubbletea model backing ReadInteractiveLine: a
// bubbles/textinput field for the line itself (which already implements
// Tab-to-complete inline suggestions when ShowSuggestions is set), plus
// this session's own Up/Down history browsing layered on top —
// intercepted before textinput's own Update so its default KeyMap
// (which binds Up/Down to *suggestion* cycling, not history) never sees
// those keys.
type promptModel struct {
	input      textinput.Model
	history    *PromptHistory
	historyIdx int // -1 = editing the live draft, not browsing history
	draft      string
	submitted  string
	eof        bool
}

func newPromptModel(prompt string, candidates []string, history *PromptHistory) promptModel {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.Focus()
	ti.ShowSuggestions = len(candidates) > 0
	ti.SetSuggestions(candidates)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	ti.CompletionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return promptModel{input: ti, history: history, historyIdx: -1}
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
		case tea.KeyEnter:
			m.submitted = m.input.Value()
			return m, tea.Quit
		case tea.KeyUp:
			m.browseHistory(1)
			return m, nil
		case tea.KeyDown:
			m.browseHistory(-1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
	return m.input.View()
}
