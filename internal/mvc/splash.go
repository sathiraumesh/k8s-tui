package mvc

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The start-up screen: the app's name in big letters with a light sweeping
// across them, shown while the first rows load (how it is drawn is in
// splash_view.go). A timer moves the animation on every splashInterval;
// each tick also checks whether the screen can close.
//
//	start ──▶ splash ──tick, tick, …──▶ the nodes have loaded (or failed)
//	                                    and minSplashFrames were shown ──▶ tabs

// splashInterval is the time between two pictures of the animation.
const splashInterval = 80 * time.Millisecond

// minSplashFrames is the fewest pictures the start-up screen shows. Without
// it a cluster that answers at once would make it flash by too fast to see.
const minSplashFrames = 15 // 15 × 80ms = 1.2s

// splashTickMsg is sent when it is time for the next picture.
type splashTickMsg struct{}

// WithSplash returns m showing the start-up screen until its first tab has
// loaded. New doesn't show it, so tests start straight on the tabs.
func (m Model) WithSplash() Model {
	m.splash = true
	m.splashFrame = 0
	return m
}

// splashTick waits one splashInterval, then sends a splashTickMsg. It is nil
// once the start-up screen has closed, which ends the animation.
func (m Model) splashTick() tea.Cmd {
	if !m.splash {
		return nil
	}
	return tea.Tick(splashInterval, func(time.Time) tea.Msg { return splashTickMsg{} })
}

// onSplashTick moves the animation on by one picture, and closes the
// start-up screen once the active tab has answered, with rows or an error.
func (m Model) onSplashTick() (Model, tea.Cmd) {
	if !m.splash {
		return m, nil
	}
	m.splashFrame++
	_, loaded := m.updated[m.activeName()]
	if (loaded || m.err != nil) && m.splashFrame >= minSplashFrames {
		m.splash = false
	}
	return m, m.splashTick()
}
