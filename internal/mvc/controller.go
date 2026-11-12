package mvc

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	// Time to refresh: reload the active tab and schedule the next tick.
	// Drop ticks from an old timer, and stop when auto refresh is off.
	case refreshTickMsg:
		if msg.gen != m.tickGen || !m.autoRefresh {
			return m, nil
		}
		return m, tea.Batch(m.loadActive(), m.startTimer())

	// A tab's data arrived from the cluster (see nodes_commands.go).
	case itemsLoadedMsg:
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
		rows := m.items[m.tabs[m.active]]
		switch msg.String() {
		case KeyQuit, KeyCtrlC:
			return m, tea.Quit

		// a: turn auto refresh on/off. Turning it on reloads now and starts a new timer.
		case KeyAutoRefresh:
			m.autoRefresh = !m.autoRefresh
			m.tickGen++ // any tick still on its way is from the old timer
			if m.autoRefresh {
				return m, tea.Batch(m.loadActive(), m.startTimer())
			}

		// f: next rate (2s → 5s → 10s → 2s). Restart the timer so it takes effect now.
		case KeyRefreshRate:
			m.refreshEvery = nextRate(m.refreshEvery)
			m.tickGen++
			if m.autoRefresh {
				return m, m.startTimer()
			}

		// tab: jump between the tab bar and the list
		case KeyTab:
			if m.focus == focusTabs {
				m.focus = focusList
			} else {
				m.focus = focusTabs
			}

		// ← →: switch tabs (works from anywhere)
		// Load the new tab right away instead of waiting for the next tick.
		case KeyRight, KeyL:
			m.active = (m.active + 1) % len(m.tabs)
			m.cursor = 0
			return m, m.loadActive()
		case KeyShiftTab, KeyLeft, KeyH:
			m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
			m.cursor = 0
			return m, m.loadActive()

		// ↓: from the tabs go into the list, in the list move down
		case KeyDown, KeyJ:
			if m.focus == focusTabs {
				m.focus = focusList
			} else if m.cursor < len(rows)-1 {
				m.cursor++
			}

		// ↑: in the list move up, at the top row go back to the tabs
		case KeyUp, KeyK:
			if m.focus == focusList {
				if m.cursor > 0 {
					m.cursor--
				} else {
					m.focus = focusTabs
				}
			}
		}

	}
	return m, nil
}
