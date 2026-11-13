package mvc

import (
	"context"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// The namespace the namespaced tabs (Pods, Deployments, Services) show.
// It is one setting for all of them, like kubectl's -n, picked from the
// namespaces of the cluster:
//
//	n / ns button ──▶ picker (namespaces load) ──enter──▶ that namespace,
//	                       │                              the tab reloads
//	                       └──esc──▶ back, nothing changed
//
// A tab that isn't namespaced (Nodes, More) has no ns button and always
// loads everything.

// allNamespaces is the namespace setting for every namespace, like
// `kubectl get pods --all-namespaces`.
const allNamespaces = ""

// allNamespacesChoice is the first choice of the picker.
const allNamespacesChoice = "all namespaces"

// namespacesLoadedMsg brings the cluster's namespaces for the picker.
type namespacesLoadedMsg struct {
	names []string
	err   error
}

// loadNamespaces calls the API for the namespaces, like
// `kubectl get namespaces`.
func loadNamespaces(c *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		names, err := c.ListNamespaces(ctx)
		return namespacesLoadedMsg{names: names, err: err}
	}
}

// tabNamespace is the namespace the active tab loads: the one picked if it
// is namespaced, else all.
func (m Model) tabNamespace() string {
	if m.activeTab().namespaced {
		return m.namespace
	}
	return allNamespaces
}

// tabNamed is the tab called name.
func (m Model) tabNamed(name string) (tab, bool) {
	for _, t := range m.tabs {
		if t.name == name {
			return t, true
		}
	}
	return tab{}, false
}

// openNamespaces opens the picker, and loads the namespaces into it.
func (m Model) openNamespaces() (Model, tea.Cmd) {
	m.overlay = overlayNamespaces
	m.overlayCursor = 0
	m.focus = focusList
	m.namespaces, m.namespacesErr = nil, nil
	if m.client == nil {
		return m, nil // tests without a cluster
	}
	return m, loadNamespaces(m.client)
}

// onNamespacesLoaded fills the picker, with the cursor on the namespace
// picked now.
func (m Model) onNamespacesLoaded(msg namespacesLoadedMsg) Model {
	m.namespaces, m.namespacesErr = msg.names, msg.err
	m.overlayCursor = 0
	for i, name := range m.namespaces {
		if name == m.namespace {
			m.overlayCursor = i + 1 // after "all namespaces"
		}
	}
	return m
}

// namespaceChoices are what the picker offers: all namespaces, then each
// one. Before they have loaded there are none.
func (m Model) namespaceChoices() []string {
	if m.namespaces == nil {
		return nil
	}
	return append([]string{allNamespacesChoice}, m.namespaces...)
}

// pickNamespace makes choice i the namespace, and reloads the tab from it.
// The rows of the namespaced tabs are from the old one, so they show
// "loading..." until each is loaded again.
func (m Model) pickNamespace(i int) (Model, tea.Cmd) {
	choices := m.namespaceChoices()
	if i < 0 || i >= len(choices) {
		return m, nil
	}
	m.namespace = allNamespaces
	if i > 0 {
		m.namespace = choices[i]
	}
	for _, t := range m.tabs {
		if t.namespaced {
			m.items[t.name] = []string{"loading..."}
			delete(m.updated, t.name)
		}
	}
	m.err = nil
	m.cursor, m.offset = 0, 0
	m = m.closeOverlay()
	return m, m.loadActive()
}

// onNamespacesKey handles a key in the picker.
func (m Model) onNamespacesKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case KeyUp, KeyK:
		m.overlayCursor = max(m.overlayCursor-1, 0)
	case KeyDown, KeyJ:
		m.overlayCursor = min(m.overlayCursor+1, max(len(m.namespaceChoices())-1, 0))
	case KeyEnter:
		return m.pickNamespace(m.overlayCursor)
	case KeyEsc:
		return m.closeOverlay(), nil
	}
	return m, nil
}

// namespaceRows are the picker's rows, the picked one marked:
//
//	  all namespaces
//	✓ default
//	  kube-system
func (m Model) namespaceRows() (rows []string, cursor int) {
	switch {
	case m.namespacesErr != nil:
		return []string{"error: " + m.namespacesErr.Error()}, -1
	case m.namespaces == nil:
		return []string{"loading..."}, -1
	}
	for i, choice := range m.namespaceChoices() {
		mark := "  "
		if (i == 0 && m.namespace == allNamespaces) || (i > 0 && choice == m.namespace) {
			mark = "✓ "
		}
		rows = append(rows, mark+choice)
	}
	return rows, m.overlayCursor
}
