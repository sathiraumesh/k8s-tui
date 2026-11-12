package mvc

import (
	"strings"

	"k8s-tui/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

// The secondary bar: the buttons set into the panel's top border, under the tabs:
//
//	┌─  wide: off  ───  refresh: 5s  ─────────────── updated 14:30:05 ─┐
//
// Only drawing lives here; the refresh logic (timer, rates, loading) is in
// refresh.go, and what pressing a button does is in controller.go.

// buttonGap is the number of border pieces (─) between two buttons.
const buttonGap = 3

// Button names, so Update knows which button was pressed.
const (
	buttonWide    = "wide"
	buttonRefresh = "refresh"
)

// button is one label set into the panel's top border, e.g. "refresh: 5s".
type button struct {
	name  string // one of the button names above
	label string // the text on the button
	on    bool   // green when true, dark gray when false
}

// Styles for the buttons on the top border (same look as systems-programing).
var (
	// buttonOn is a button that is switched on: green.
	buttonOn = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(styles.OnAccent).
			Background(styles.On)

	// buttonOff is a button that is switched off: dark gray, same padding so nothing shifts.
	buttonOff = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(styles.OnAccent).
			Background(styles.Off)

	// borderLine draws border pieces in the panel's border colour.
	borderLine = lipgloss.NewStyle().Foreground(styles.Accent)
)

// render draws the button. focused is whether the cursor is on it: then its
// text is bold, white and underlined, like the focused tab.
func (b button) render(focused bool) string {
	s := buttonOff
	if b.on {
		s = buttonOn
	}
	if focused {
		s = s.Bold(true).Foreground(styles.ColorWhite).Underline(true)
	}
	return s.Render(b.label)
}

// panelButtons are the buttons on the panel's top border, left to right
// (same order as systems-programing):
//
//	[wide: off] [refresh: 5s]
//
// Each is green when on: wide while the active tab shows its wide columns,
// refresh while auto refresh runs.
func (m Model) panelButtons() []button {
	wide := button{name: buttonWide, label: "wide: off"}
	if m.activeWide() {
		wide.label, wide.on = "wide: on", true
	}

	// A tab that isn't auto refreshed (More) has no refresh button.
	if !m.autoRefreshed() {
		return []button{wide}
	}

	refresh := button{name: buttonRefresh, label: "refresh: off"}
	if m.refreshEvery != refreshOff {
		refresh.label, refresh.on = "refresh: "+m.refreshEvery.String(), true
	}

	return []button{wide, refresh}
}

// focusedButton is the index of the button the cursor is on, or -1 if the
// cursor is somewhere else.
func (m Model) focusedButton() int {
	if m.focus != focusButtons {
		return -1
	}
	return m.button
}

// buttonRow draws the buttons side by side, joined by pieces of border.
// focused is the index of the button the cursor is on, or -1.
func buttonRow(buttons []button, focused int) string {
	parts := make([]string, len(buttons))
	for i, b := range buttons {
		parts[i] = b.render(i == focused)
	}
	return strings.Join(parts, borderLine.Render(strings.Repeat("─", buttonGap)))
}

// buttonsShown reports whether the buttons fit on the top border. The
// corners and one piece of border on each side take 4 columns. When they
// don't fit they are not drawn, and the cursor skips them.
func (m Model) buttonsShown() bool {
	return lipgloss.Width(buttonRow(m.panelButtons(), -1)) <= m.width-4
}

// lastUpdated is the text for the right corner of the top border,
// "updated 14:30:05": when the active tab last loaded. It is "" while
// refresh is off, or before the tab has loaded once.
//
// On a tab that isn't auto refreshed (More) it is "loaded 14:30:05" instead,
// shown whatever the refresh rate: the tab is loaded once, not kept updated.
func (m Model) lastUpdated() string {
	at, ok := m.updated[m.activeName()]
	switch {
	case !ok:
		return ""
	case !m.autoRefreshed():
		return "loaded " + at.Format("15:04:05")
	case m.refreshEvery == refreshOff:
		return ""
	}
	return "updated " + at.Format("15:04:05")
}

// withButtons redraws the first line of box, its top border, with row set
// into it on the left and right (if not "") on the right:
//
//	┌─[refresh: 5s]─────────────────────────────── updated 14:30:05 ─┐
//
// width is the box's full width. If right doesn't fit next to row, it is
// left out. If row doesn't fit either, box is returned as it is, with a
// plain top border.
func withButtons(box, row, right string, width int) string {
	return withBorderText(box, edgeTop, row, borderLabel(Help, right), width)
}
