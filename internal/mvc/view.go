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
	bodyH := m.height - 3 // tab bar + gap (Panel's MarginTop) + help line
	panelW := m.width - 2 // left + right border
	panelH := bodyH - 2   // top + bottom border

	// Single panel: list. Underline the selected row only when the cursor is on the list
	rows := m.items[m.tabs[m.active]]
	if m.err != nil {
		rows = []string{"error: " + m.err.Error()}
	}
	var list strings.Builder
	for i, r := range rows {
		if i == m.cursor {
			style := Selected.Width(panelW - 2)
			if m.focus == focusList {
				style = style.Underline(true)
			}
			list.WriteString(style.Render(r))
		} else {
			list.WriteString(r)
		}
		list.WriteString("\n")
	}
	body := Panel.Width(panelW).Height(panelH).Render(list.String())

	// Set the refresh buttons into the panel's top border, right under the tabs.
	body = withButtons(body, m.refreshButtons(), lipgloss.Width(body))

	// MaxWidth cuts a line off at the terminal edge instead of wrapping it,
	// so a narrow terminal never gets an extra line. Every one-line part uses it:
	// JoinVertical pads all lines to the widest one, so one wide line widens them all.
	fit := lipgloss.NewStyle().MaxWidth(m.width)

	tabBar = fit.Render(tabBar)
	footer := fit.Render(Help.Render("tab focus • ←→ tab • ↑↓ move • a auto • f rate • q quit"))

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, footer)
}
