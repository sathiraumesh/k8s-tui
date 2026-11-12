package mvc

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		rows := m.items[m.tabs[m.active]]
		switch msg.String() {
		case KeyQuit, KeyCtrlC:
			return m, tea.Quit
		case KeyTab, KeyRight, KeyL:
			m.active = (m.active + 1) % len(m.tabs)
			m.cursor = 0
		case KeyShiftTab, KeyLeft, KeyH:
			m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
			m.cursor = 0
		case KeyDown, KeyJ:
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		case KeyUp, KeyK:
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}
