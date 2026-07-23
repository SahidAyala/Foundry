package cli

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestLoadPromptHistory_MissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".foundry", "history")
	h := LoadPromptHistory(path)
	if _, ok := h.at(0); ok {
		t.Error("at(0) on a freshly loaded, never-written history reported ok=true, want false")
	}
}

func TestLoadPromptHistory_ReadsExistingLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := os.WriteFile(path, []byte("/feature add x\n/review\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	h := LoadPromptHistory(path)
	if got, ok := h.at(0); !ok || got != "/review" {
		t.Errorf("at(0) = %q, %v, want %q, true (most recent)", got, ok, "/review")
	}
	if got, ok := h.at(1); !ok || got != "/feature add x" {
		t.Errorf("at(1) = %q, %v, want %q, true", got, ok, "/feature add x")
	}
}

func TestPromptHistory_AddPersistsAcrossLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".foundry", "history") // parent dir does not exist yet
	first := LoadPromptHistory(path)
	first.Add("/feature add x")
	first.Add("/review")

	second := LoadPromptHistory(path)
	if got, ok := second.at(0); !ok || got != "/review" {
		t.Errorf("reloaded at(0) = %q, %v, want %q, true", got, ok, "/review")
	}
	if got, ok := second.at(1); !ok || got != "/feature add x" {
		t.Errorf("reloaded at(1) = %q, %v, want %q, true", got, ok, "/feature add x")
	}
}

func TestNewPromptHistory_AddDoesNotPersist(t *testing.T) {
	h := NewPromptHistory()
	h.Add("/feature add x") // must not panic or attempt any filesystem write
	if got, ok := h.at(0); !ok || got != "/feature add x" {
		t.Errorf("at(0) = %q, %v, want %q, true (in-memory only)", got, ok, "/feature add x")
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

func testCandidates() []CommandCandidate {
	return []CommandCandidate{
		{Name: "/feature", Description: "Run the \"feature\" Pipeline."},
		{Name: "/bug", Description: "Run the \"bugfix\" Pipeline."},
		{Name: "/help", Description: "List the slash commands."},
	}
}

func TestPromptModel_MenuOpensOnSlashAndFiltersByPrefix(t *testing.T) {
	m := newPromptModel("foundry> ", testCandidates(), nil)
	if m.menuOpen {
		t.Fatal("menu is open before any input, want closed")
	}

	m = typeString(t, m, "/")
	if !m.menuOpen || len(m.filtered) != 3 {
		t.Fatalf("after typing \"/\", menuOpen = %v, filtered = %d entries, want open with all 3", m.menuOpen, len(m.filtered))
	}

	m = typeString(t, m, "b")
	if !m.menuOpen || len(m.filtered) != 1 || m.filtered[0].Name != "/bug" {
		t.Fatalf("after typing \"/b\", filtered = %+v, want exactly [/bug]", m.filtered)
	}
}

func TestPromptModel_MenuClosesOnSpaceOrNoLeadingSlash(t *testing.T) {
	m := newPromptModel("foundry> ", testCandidates(), nil)
	m = typeString(t, m, "/bu") // a partial, not-yet-exact prefix match
	if !m.menuOpen {
		t.Fatal("menu should be open after typing a matching, not-yet-complete prefix")
	}

	m = typeString(t, m, "g ") // finishes the name, then a space into its arguments
	if m.menuOpen {
		t.Error("menu stayed open after a space (into the command's arguments), want closed")
	}
}

func TestPromptModel_MenuNavigationAndAccept(t *testing.T) {
	m := newPromptModel("foundry> ", testCandidates(), nil)
	m = typeString(t, m, "/")
	if got := m.filtered[m.menuCursor].Name; got != "/feature" {
		t.Fatalf("initial menu cursor = %q, want the first candidate %q", got, "/feature")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(promptModel)
	if got := m.filtered[m.menuCursor].Name; got != "/bug" {
		t.Fatalf("after Down, cursor = %q, want %q", got, "/bug")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(promptModel)
	if m.menuOpen {
		t.Error("menu stayed open after Enter accepted a selection, want closed")
	}
	if got := m.input.Value(); got != "/bug " {
		t.Errorf("input value after accepting = %q, want %q", got, "/bug ")
	}
	if m.submitted != "" {
		t.Error("accepting a menu selection must not submit the line — only a later, unmenued Enter does")
	}
}

// TestPromptModel_ExactMatchClosesMenuSoOneEnterSubmits is a regression
// test for a real bug a live pty run surfaced: typing a command's name
// out in full (e.g. "/exit", which no other candidate's name has as a
// proper prefix) left the menu open, so the first Enter only "accepted"
// the already-fully-typed selection (appending a trailing space) instead
// of submitting the line — forcing a confusing second Enter for every
// single command whenever the user typed instead of arrow-selected.
func TestPromptModel_ExactMatchClosesMenuSoOneEnterSubmits(t *testing.T) {
	m := newPromptModel("foundry> ", testCandidates(), nil)
	m = typeString(t, m, "/bug")
	if m.menuOpen {
		t.Fatal("menu still open once the input exactly matches a candidate's name, want closed")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := next.(promptModel)
	if pm.submitted != "/bug" {
		t.Errorf("submitted = %q after one Enter on an exact match, want %q", pm.submitted, "/bug")
	}
	if cmd == nil {
		t.Error("Update on Enter returned a nil tea.Cmd, want tea.Quit")
	}
}

func TestPromptModel_EscClosesMenuWithoutClearingInput(t *testing.T) {
	m := newPromptModel("foundry> ", testCandidates(), nil)
	m = typeString(t, m, "/bu")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(promptModel)
	if m.menuOpen {
		t.Error("menu still open after Esc, want closed")
	}
	if got := m.input.Value(); got != "/bu" {
		t.Errorf("input value after Esc = %q, want it unchanged at %q", got, "/bu")
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
