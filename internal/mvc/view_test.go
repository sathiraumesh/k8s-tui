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
