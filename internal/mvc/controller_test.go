package mvc

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Key presses used in the tests.
var (
	down     = tea.KeyMsg{Type: tea.KeyDown}
	up       = tea.KeyMsg{Type: tea.KeyUp}
	left     = tea.KeyMsg{Type: tea.KeyLeft}
	right    = tea.KeyMsg{Type: tea.KeyRight}
	tab      = tea.KeyMsg{Type: tea.KeyTab}
	shiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
	quit     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	ctrlC    = tea.KeyMsg{Type: tea.KeyCtrlC}
	j        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	k        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	h        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}
	l        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
)

// press sends each message to Update in order and returns the final model.
func press(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestStartsOnTabs(t *testing.T) {
	m := New()
	if m.focus != focusTabs || m.active != 0 || m.cursor != 0 {
		t.Fatalf("got focus=%d active=%d cursor=%d, want focusTabs, 0, 0", m.focus, m.active, m.cursor)
	}
}

func TestWindowSize(t *testing.T) {
	m := press(t, New(), tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Fatalf("got %dx%d, want 80x24", m.width, m.height)
	}
}

func TestFocusMoves(t *testing.T) {
	tests := []struct {
		name       string
		keys       []tea.Msg
		wantFocus  focus
		wantCursor int
	}{
		{"down from tabs enters list", []tea.Msg{down}, focusList, 0},
		{"down in list moves cursor", []tea.Msg{down, down}, focusList, 1},
		{"up in list moves cursor", []tea.Msg{down, down, up}, focusList, 0},
		{"up on top row goes back to tabs", []tea.Msg{down, up}, focusTabs, 0},
		{"up on tabs does nothing", []tea.Msg{up}, focusTabs, 0},
		{"tab goes to list", []tea.Msg{tab}, focusList, 0},
		{"tab twice goes back to tabs", []tea.Msg{tab, tab}, focusTabs, 0},
		{"tab keeps cursor", []tea.Msg{down, down, tab, tab}, focusList, 1},
		{"j and k work like down and up", []tea.Msg{j, j, k}, focusList, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := press(t, New(), tt.keys...)
			if m.focus != tt.wantFocus || m.cursor != tt.wantCursor {
				t.Fatalf("got focus=%d cursor=%d, want focus=%d cursor=%d",
					m.focus, m.cursor, tt.wantFocus, tt.wantCursor)
			}
		})
	}
}

func TestCursorStopsAtLastRow(t *testing.T) {
	m := New()
	last := len(m.items["Pods"]) - 1

	// Enter the list, then press down many more times than there are rows.
	m = press(t, m, down)
	for range 10 {
		m = press(t, m, down)
	}
	if m.cursor != last {
		t.Fatalf("cursor = %d, want last row %d", m.cursor, last)
	}
}

func TestSwitchTabs(t *testing.T) {
	tests := []struct {
		name       string
		keys       []tea.Msg
		wantActive int
	}{
		{"right goes to next tab", []tea.Msg{right}, 1},
		{"l goes to next tab", []tea.Msg{l}, 1},
		{"left goes to previous tab", []tea.Msg{right, left}, 0},
		{"h goes to previous tab", []tea.Msg{right, h}, 0},
		{"shift+tab goes to previous tab", []tea.Msg{right, shiftTab}, 0},
		{"right on last tab wraps to first", []tea.Msg{right, right, right}, 0},
		{"left on first tab wraps to last", []tea.Msg{left}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := press(t, New(), tt.keys...)
			if m.active != tt.wantActive {
				t.Fatalf("active = %d, want %d", m.active, tt.wantActive)
			}
		})
	}
}

func TestSwitchTabResetsCursor(t *testing.T) {
	// Move to row 2 in the list, then switch tab.
	m := press(t, New(), down, down, down, right)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d after switching tab, want 0", m.cursor)
	}
}

func TestQuit(t *testing.T) {
	for _, key := range []tea.KeyMsg{quit, ctrlC} {
		t.Run(key.String(), func(t *testing.T) {
			_, cmd := New().Update(key)
			if cmd == nil {
				t.Fatal("got no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("command returned %T, want tea.QuitMsg", cmd())
			}
		})
	}
}
