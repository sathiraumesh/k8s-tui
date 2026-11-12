package mvc

import (
	"time"

	"k8s-tui/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Auto refresh: while it is on, a tick fires every refreshEvery; on each tick
// Update reloads the active tab and schedules the next tick.
//
//	Init ──▶ tick(5s) ──▶ refreshTickMsg ──▶ Update ──▶ loadActive + tick(5s) ──▶ ...
//
// Turning it off simply stops scheduling the next tick.

// tick returns a command that sends one refreshTickMsg after d.
func tick(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return refreshTickMsg{gen: gen} })
}

// startTimer returns the command for the next tick of the current timer.
func (m Model) startTimer() tea.Cmd {
	return tick(m.refreshEvery, m.tickGen)
}

// nextRate returns the rate after d in refreshRates, wrapping 10s → 2s.
func nextRate(d time.Duration) time.Duration {
	for i, r := range refreshRates {
		if r == d {
			return refreshRates[(i+1)%len(refreshRates)]
		}
	}
	return refreshRates[0]
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

// Styles for the status line.
var (
	// statusOn is the "● on" badge while auto refresh is running.
	statusOn = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)

	// rateActive highlights the chosen rate, like the active tab.
	rateActive = lipgloss.NewStyle().
			Foreground(styles.OnAccent).
			Background(styles.Accent).
			Padding(0, 1)

	// rateInactive is a rate that is not chosen; same padding so nothing shifts.
	rateInactive = lipgloss.NewStyle().Foreground(styles.Muted).Padding(0, 1)
)

// statusLine is the line under the panel, e.g.
//
//	auto refresh ● on   2s [5s] 10s   updated 14:30:05
//
// The chosen rate is highlighted. "updated" is left out until the active
// tab has loaded once.
func (m Model) statusLine() string {
	state := Help.Render("○ off")
	if m.autoRefresh {
		state = statusOn.Render("● on")
	}

	var rates []string
	for _, r := range refreshRates {
		if r == m.refreshEvery {
			rates = append(rates, rateActive.Render(r.String()))
		} else {
			rates = append(rates, rateInactive.Render(r.String()))
		}
	}

	parts := []string{
		Help.Render("auto refresh "), state, "   ",
		lipgloss.JoinHorizontal(lipgloss.Top, rates...),
	}
	if at, ok := m.updated[m.tabs[m.active]]; ok {
		parts = append(parts, Help.Render("   updated "+at.Format("15:04:05")))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
