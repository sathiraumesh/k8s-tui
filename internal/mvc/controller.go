package mvc

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	// Time to refresh: reload the active tab and schedule the next tick.
	// Drop ticks from an old timer, and stop when auto refresh is off.
	case refreshTickMsg:
		if msg.gen != m.tickGen || m.refreshEvery == refreshOff {
			return m, nil
		}
		return m, tea.Batch(m.loadActive(), m.startTimer())

	// A tab's data arrived from the cluster (see nodes_commands.go).
	case itemsLoadedMsg:
		// Rows with the wrong columns: wide was switched while they were on
		// their way. The load that switch started brings the right ones.
		if msg.wide != m.wide[msg.tab] {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.items[msg.tab] = msg.rows
		m.updated[msg.tab] = msg.at

		// A refresh can return fewer rows (a node was removed):
		// keep the cursor on a row that still exists.
		if msg.tab == m.tabs[m.active] && m.cursor >= len(msg.rows) {
			m.cursor = max(len(msg.rows)-1, 0)
		}

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// onKey handles a key press. Where the cursor is (m.focus) decides what the
// arrow keys do:
//
//	tabs     ← → switch tab     ↓ buttons (or list)
//	buttons  ← → move button    ↑ tabs   ↓ list   enter/space press
//	list     ↑ ↓ move row       ↑ on the top row: buttons (or tabs)
func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.items[m.tabs[m.active]]

	switch msg.String() {
	case KeyQuit, KeyCtrlC:
		return m, tea.Quit

	// r: step the refresh rate from anywhere, same as pressing the button.
	case KeyRefreshRate:
		return m.stepRefresh()

	// w: wide columns on/off from anywhere, same as pressing the button.
	case KeyWide:
		return m.toggleWide()

	// tab: step the focus tabs → buttons → list → tabs.
	case KeyTab:
		switch m.focus {
		case focusTabs:
			m.focus = m.belowTabs()
		case focusButtons:
			m.focus = focusList
		default:
			m.focus = focusTabs
		}

	// enter / space: press the button the cursor is on.
	case KeyEnter, KeySpace:
		if m.focus == focusButtons {
			return m.pressButton()
		}

	// → : next button on the buttons, otherwise next tab.
	case KeyRight, KeyL:
		if m.focus == focusButtons {
			m.button = min(m.button+1, len(m.panelButtons())-1)
			return m, nil
		}
		return m.switchTab(+1)

	// ← : previous button on the buttons, otherwise previous tab.
	case KeyLeft, KeyH:
		if m.focus == focusButtons {
			m.button = max(m.button-1, 0)
			return m, nil
		}
		return m.switchTab(-1)

	// shift+tab: always the previous tab.
	case KeyShiftTab:
		return m.switchTab(-1)

	// ↓ : tabs → buttons → list, then down the list.
	case KeyDown, KeyJ:
		switch m.focus {
		case focusTabs:
			m.focus = m.belowTabs()
		case focusButtons:
			m.focus = focusList
		default:
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		}

	// ↑ : up the list; from its top row to the buttons, then to the tabs.
	case KeyUp, KeyK:
		switch m.focus {
		case focusList:
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.focus = m.aboveList()
			}
		case focusButtons:
			m.focus = focusTabs
		}
	}
	return m, nil
}

// switchTab moves to the tab step places along (wrapping), and loads it
// right away instead of waiting for the next tick.
func (m Model) switchTab(step int) (tea.Model, tea.Cmd) {
	m.active = (m.active + step + len(m.tabs)) % len(m.tabs)
	m.cursor = 0
	return m, m.loadActive()
}

// pressButton does what the button the cursor is on does.
func (m Model) pressButton() (tea.Model, tea.Cmd) {
	buttons := m.panelButtons()
	if m.button < 0 || m.button >= len(buttons) {
		return m, nil
	}
	switch buttons[m.button].name {
	case buttonWide:
		return m.toggleWide()
	case buttonRefresh:
		return m.stepRefresh()
	}
	return m, nil
}

// toggleWide turns the active tab's wide columns on or off, and reloads the
// tab so its rows get the new columns. The rows on screen stay until then.
func (m Model) toggleWide() (tea.Model, tea.Cmd) {
	tab := m.tabs[m.active]
	m.wide[tab] = !m.wide[tab]
	return m, m.loadActive()
}

// belowTabs is where the focus goes coming down from the tabs: the buttons
// if they are shown, otherwise straight to the list.
func (m Model) belowTabs() focus {
	if m.buttonsShown() {
		return focusButtons
	}
	return focusList
}

// aboveList is where the focus goes leaving the list upwards: the buttons if
// they are shown, otherwise the tabs.
func (m Model) aboveList() focus {
	if m.buttonsShown() {
		return focusButtons
	}
	return focusTabs
}
