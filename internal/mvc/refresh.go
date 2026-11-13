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
// How the refresh state is drawn is in secondary_bar_view.go.

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
		return m, tea.Batch(m.refreshActive(), m.startTimer())
	}
	return m, m.startTimer()
}

// autoRefreshed reports whether the active tab is reloaded by the timer
// (its autoRefresh in allTabs). More is not: the resource types a cluster
// serves rarely change, so it is loaded once.
func (m Model) autoRefreshed() bool {
	return m.activeTab().autoRefresh
}

// refreshActive is loadActive for the timer: nil on a tab that is not
// auto refreshed.
func (m Model) refreshActive() tea.Cmd {
	if !m.autoRefreshed() {
		return nil
	}
	return m.loadActive()
}

// loadActive returns the command that loads the active tab with its loader
// from allTabs, or nil if there is no client (tests).
func (m Model) loadActive() tea.Cmd {
	if m.client == nil {
		return nil
	}
	t := m.activeTab()
	return t.load(m.client, m.wide[t.name], m.tabNamespace())
}
