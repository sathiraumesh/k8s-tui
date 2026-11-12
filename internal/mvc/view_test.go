package mvc

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// sized returns a model that has received a terminal size.
func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	return press(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}

func TestViewLoadingBeforeSize(t *testing.T) {
	if got := testModel().View(); got != "loading..." {
		t.Fatalf("View() = %q, want %q", got, "loading...")
	}
}

// The screen must fill the terminal exactly. Taller and the tab bar scrolls
// off the top; shorter and there is an empty row at the bottom.
func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},
		{120, 40},
		{60, 15},
		{40, 10},
	}
	for _, s := range sizes {
		t.Run(fmt.Sprintf("%dx%d", s.w, s.h), func(t *testing.T) {
			out := sized(t, testModel(), s.w, s.h).View()
			if got := lipgloss.Height(out); got != s.h {
				t.Errorf("height = %d, want %d", got, s.h)
			}
			if got := lipgloss.Width(out); got > s.w {
				t.Errorf("width = %d, want at most %d", got, s.w)
			}
		})
	}
}

// The tab bar must be the first line, or it has scrolled off.
func TestViewTabBarOnFirstLine(t *testing.T) {
	out := sized(t, testModel(), 80, 24).View()
	first := strings.Split(out, "\n")[0]
	for _, name := range []string{"Nodes", "Pods", "Deployments", "Services"} {
		if !strings.Contains(first, name) {
			t.Errorf("first line %q is missing tab %q", first, name)
		}
	}
}

func TestViewShowsActiveTabRows(t *testing.T) {
	m := sized(t, testModel(), 80, 24)
	m = press(t, m, right, right) // Nodes → Pods → Deployments
	out := m.View()
	for _, row := range m.items["Deployments"] {
		if !strings.Contains(out, row) {
			t.Errorf("view is missing row %q", row)
		}
	}
	if strings.Contains(out, "redis-0") {
		t.Error("view shows a Pods row while the Deployments tab is active")
	}
}

func TestViewEmptyTab(t *testing.T) {
	m := testModel()
	m.items["Nodes"] = nil
	out := sized(t, m, 80, 24).View() // must not panic
	if got := lipgloss.Height(out); got != 24 {
		t.Errorf("height = %d, want 24", got)
	}
}

// A small terminal must not crash View.
func TestViewTinyTerminal(t *testing.T) {
	for _, s := range []struct{ w, h int }{{20, 5}, {10, 3}, {1, 1}} {
		t.Run(fmt.Sprintf("%dx%d", s.w, s.h), func(t *testing.T) {
			_ = sized(t, testModel(), s.w, s.h).View()
		})
	}
}

// A row longer than the panel must not make the screen taller.
func TestViewLongRowDoesNotGrowScreen(t *testing.T) {
	m := testModel()
	m.items["Nodes"] = []string{strings.Repeat("x", 200)}
	out := sized(t, m, 60, 15).View()
	if got := lipgloss.Height(out); got != 15 {
		t.Errorf("height = %d, want 15", got)
	}
}

// More rows than fit in the panel must not make the screen taller.
func TestViewManyRowsDoNotGrowScreen(t *testing.T) {
	m := testModel()
	var rows []string
	for i := range 50 {
		rows = append(rows, fmt.Sprintf("pod-%d", i))
	}
	m.items["Nodes"] = rows
	out := sized(t, m, 60, 15).View()
	if got := lipgloss.Height(out); got != 15 {
		t.Errorf("height = %d, want 15", got)
	}
}

// ---------- margins above and below the panel ----------

// screenLines renders the view at w×h and returns its lines.
func screenLines(t *testing.T, w, h int) []string {
	t.Helper()
	return strings.Split(sized(t, testModel(), w, h).View(), "\n")
}

// blank reports whether a line has nothing but spaces.
func blank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// The screen is, top to bottom: tab bar, margin rows, panel, margin rows, help line.
func TestViewLayoutOrder(t *testing.T) {
	lines := screenLines(t, 80, 24)

	if !strings.Contains(lines[0], "Nodes") {
		t.Errorf("line 0 = %q, want the tab bar", lines[0])
	}
	for i := 1; i <= panelMarginY; i++ {
		if !blank(lines[i]) {
			t.Errorf("line %d = %q, want an empty margin row above the panel", i, lines[i])
		}
	}
	if top := lines[1+panelMarginY]; !strings.HasPrefix(top, "┌") {
		t.Errorf("line %d = %q, want the panel's top border", 1+panelMarginY, top)
	}

	last := len(lines) - 1
	if bottom := lines[last-1-panelMarginY]; !strings.HasPrefix(bottom, "└") {
		t.Errorf("line %d = %q, want the panel's bottom border", last-1-panelMarginY, bottom)
	}
	for i := last - panelMarginY; i < last; i++ {
		if !blank(lines[i]) {
			t.Errorf("line %d = %q, want an empty margin row below the panel", i, lines[i])
		}
	}
	if !strings.Contains(lines[last], "q quit") {
		t.Errorf("last line = %q, want the help line", lines[last])
	}
}

// Exactly panelMarginY empty rows on each side: no more, no less.
func TestViewMarginRowCount(t *testing.T) {
	lines := screenLines(t, 80, 24)

	above := 0
	for i := 1; i < len(lines) && blank(lines[i]); i++ {
		above++
	}
	below := 0
	for i := len(lines) - 2; i >= 0 && blank(lines[i]); i-- {
		below++
	}
	if above != panelMarginY || below != panelMarginY {
		t.Errorf("empty rows above = %d, below = %d, want %d each", above, below, panelMarginY)
	}
}

// The panel takes every row the tab bar, margins and help line leave over,
// so the whole screen is exactly the terminal's height.
func TestViewPanelFillsSpaceBetweenMargins(t *testing.T) {
	for _, h := range []int{10, 15, 24, 40} {
		t.Run(fmt.Sprintf("height %d", h), func(t *testing.T) {
			lines := screenLines(t, 80, h)
			if len(lines) != h {
				t.Fatalf("screen has %d lines, want %d", len(lines), h)
			}
			top, bottom := 1+panelMarginY, h-2-panelMarginY
			if !strings.HasPrefix(lines[top], "┌") || !strings.HasPrefix(lines[bottom], "└") {
				t.Errorf("panel is not between rows %d and %d:\n%s", top, bottom, strings.Join(lines, "\n"))
			}
			if want := h - 2 - 2*panelMarginY; bottom-top+1 != want {
				t.Errorf("panel is %d rows tall, want %d", bottom-top+1, want)
			}
		})
	}
}

// The refresh button stays on the panel's top border, not on a margin row.
func TestViewButtonNotOnMarginRow(t *testing.T) {
	lines := screenLines(t, 80, 24)
	for i := 1; i <= panelMarginY; i++ {
		if strings.Contains(lines[i], "refresh") {
			t.Errorf("margin row %d = %q has the refresh button, want it on the border", i, lines[i])
		}
	}
	if !strings.Contains(lines[1+panelMarginY], "refresh: 5s") {
		t.Errorf("top border %q is missing the refresh button", lines[1+panelMarginY])
	}
}

// The margin rows are as wide as the rest of the screen, so nothing shifts.
func TestViewMarginRowsFullWidth(t *testing.T) {
	lines := screenLines(t, 80, 24)
	last := len(lines) - 1
	for _, i := range []int{1, last - 1} {
		if w := lipgloss.Width(lines[i]); w != 80 {
			t.Errorf("margin row %d is %d wide, want 80", i, w)
		}
	}
}
