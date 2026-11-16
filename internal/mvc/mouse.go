package mvc

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The mouse. Everything the keys do on screen can be done with it:
//
//	wheel        scrolls what is in the panel, one row a step: the list,
//	             a page of text (Describe), or a page's list (the actions)
//	left click   a tab                 switches to it
//	             a button              presses it (wide, refresh, ns, + run, search)
//	             a row of the list     selects it; clicking it again opens its actions
//	             an action             picks it
//	             a namespace/container picks it
//	             the text field        puts the cursor there to type
//	             a tick box            ticks or unticks it
//	             the last line         runs the action ("y run" / "enter run")
//	right click  goes back, like esc
//
// Unlike ↑ on the top row, the wheel never takes the cursor out of the
// panel to the buttons or the tabs. Without the app reading the mouse, the
// terminal sends ↑ and ↓ for the wheel, several per step, so the cursor
// jumped rows and, at the top, left the panel.
//
// The mouse is read with tea.WithMouseCellMotion (internal/app); while it
// is, most terminals leave selecting text to shift+drag.

// Where things are on the screen, from the top, as View draws them:
//
//	0  tabs
//	1  (margin)
//	2  ┌─ buttons, or the page's title ─┐
//	3  │                                 │   (padding)
//	4  │ first row on screen             │
//	…
const (
	tabsY      = 0
	topBorderY = 1 + panelMarginY
	firstRowY  = topBorderY + 1 + panelPaddingTop
)

// panelButtonsX is the column of the first button: after "┌─".
const panelButtonsX = 2

// wheelStep is the rows one step of the wheel moves: down is +1, up -1,
// anything else 0.
func wheelStep(msg tea.MouseMsg) int {
	if msg.Action != tea.MouseActionPress {
		return 0
	}
	switch msg.Button {
	case tea.MouseButtonWheelDown:
		return +1
	case tea.MouseButtonWheelUp:
		return -1
	}
	return 0
}

// onMouse handles a mouse event.
func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.splash || m.width == 0 {
		return m, nil // nothing on screen to point at yet
	}
	if step := wheelStep(msg); step != 0 {
		return m.onWheel(step)
	}
	if msg.Action != tea.MouseActionPress {
		return m, nil // releases and drags
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		m.notice = "" // like a key press, a click clears the last action's message
		return m.onClick(msg.X, msg.Y)
	case tea.MouseButtonRight:
		m.notice = ""
		return m.back()
	}
	return m, nil
}

// onWheel scrolls the panel by step rows.
func (m Model) onWheel(step int) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayNone:
		m.searching = false // stop typing the search; what was typed stays
		m.focus = focusList
		return m.moveCursor(step), nil
	case overlayText:
		m.textOffset = max(min(m.textOffset+step, len(m.text)-m.listHeight()), 0)
		return m, nil
	}
	if m.typing() {
		return m, nil // the wheel doesn't leave the text field
	}
	// The other pages move their cursor the way ↑ and ↓ do.
	k := tea.KeyMsg{Type: tea.KeyDown}
	if step < 0 {
		k = tea.KeyMsg{Type: tea.KeyUp}
	}
	return m.onOverlayKey(k)
}

// back is esc: a page goes back to the one before, a search is cleared.
func (m Model) back() (tea.Model, tea.Cmd) {
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	switch {
	case m.overlay != overlayNone:
		return m.onOverlayKey(esc)
	case m.searching:
		return m.onSearchKey(esc)
	}
	return m, nil
}

// onClick handles a left click at column x, row y of the screen.
func (m Model) onClick(x, y int) (tea.Model, tea.Cmd) {
	m.searching = false // a click ends typing the search; what was typed stays

	if y == tabsY {
		i := m.tabAt(x)
		if i < 0 {
			return m, nil
		}
		m = m.closeOverlay()
		m.focus = focusTabs
		if i == m.active {
			return m, nil
		}
		return m.switchTab(i - m.active)
	}

	if y == topBorderY && m.overlay == overlayNone && m.buttonsShown() {
		if i := m.buttonAt(x); i >= 0 {
			m.focus = focusButtons
			m.button = i
			return m.pressButton()
		}
		return m, nil
	}

	if i := m.rowAt(y); i >= 0 {
		return m.clickRow(i)
	}
	return m, nil
}

// tabAt is the index of the tab at column x of the tab bar, or -1.
func (m Model) tabAt(x int) int {
	left := 0
	for i, t := range m.tabs {
		w := lipgloss.Width(InactiveTab.Render(t.name)) // the active tab is as wide
		if x >= left && x < left+w {
			return i
		}
		left += w + tabGap
	}
	return -1
}

// buttonAt is the index of the button at column x of the top border, or -1.
func (m Model) buttonAt(x int) int {
	left := panelButtonsX
	for i, b := range m.panelButtons() {
		w := lipgloss.Width(b.render(false))
		if x >= left && x < left+w {
			return i
		}
		left += w + buttonGap
	}
	return -1
}

// rowAt is the index, in what the panel shows (content), of the row at
// line y of the screen, or -1 if y isn't on one.
func (m Model) rowAt(y int) int {
	c := m.content()
	first, end := visibleRange(c.offset, len(c.rows), m.listHeight())
	i := first + y - firstRowY
	if y < firstRowY || i >= end {
		return -1
	}
	return i
}

// clickRow handles a click on row i of what the panel shows.
func (m Model) clickRow(i int) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayNone:
		// The selected row clicked again opens its actions, like enter.
		if m.focus == focusList && m.cursor == i && m.canOpenActions() {
			return m.openActions(), nil
		}
		m.focus = focusList
		m.cursor = i
		return m.keepCursorVisible(), nil

	case overlayActions:
		return m.pick(i)

	case overlayNamespaces:
		return m.pickNamespace(i) // does nothing while they load

	case overlayContainers:
		m.overlayCursor = i
		return m.onContainersKey(KeyEnter)

	case overlayConfirm:
		return m.clickConfirm(i)
	}
	return m, nil // a page of text: nothing to click
}

// clickConfirm handles a click on row i of the confirm page (confirmRows):
//
//	0  what the action does
//	1
//	2  the text field          → the cursor goes there, to type
//	3
//	   [ ] a flag              → ticked or unticked
//	   …
//
//	   y run • n back          → runs it
func (m Model) clickConfirm(i int) (tea.Model, tea.Cmd) {
	rows, _ := m.confirmRows()
	fieldRows := m.fieldRows()
	firstFlag := 2 + 2*fieldRows // the field and the empty row under it

	switch {
	case fieldRows == 1 && i == 2:
		m.overlayCursor = 0
	case i >= firstFlag && i < firstFlag+len(m.flags):
		f := i - firstFlag
		m.overlayCursor = fieldRows + f
		m.flags[f].On = !m.flags[f].On
	case i == len(rows)-1:
		return m.runChosen()
	}
	return m, nil
}
