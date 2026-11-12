package mvc

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pgDown = tea.KeyMsg{Type: tea.KeyPgDown}
	pgUp   = tea.KeyMsg{Type: tea.KeyPgUp}
)

// longList is a model whose Nodes tab has n rows ("node-0", "node-1", …), on
// an 80×15 terminal: 15 - tab bar - help - 2 margins - 2 borders = 9 rows fit.
func longList(t *testing.T, n int) Model {
	t.Helper()
	m := New(nil)
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprintf("node-%d", i)
	}
	m.items["Nodes"] = rows
	return sized(t, m, 80, 15)
}

// inList moves the focus from the tabs into the list (past the buttons).
func inList(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, down, down)
	if m.focus != focusList {
		t.Fatalf("focus = %d, want the list", m.focus)
	}
	return m
}

// shownRows returns the rows the panel shows, in order.
func shownRows(t *testing.T, m Model) []string {
	t.Helper()
	var rows []string
	for _, line := range strings.Split(m.View(), "\n") {
		if i := strings.Index(line, "node-"); i >= 0 {
			rows = append(rows, strings.Fields(line[i:])[0])
		}
	}
	return rows
}

func TestListHeight(t *testing.T) {
	if got := longList(t, 30).listHeight(); got != 9 {
		t.Errorf("listHeight = %d at 15 rows, want 9", got)
	}
	if got := sized(t, New(nil), 80, 3).listHeight(); got != 1 {
		t.Errorf("listHeight = %d on a 3-row terminal, want at least 1", got)
	}
}

// The panel shows only the rows that fit, starting at the top.
func TestShowsOnlyRowsThatFit(t *testing.T) {
	got := shownRows(t, longList(t, 30))
	if len(got) != 9 || got[0] != "node-0" || got[8] != "node-8" {
		t.Errorf("shown rows = %v, want node-0 … node-8", got)
	}
}

// The screen stays exactly the terminal's height however long the list is.
func TestLongListKeepsScreenHeight(t *testing.T) {
	for _, n := range []int{9, 10, 100, 1000} {
		if h := lipgloss.Height(longList(t, n).View()); h != 15 {
			t.Errorf("%d rows: screen is %d rows tall, want 15", n, h)
		}
	}
}

// Moving the cursor down inside the screen doesn't scroll.
func TestCursorInsideScreenDoesNotScroll(t *testing.T) {
	m := inList(t, longList(t, 30))
	for range 8 {
		m = press(t, m, down)
	}
	if m.cursor != 8 || m.offset != 0 {
		t.Errorf("cursor = %d, offset = %d, want 8 and 0 (last row on screen)", m.cursor, m.offset)
	}
}

// Moving past the bottom row scrolls down one row at a time.
func TestCursorPastBottomScrollsDown(t *testing.T) {
	m := inList(t, longList(t, 30))
	for range 10 {
		m = press(t, m, down)
	}
	if m.cursor != 10 || m.offset != 2 {
		t.Fatalf("cursor = %d, offset = %d, want 10 and 2", m.cursor, m.offset)
	}
	if got := shownRows(t, m); got[0] != "node-2" || got[len(got)-1] != "node-10" {
		t.Errorf("shown rows = %v, want node-2 … node-10", got)
	}
}

// Moving back up past the top row scrolls up.
func TestCursorPastTopScrollsUp(t *testing.T) {
	m := inList(t, longList(t, 30))
	for range 15 {
		m = press(t, m, down) // cursor 15, offset 7
	}
	for range 9 {
		m = press(t, m, up) // cursor 6: above the top row (7)
	}
	if m.cursor != 6 || m.offset != 6 {
		t.Errorf("cursor = %d, offset = %d, want 6 and 6", m.cursor, m.offset)
	}
}

// At the last row the list stops: no empty rows below it.
func TestScrollStopsAtEnd(t *testing.T) {
	m := inList(t, longList(t, 30))
	for range 50 {
		m = press(t, m, down)
	}
	if m.cursor != 29 || m.offset != 21 {
		t.Errorf("cursor = %d, offset = %d, want 29 and 21", m.cursor, m.offset)
	}
	if got := shownRows(t, m); len(got) != 9 || got[8] != "node-29" {
		t.Errorf("shown rows = %v, want 9 rows ending with node-29", got)
	}
}

