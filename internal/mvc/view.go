package mvc

import (
	"k8s-tui/internal/styles"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// activeTab draws the currently selected tab as a filled "pill".
	ActiveTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.OnAccent). // was styles.ColorCream
			Background(styles.Accent).
			Padding(0, 2) // 0 lines top/bottom, 2 spaces left/right

	// inactiveTab draws the other tabs as plain gray text.
	// Same padding as activeTab so tabs don't shift when switching.
	InactiveTab = lipgloss.NewStyle().
			Foreground(styles.Muted).
			Padding(0, 2)

	// Panel draws a square single-line box around content.
	// Set the size per use, e.g. Panel.Width(30).Height(10).Render(text).
	Panel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()). // ┌─┐ │ │ └─┘ corners
		BorderForeground(styles.Accent). // colors only the border lines
		Padding(0, 1)                    // 1 space between border and text

	// selected highlights the row under the cursor in the list.
	Selected = lipgloss.NewStyle().
			Background(styles.Shade). // VS Code's selected-row blue
			Foreground(styles.OnAccent)

	// help draws the key-hint line at the bottom of the screen.
	Help = lipgloss.NewStyle().Foreground(styles.Muted)
)

// tabGap is the number of empty columns between two tabs.
const tabGap = 2

// panelMarginY is the number of empty rows above the panel (below the tabs)
// and below it (above the help line).
const panelMarginY = 1

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	// Tab bar: underline the active tab only when the cursor is on the tabs
	var tabs []string
	for i, t := range m.tabs {
		if i == m.active {
			style := ActiveTab
			if m.focus == focusTabs {
				style = style.Underline(true)
			}
			tabs = append(tabs, style.Render(t))
		} else {
			tabs = append(tabs, InactiveTab.Render(t))
		}
	}
	tabBar := strings.Join(tabs, strings.Repeat(" ", tabGap))

	// Panel size: full width, full height minus the lines around it.
	// Width/Height include padding but not the border or margin, so subtract those.
	// If the total is taller than the terminal, the top lines (the tab bar) scroll off.
	panelW := m.width - 2    // left + right border
	panelH := m.listHeight() // rows inside the panel (see scroll.go)
	innerW := panelW - 2     // minus Panel's Padding(0, 1) on each side

	// Single panel: the rows that fit, from the scroll offset on.
	// Underline the selected row only when the cursor is on the list.
	rows := m.items[m.tabs[m.active]]
	if m.err != nil {
		rows = []string{"error: " + m.err.Error()}
	}
	first, end := visibleRange(m.offset, len(rows), panelH)

	// A row longer than the panel is cut off at its edge: a wrapped row would
	// take two lines and push the panel taller than the screen.
	cut := lipgloss.NewStyle().MaxWidth(innerW)

	lines := make([]string, 0, end-first)
	for i := first; i < end; i++ {
		line := cut.Render(rows[i])
		if i == m.cursor {
			style := Selected.Width(innerW)
			if m.focus == focusList {
				style = style.Underline(true)
			}
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	body := Panel.Width(panelW).Height(panelH).Render(strings.Join(lines, "\n"))

	// "4-9 of 30" on the bottom border while the list is longer than the panel.
	body = withScrollInfo(body, scrollInfo(first, end, len(rows), panelH), lipgloss.Width(body))

	// Set the buttons into the panel's top border, right under the tabs.
	// The one the cursor is on (if any) is underlined.
	if m.buttonsShown() {
		row := buttonRow(m.panelButtons(), m.focusedButton())
		body = withButtons(body, row, m.lastUpdated(), lipgloss.Width(body))
	}

	// Empty rows above and below the panel. Added after withButtons, which
	// needs the panel's top border to be its first line.
	body = lipgloss.NewStyle().MarginTop(panelMarginY).MarginBottom(panelMarginY).Render(body)

	// MaxWidth cuts a line off at the terminal edge instead of wrapping it,
	// so a narrow terminal never gets an extra line. Every one-line part uses it:
	// JoinVertical pads all lines to the widest one, so one wide line widens them all.
	fit := lipgloss.NewStyle().MaxWidth(m.width)

	tabBar = fit.Render(tabBar)
	footer := fit.Render(Help.Render("tab focus • ←→ tab/button • ↑↓ move • enter press • w wide • r refresh • q quit"))

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, footer)
}
