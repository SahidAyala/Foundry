package cli

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptHistory_AddAndAt(t *testing.T) {
	h := NewPromptHistory()
	h.Add("/feature add x")
	h.Add("/review")
	h.Add("  ") // blank, must not be recorded

	if got, ok := h.at(0); !ok || got != "/review" {
		t.Errorf("at(0) = %q, %v, want %q, true (most recent)", got, ok, "/review")
	}
	if got, ok := h.at(1); !ok || got != "/feature add x" {
		t.Errorf("at(1) = %q, %v, want %q, true", got, ok, "/feature add x")
	}
	if _, ok := h.at(2); ok {
		t.Error("at(2) reported ok=true, want false: only 2 entries recorded")
	}
}

func TestPromptHistory_NilSafe(t *testing.T) {
	var h *PromptHistory
	h.Add("/feature") // must not panic
	if _, ok := h.at(0); ok {
		t.Error("at(0) on a nil *PromptHistory reported ok=true, want false")
	}
}

func TestPromptModel_EnterSubmitsValue(t *testing.T) {
	m := newPromptModel("foundry> ", nil, nil)
	m = typeString(t, m, "/feature add x")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := next.(promptModel)
	if pm.submitted != "/feature add x" {
		t.Errorf("submitted = %q, want %q", pm.submitted, "/feature add x")
	}
	if cmd == nil {
		t.Error("Update on Enter returned a nil tea.Cmd, want tea.Quit")
	}
}

func TestPromptModel_CtrlCSignalsEOF(t *testing.T) {
	m := newPromptModel("foundry> ", nil, nil)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	pm := next.(promptModel)
	if !pm.eof {
		t.Error("eof = false after Ctrl-C, want true")
	}
	if cmd == nil {
		t.Error("Update on Ctrl-C returned a nil tea.Cmd, want tea.Quit")
	}
}

func TestPromptModel_HistoryBrowsing(t *testing.T) {
	history := NewPromptHistory()
	history.Add("/feature add x")
	history.Add("/review")

	m := newPromptModel("foundry> ", nil, history)
	m = typeString(t, m, "draft in progress")

	// Up twice recalls both history entries, oldest last.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(promptModel)
	if got := m.input.Value(); got != "/review" {
		t.Errorf("after one Up, value = %q, want %q", got, "/review")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(promptModel)
	if got := m.input.Value(); got != "/feature add x" {
		t.Errorf("after two Ups, value = %q, want %q", got, "/feature add x")
	}

	// Down twice returns to the original draft, unmodified.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(promptModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(promptModel)
	if got := m.input.Value(); got != "draft in progress" {
		t.Errorf("after returning past the newest entry, value = %q, want the original draft %q", got, "draft in progress")
	}
}

// typeString feeds each rune of s through Update as a separate KeyRunes
// message, exactly as bubbletea would deliver real keystrokes.
func typeString(t *testing.T, m promptModel, s string) promptModel {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(promptModel)
	}
	return m
}

func TestIsInteractiveTerminal_FalseForNonFileIO(t *testing.T) {
	if IsInteractiveTerminal(strings.NewReader(""), &bytes.Buffer{}) {
		t.Error("IsInteractiveTerminal returned true for non-*os.File input/output")
	}
}
