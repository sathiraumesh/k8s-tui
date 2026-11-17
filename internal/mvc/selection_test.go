package mvc

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// copied catches what is put on the clipboard, instead of the terminal.
func copied(t *testing.T) *string {
	t.Helper()
	var got string
	old := writeClipboard
	writeClipboard = func(text string) { got = text }
	t.Cleanup(func() { writeClipboard = old })
	return &got
}

// drag presses at from, moves to to and lets go; it returns the model and
// what the release returned.
func drag(t *testing.T, m Model, from, to point) (Model, tea.Cmd) {
	t.Helper()
	m = press(t, m,
		tea.MouseMsg{X: from.x, Y: from.y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		tea.MouseMsg{X: to.x, Y: to.y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	next, cmd := m.Update(tea.MouseMsg{X: to.x, Y: to.y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	return next.(Model), cmd
}

// A drag over a word copies it, and says so.
func TestDragCopiesText(t *testing.T) {
	got := copied(t)
	m, _ := nodesOnScreen(t)
	x, y := at(t, m, "worker1")
	m, cmd := drag(t, m, point{x, y}, point{x + len("worker1") - 1, y})
	if cmd == nil {
		t.Fatal("letting go didn't copy")
	}
	m = press(t, m, cmd())
	if *got != "worker1" || m.notice != "copied 7 characters" {
		t.Errorf("copied %q, notice %q", *got, m.notice)
	}
	if m.sel == nil {
		t.Error("the selection isn't shown after copying")
	}
	// A drag is not a click: the cursor stays where it was (worker2).
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want it unmoved", m.cursor)
	}
}

// A drag over two lines copies both, as the terminal selects them.
func TestDragTwoLines(t *testing.T) {
	got := copied(t)
	m, _ := nodesOnScreen(t)
	x1, y1 := at(t, m, "worker1")
	x2, y2 := at(t, m, "worker2")
	m, cmd := drag(t, m, point{x2 + 6, y2}, point{x1, y1}) // backwards, from the end
	press(t, m, cmd())
	lines := strings.Split(*got, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "worker1") || !strings.HasSuffix(lines[1], "worker2") {
		t.Errorf("copied %q", *got)
	}
}

// The selection is drawn reversed, and a key clears it.
func TestSelectionShownAndCleared(t *testing.T) {
	withColors(t)
	m, _ := nodesOnScreen(t)
	x, y := at(t, m, "worker1")
	m, _ = drag(t, m, point{x, y}, point{x + 3, y})
	if line := lineWith(t, m, "worker1"); !strings.Contains(line, "\x1b[7m") {
		t.Errorf("not reversed: %q", line)
	}
	if m = press(t, m, key("x")); m.sel != nil {
		t.Error("a key didn't clear the selection")
	}
}

// A press and release in one place is still a click.
func TestClickIsNotADrag(t *testing.T) {
	m, _ := nodesOnScreen(t)
	if m = click(t, m, "worker1"); m.sel != nil || m.cursor != 0 {
		t.Errorf("sel = %v, cursor = %d", m.sel, m.cursor)
	}
}

// ---------- the right-click menu ----------

// menuLabels are the labels of the open menu.
func menuLabels(m Model) []string {
	var out []string
	for _, it := range m.menu.items {
		out = append(out, it.label)
	}
	return out
}

// Right click on a row: copy that row, everything, the command; no Back on
// the list.
func TestMenuOnRow(t *testing.T) {
	got := copied(t)
	m, _ := nodesOnScreen(t)
	x, y := at(t, m, "worker1")
	m = rightClickAt(t, m, x, y)
	if strings.Join(menuLabels(m), ",") != "Copy row,Copy all,Copy command" {
		t.Fatalf("menu = %q", menuLabels(m))
	}
	if !strings.Contains(ansi.Strip(m.View()), "Copy command") {
		t.Fatal("the menu isn't drawn")
	}
	mx, my := at(t, m, "Copy row")
	n, cmd := m.Update(tea.MouseMsg{X: mx + 2, Y: my, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}) // a press picks
	next := press(t, n.(Model), cmd())
	if *got != "worker1   Ready   v1.37.2" || next.menu != nil {
		t.Errorf("copied %q, menu = %v", *got, next.menu)
	}
}

// With a selection the menu offers to copy it first.
func TestMenuCopiesSelection(t *testing.T) {
	got := copied(t)
	m, _ := nodesOnScreen(t)
	x, y := at(t, m, "worker2")
	m, _ = drag(t, m, point{x, y}, point{x + 5, y})
	m = rightClickAt(t, m, 50, 10)
	if menuLabels(m)[0] != "Copy" {
		t.Fatalf("menu = %q", menuLabels(m))
	}
	next, cmd := m.Update(enterKey) // enter picks the first item
	press(t, next.(Model), cmd())
	if *got != "worker" {
		t.Errorf("copied %q", *got)
	}
}

// Copy command copies the kubectl command on the bottom border.
func TestMenuCopyCommand(t *testing.T) {
	got := copied(t)
	m, _ := nodesOnScreen(t)
	m = rightClickAt(t, m, 50, 3)
	m = press(t, m, down, down) // Copy all, Copy command (no row at line 3)
	next, cmd := m.Update(enterKey)
	press(t, next.(Model), cmd())
	if *got != "kubectl get nodes" {
		t.Errorf("copied %q, menu was %q", *got, menuLabels(m))
	}
}

// esc, or a click outside it, closes the menu without doing anything.
func TestMenuCloses(t *testing.T) {
	m, _ := nodesOnScreen(t)
	if m = press(t, rightClickAt(t, m, 50, 10), escKey); m.menu != nil || m.overlay != overlayNone {
		t.Errorf("esc: menu = %v, overlay = %d", m.menu, m.overlay)
	}
	m = rightClickAt(t, m, 50, 10)
	if m = clickAt(t, m, 0, 0); m.menu != nil || m.activeName() != tabNodes {
		t.Errorf("click outside: menu = %v, active = %q", m.menu, m.activeName())
	}
}

// Near the bottom right corner the menu moves up and left to fit.
func TestMenuFitsOnScreen(t *testing.T) {
	m, _ := nodesOnScreen(t)
	m = rightClickAt(t, m, m.width-1, m.height-1)
	p := m.menu.place(m.width, m.height)
	box := m.menu.box()
	if p.x+ansi.StringWidth(strings.Split(box, "\n")[0]) > m.width || p.y+strings.Count(box, "\n")+1 > m.height {
		t.Errorf("menu at %v doesn't fit a %dx%d screen", p, m.width, m.height)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Errorf("screen has %d lines, want %d", len(lines), m.height)
	}
}
