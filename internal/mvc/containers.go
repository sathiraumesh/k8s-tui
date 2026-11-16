package mvc

import (
	"context"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	tea "github.com/charmbracelet/bubbletea"
)

// Which container an action is for, when the pod has more than one. An
// action with containers set, such as Logs, loads the pod's containers when
// it is picked:
//
//	one container ──▶ confirm page / run, as before
//	several       ──▶ picker: "all containers", then each ──enter──▶ confirm page / run
//
// The choice goes to the action as its input: a container's name, or
// allContainers.

// containersLoadedMsg brings the containers of the pod to pick from.
type containersLoadedMsg struct {
	target string // "NAMESPACE/NAME", to ignore a late answer for another pod
	names  []string
	err    error
}

// loadContainers calls the API for the containers of the pod target.
func loadContainers(c *k8s.Client, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		namespace, name := commands.SplitTarget(target)
		names, err := c.PodContainers(ctx, namespace, name)
		return containersLoadedMsg{target: target, names: names, err: err}
	}
}

// onContainersLoaded opens the picker if the pod has several containers.
// With one the action goes on at once, as it would for any pod; if they
// can't be loaded, the action also goes on, and says why if it fails.
func (m Model) onContainersLoaded(msg containersLoadedMsg) (Model, tea.Cmd) {
	if m.overlay != overlayActions || msg.target != m.target {
		return m, nil
	}
	if msg.err != nil || len(msg.names) < 2 {
		m.input = commands.AllContainers
		return m.afterPick()
	}
	m.containers = msg.names
	m.overlay = overlayContainers
	m.overlayCursor = 0
	return m, nil
}

// containerChoices are what the picker offers: all containers, then each.
func (m Model) containerChoices() []string {
	return append([]string{commands.AllContainers}, m.containers...)
}

// onContainersKey handles a key in the picker.
func (m Model) onContainersKey(key string) (tea.Model, tea.Cmd) {
	choices := m.containerChoices()
	switch key {
	case KeyUp, KeyK:
		m.overlayCursor = max(m.overlayCursor-1, 0)
	case KeyDown, KeyJ:
		m.overlayCursor = min(m.overlayCursor+1, len(choices)-1)
	case KeyEnter:
		m.input = choices[m.overlayCursor]
		return m.afterPick()
	case KeyEsc:
		m.overlay = overlayActions
		m.overlayCursor = m.chosenAction
	}
	return m, nil
}

// containerRows are the picker's rows.
func (m Model) containerRows() []string {
	rows := m.containerChoices()
	rows[0] = "all containers"
	return rows
}
