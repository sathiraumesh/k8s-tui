package mvc

// Scrolling the list. The panel shows listHeight() rows, starting at
// m.offset; Update moves the offset whenever the cursor would leave the
// rows on screen. How the scroll position is drawn is in scroll_view.go.
//
//	rows:  0 1 2 3 4 5 6 7 8 9
//	           └─ on screen ─┘      offset = 2, listHeight = 5
//	                   ▲ cursor = 5

// listHeight is how many rows of the list fit in the panel: the terminal's
// height minus the tab bar, the help line, the margins, the panel's top
// and bottom border, and the empty row under the top border. It is at
// least 1.
func (m Model) listHeight() int {
	return max(m.height-2-2*panelMarginY-2-panelPaddingTop, 1)
}

// keepCursorVisible moves the offset as little as possible so the cursor's
// row is on screen, and so the screen is never scrolled past the end of the
// list (no empty rows at the bottom while there are rows above).
func (m Model) keepCursorVisible() Model {
	h := m.listHeight()
	total := len(m.rows())

	if m.cursor < m.offset {
		m.offset = m.cursor // cursor went above the top: scroll up to it
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1 // cursor went below the bottom: scroll down to it
	}
	m.offset = max(min(m.offset, total-h), 0)
	return m
}

// moveCursor moves the list's cursor by step rows (negative is up), stopping
// at the first and last row, and scrolls so it stays on screen.
func (m Model) moveCursor(step int) Model {
	last := len(m.rows()) - 1
	m.cursor = max(min(m.cursor+step, last), 0)
	return m.keepCursorVisible()
}

// visibleRange returns the index of the first row on screen and the one
// after the last, for a list of total rows scrolled to offset, with room
// for h rows. offset is kept inside the list, so a stale offset (the list
// got shorter) still gives a valid range.
func visibleRange(offset, total, h int) (first, end int) {
	first = max(min(offset, total-h), 0)
	end = min(first+h, total)
	return first, end
}