// ↑ on the first row still goes to the buttons, after scrolling back.
func TestUpFromFirstRowStillLeavesList(t *testing.T) {
	m := inList(t, longList(t, 30))
	m = press(t, m, down, down, down, up, up, up, up)
	if m.focus != focusButtons || m.offset != 0 {
		t.Errorf("focus = %d, offset = %d, want the buttons and 0", m.focus, m.offset)
	}
}

func TestPageDownAndUp(t *testing.T) {
	m := inList(t, longList(t, 30))
	m = press(t, m, pgDown)
	if m.cursor != 9 || m.offset != 1 {
		t.Errorf("after pgdown: cursor = %d, offset = %d, want 9 and 1", m.cursor, m.offset)
	}
	m = press(t, m, pgDown, pgDown, pgDown) // past the end: stops at the last row
	if m.cursor != 29 {
		t.Errorf("after more pgdown: cursor = %d, want 29", m.cursor)
	}
	m = press(t, m, pgUp)
	if m.cursor != 20 || m.offset != 20 {
		t.Errorf("after pgup: cursor = %d, offset = %d, want 20 and 20", m.cursor, m.offset)
	}
	m = press(t, m, pgUp, pgUp, pgUp) // past the start: stops at the first row
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("after more pgup: cursor = %d, offset = %d, want 0 and 0", m.cursor, m.offset)
	}
}

// pgdown/pgup only move the list while the cursor is on it.
func TestPageKeysIgnoredOutsideList(t *testing.T) {
	m := press(t, longList(t, 30), pgDown) // cursor on the tabs
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("cursor = %d, offset = %d, want 0 and 0", m.cursor, m.offset)
	}
}

func TestSwitchTabScrollsToTop(t *testing.T) {
	m := inList(t, longList(t, 30))
	m = press(t, m, pgDown, pgDown, right)
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("cursor = %d, offset = %d after switching tab, want 0 and 0", m.cursor, m.offset)
	}
}

// A shorter window still shows the cursor's row.
func TestResizeKeepsCursorVisible(t *testing.T) {
	m := inList(t, longList(t, 30))
	for range 8 {
		m = press(t, m, down) // cursor 8: the last row at 15 rows tall
	}
	m = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 10}) // 4 rows fit
	if m.offset != 5 {
		t.Errorf("offset = %d after shrinking, want 5 (cursor 8 on the last of 4 rows)", m.offset)
	}
	if got := shownRows(t, m); got[len(got)-1] != "node-8" {
		t.Errorf("shown rows = %v, want them to end with the cursor's node-8", got)
	}
}

// A refresh that returns fewer rows doesn't leave the screen scrolled past the end.
func TestShorterListAfterRefresh(t *testing.T) {
	m := inList(t, longList(t, 30))
	m = press(t, m, pgDown, pgDown, pgDown) // near the end
	m = press(t, m, itemsLoadedMsg{tab: "Nodes", rows: []string{"node-0", "node-1", "node-2"}})
	if m.cursor != 2 || m.offset != 0 {
		t.Errorf("cursor = %d, offset = %d, want 2 and 0", m.cursor, m.offset)
	}
	if got := shownRows(t, m); len(got) != 3 {
		t.Errorf("shown rows = %v, want all 3", got)
	}
}

// ---------- long rows ----------

// A row wider than the panel is cut off, not wrapped onto a second line.
func TestLongRowIsCutNotWrapped(t *testing.T) {
	m := New(nil)
	m.items["Nodes"] = []string{"node-0 " + strings.Repeat("x", 200), "node-1"}
	m = sized(t, m, 60, 15)
	if got := shownRows(t, m); len(got) != 2 {
		t.Errorf("shown rows = %v, want node-0 and node-1 on one line each", got)
	}
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line is %d wide, want at most 60: %q", w, line)
		}
	}
}

