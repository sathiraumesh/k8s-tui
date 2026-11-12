package mvc

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Auto refresh: while it is on, a tick fires every refreshEvery; on each tick
// Update reloads the active tab and schedules the next tick.
//
//	Init ──▶ tick(5s) ──▶ refreshTickMsg ──▶ Update ──▶ loadActive + tick(5s) ──▶ ...
//
// Setting the rate to off (0) simply stops scheduling the next tick.
// How the refresh state is drawn is in refresh_view.go.

// tick returns a command that sends one refreshTickMsg after d.
func tick(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return refreshTickMsg{gen: gen} })
}

// startTimer returns the command for the next tick of the current timer,
// or nil when auto refresh is off.
func (m Model) startTimer() tea.Cmd {
	if m.refreshEvery == refreshOff {
		return nil
	}
	return tick(m.refreshEvery, m.tickGen)
}

// nextRate returns the rate after d in refreshRates: off → 2s → 5s → 10s → off.
func nextRate(d time.Duration) time.Duration {
	for i, r := range refreshRates {
		if r == d {
			return refreshRates[(i+1)%len(refreshRates)]
		}
	}
	return refreshRates[0]
}

// stepRefresh moves to the next rate and starts a new timer for it, so the
// change takes effect now. Coming from off, it also reloads straight away.
func (m Model) stepRefresh() (Model, tea.Cmd) {
	wasOff := m.refreshEvery == refreshOff
	m.refreshEvery = nextRate(m.refreshEvery)
	m.tickGen++ // any tick still on its way is from the old timer
	if wasOff {
		return m, tea.Batch(m.loadActive(), m.startTimer())
	}
	return m, m.startTimer()
}

// loadActive returns the command that loads the active tab from the cluster,
// or nil if there is no client (tests) or the tab has no loader yet.
func (m Model) loadActive() tea.Cmd {
	if m.client == nil {
		return nil
	}
	switch m.tabs[m.active] {
	case "Nodes":
		return loadNodes(m.client)
	}
	return nil // Pods, Deployments, Services: placeholder data for now
}
