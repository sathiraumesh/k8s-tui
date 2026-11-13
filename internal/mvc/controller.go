package mvc

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m = m.keepCursorVisible() // a shorter window shows fewer rows

	// Time to refresh: reload the active tab and schedule the next tick.
	// Drop ticks from an old timer, and stop when auto refresh is off.
	case refreshTickMsg:
		if msg.gen != m.tickGen || m.refreshEvery == refreshOff {
			return m, nil
		}
		// On a tab that isn't auto refreshed (More) the timer keeps running,
		// so refreshing starts again on the next tab that is.
		return m, tea.Batch(m.refreshActive(), m.startTimer())

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

		// A refresh can return fewer rows (a node was removed, or fewer
		// match the search): keep the cursor on a row that still exists.
		if msg.tab == m.activeName() {
			if n := len(m.rows()); m.cursor >= n {
				m.cursor = max(n-1, 0)
			}
			m = m.keepCursorVisible()
		}

	// Time for the next picture of the start-up screen (splash.go).
	case splashTickMsg:
		return m.onSplashTick()

	// An action finished (actions.go).
	case actionDoneMsg:
		return m.onActionDone(msg)

	case tea.KeyMsg:
		if m.splash {
			// Nothing is on screen to act on yet; only quitting works.
			if k := msg.String(); k == KeyQuit || k == KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}
		m.notice = "" // a key press clears the last action's message
		if m.overlay != overlayNone {
			return m.onOverlayKey(msg)
		}
		if m.searching {
			return m.onSearchKey(msg) // keys type the search (search.go)
		}
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
	switch msg.String() {
	case KeyQuit, KeyCtrlC:
		return m, tea.Quit

	// r: step the refresh rate from anywhere, same as pressing the button.
	case KeyRefreshRate:
		return m.stepRefresh()

	// w: wide columns on/off from anywhere, same as pressing the button.
	case KeyWide:
		return m.toggleWide()

	// /: search the list, same as pressing the button (search.go).
	case KeySearch:
		return m.startSearch(), nil

	// +: the tab's own actions (e.g. run a pod), same as its "+ run" button.
	case KeyOwn:
		return m.openOwnActions(), nil

	// esc: clear the search, if there is one.
	case KeyEsc:
		if m.searchText() != "" {
			return m.clearSearch(), nil
		}

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
	// On the list, enter opens the row's actions (actions.go).
	case KeyEnter, KeySpace:
		if m.focus == focusButtons {
			return m.pressButton()
		}
		if msg.String() == KeyEnter && m.focus == focusList && m.canOpenActions() {
			return m.openActions(), nil
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
			m = m.moveCursor(+1) // scrolls when it reaches the bottom
		}

	// ↑ : up the list; from its top row to the buttons, then to the tabs.
	case KeyUp, KeyK:
		switch m.focus {
		case focusList:
			if m.cursor > 0 {
				m = m.moveCursor(-1) // scrolls when it reaches the top
			} else {
				m.focus = m.aboveList()
			}
		case focusButtons:
			m.focus = focusTabs
		}

	// pgdown / pgup: a page (the rows that fit in the panel) down or up the list.
	case KeyPageDown:
		if m.focus == focusList {
			m = m.moveCursor(+m.listHeight())
		}
	case KeyPageUp:
		if m.focus == focusList {
			m = m.moveCursor(-m.listHeight())
		}
	}
	return m, nil
}

// switchTab moves to the tab step places along (wrapping), and loads it
// right away instead of waiting for the next tick. The new tab starts at
// its first row, scrolled to the top.
func (m Model) switchTab(step int) (tea.Model, tea.Cmd) {
	m.active = (m.active + step + len(m.tabs)) % len(m.tabs)
	m.cursor, m.offset = 0, 0

	// Tabs have different buttons (More has no refresh button): keep the
	// button cursor on one that exists.
	m.button = min(m.button, len(m.panelButtons())-1)

	// A tab that isn't auto refreshed is loaded the first time it is opened.
	if _, loaded := m.updated[m.activeName()]; !m.autoRefreshed() && loaded {
		return m, nil
	}
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
	case buttonSearch:
		return m.startSearch(), nil
	case buttonOwn:
		return m.openOwnActions(), nil
	}
	return m, nil
}

// toggleWide turns the active tab's wide columns on or off, and reloads the
// tab so its rows get the new columns. The rows on screen stay until then.
func (m Model) toggleWide() (tea.Model, tea.Cmd) {
	tab := m.activeName()
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