// ---------- "4-9 of 30" on the bottom border ----------

func TestScrollInfo(t *testing.T) {
	tests := []struct {
		first, end, total, h int
		want                 string
	}{
		{0, 9, 30, 9, "1-9 of 30"},
		{3, 12, 30, 9, "4-12 of 30"},
		{21, 30, 30, 9, "22-30 of 30"},
		{0, 9, 9, 9, ""}, // everything fits: nothing to scroll
		{0, 3, 3, 9, ""},
	}
	for _, tt := range tests {
		if got := scrollInfo(tt.first, tt.end, tt.total, tt.h); got != tt.want {
			t.Errorf("scrollInfo(%d, %d, %d, %d) = %q, want %q", tt.first, tt.end, tt.total, tt.h, got, tt.want)
		}
	}
}

// bottomBorder is the panel's bottom border line of the rendered screen.
func bottomBorder(t *testing.T, m Model) string {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	return lines[len(lines)-2-panelMarginY]
}

// The scroll position is at the right end of the bottom border.
func TestViewShowsScrollInfo(t *testing.T) {
	m := inList(t, longList(t, 30))
	if b := strings.TrimRight(bottomBorder(t, m), " "); !strings.HasSuffix(b, " 1-9 of 30 ─┘") {
		t.Errorf("bottom border = %q, want it to end with \" 1-9 of 30 ─┘\"", b)
	}
	m = press(t, m, pgDown)
	if b := bottomBorder(t, m); !strings.Contains(b, "2-10 of 30") {
		t.Errorf("bottom border = %q after pgdown, want 2-10 of 30", b)
	}
}

func TestViewNoScrollInfoWhenEverythingFits(t *testing.T) {
	b := bottomBorder(t, longList(t, 9))
	if strings.Contains(b, " of 9") {
		t.Errorf("bottom border = %q, want no scroll position when all rows fit", b)
	}
}

// The bottom border with the command and scroll info keeps the box's exact width.
func TestWithCommandKeepsWidth(t *testing.T) {
	box := Panel.Width(50).Height(3).Render("x")
	got := withCommand(box, "kubectl get nodes", "4-9 of 30", lipgloss.Width(box))
	for i, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w != lipgloss.Width(box) {
			t.Errorf("line %d is %d wide, want %d: %q", i, w, lipgloss.Width(box), line)
		}
	}
	if lipgloss.Height(got) != lipgloss.Height(box) {
		t.Errorf("box is %d rows, want %d", lipgloss.Height(got), lipgloss.Height(box))
	}
}

// With no command, the scroll position still shows.
func TestWithCommandOnlyInfo(t *testing.T) {
	box := Panel.Width(40).Height(1).Render("x")
	got := withCommand(box, "", "4-9 of 30", lipgloss.Width(box))
	if last := strings.Split(got, "\n")[2]; !strings.HasPrefix(last, "└─ 4-9 of 30 ─") {
		t.Errorf("bottom border = %q, want the scroll position", last)
	}
}

// A border too narrow for anything keeps its plain line.
func TestWithCommandTooNarrow(t *testing.T) {
	box := Panel.Width(4).Height(1).Render("x")
	if got := withCommand(box, "", "100-200 of 3000", lipgloss.Width(box)); got != box {
		t.Errorf("box changed though the info doesn't fit:\n%s", got)
	}
}

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		offset, total, h int
		first, end       int
	}{
		{0, 30, 9, 0, 9},
		{5, 30, 9, 5, 14},
		{25, 30, 9, 21, 30}, // past the end: pulled back so the last row is at the bottom
		{5, 3, 9, 0, 3},     // stale offset after the list got shorter
		{0, 0, 9, 0, 0},     // empty list
	}
	for _, tt := range tests {
		first, end := visibleRange(tt.offset, tt.total, tt.h)
		if first != tt.first || end != tt.end {
			t.Errorf("visibleRange(%d, %d, %d) = %d, %d, want %d, %d",
				tt.offset, tt.total, tt.h, first, end, tt.first, tt.end)
		}
	}
}
