package mvc

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	// A tab's data arrived from the cluster (see commands.go).
	case itemsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.items[msg.tab] = msg.rows

	case tea.KeyMsg:
		rows := m.items[m.tabs[m.active]]
		switch msg.String() {
		case KeyQuit, KeyCtrlC:
			return m, tea.Quit

		// tab: jump between the tab bar and the list
		case KeyTab:
			if m.focus == focusTabs {
				m.focus = focusList
			} else {
				m.focus = focusTabs
			}

		// ← →: switch tabs (works from anywhere)
		case KeyRight, KeyL:
			m.active = (m.active + 1) % len(m.tabs)
			m.cursor = 0
		case KeyShiftTab, KeyLeft, KeyH:
			m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
			m.cursor = 0

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
