package mvc

import (
	"strings"

	"k8s-tui/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// View code for the buttons set into the panel's top border:
//
//	┌─ refresh: on ─ every 5s ─ updated 14:30:05 ──────────────────┐
//
// Only drawing lives here; the refresh logic (timer, rates, loading) is in refresh.go.

// buttonGap is the number of border pieces (─) between two buttons.
const buttonGap = 3

// button is one label set into the panel's top border, e.g. "refresh: on".
type button struct {
	label string
	on    bool // drawn in the accent colour when true, gray when false
}

// Styles for the buttons on the top border.
var (
	// buttonOn is a button that is active: filled, like the active tab.
	buttonOn = lipgloss.NewStyle().
			Foreground(styles.OnAccent).
			Background(styles.Accent).
			Padding(0, 1)

	// buttonOff is a button that is not active; same padding so nothing shifts.
	buttonOff = lipgloss.NewStyle().Foreground(styles.Muted).Padding(0, 1)

	// borderLine draws border pieces in the panel's border colour.
	borderLine = lipgloss.NewStyle().Foreground(styles.Accent)
)

// render draws the button in its on or off style.
func (b button) render() string {
	if b.on {
		return buttonOn.Render(b.label)
	}
	return buttonOff.Render(b.label)
}

// refreshButtons are the buttons on the panel's top border, left to right:
//
//	[refresh: on] [every 5s] [updated 14:30:05]
//
// "refresh" and the rate are filled while auto refresh runs, gray when it is
// off. "updated" is left out until the active tab has loaded once.
func (m Model) refreshButtons() []button {
	state := "off"
	if m.autoRefresh {
		state = "on"
	}
	buttons := []button{
		{label: "refresh: " + state, on: m.autoRefresh},
		{label: "every " + m.refreshEvery.String(), on: m.autoRefresh},
	}
	if at, ok := m.updated[m.tabs[m.active]]; ok {
		buttons = append(buttons, button{label: "updated " + at.Format("15:04:05")})
	}
	return buttons
}

// withButtons redraws the first line of box, its top border, with the
// buttons set into it:
//
//	┌─[refresh: on]─[every 5s]─[updated 14:30:05]────────────────┐
//
// width is the box's full width. If the buttons don't fit, box is returned
// as it is, with a plain top border.
func withButtons(box string, buttons []button, width int) string {
	parts := make([]string, len(buttons))
	for i, b := range buttons {
		parts[i] = b.render()
	}
	row := strings.Join(parts, borderLine.Render(strings.Repeat("─", buttonGap)))

	b := lipgloss.NormalBorder()
	fill := width - 3 - lipgloss.Width(row) // 3 = "┌─" and "┐"
	if fill < 1 {
		return box
	}
	top := borderLine.Render(b.TopLeft+b.Top) + row + borderLine.Render(strings.Repeat(b.Top, fill)+b.TopRight)

	_, rest, _ := strings.Cut(box, "\n")
	return top + "\n" + rest
}
