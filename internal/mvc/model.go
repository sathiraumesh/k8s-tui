package mvc

import (
	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// focus says which part of the screen the cursor is on.
type focus int

const (
	focusTabs focus = iota // 0: cursor is on the tab bar
	focusList              // 1: cursor is on the list in the panel
)

// Model is everything the app remembers between key presses.
// Update changes it; View reads it to draw the screen.
type Model struct {
	// tabs are the names shown in the tab bar, in order.
	// e.g. ["Nodes", "Pods", "Deployments", "Services"]
	tabs []string

	// items holds the rows to list for each tab, keyed by tab name.
	// e.g. items["Pods"] = ["nginx-...", "redis-0"]
	// Placeholder data for now; later this comes from the cluster.
	items map[string][]string

	// active is the index of the selected tab in tabs.
	// 0 = "Nodes", 1 = "Pods", 2 = "Deployments", 3 = "Services".
	// Changed by tab / ← / →.
	active int

	// cursor is the index of the highlighted row in the panel's list.
	// Changed by ↑ / ↓, and reset to 0 when switching tabs.
	cursor int

	// width and height are the terminal size in cells (columns and rows).
	// Bubble Tea sends them in a tea.WindowSizeMsg at start and on every resize.
	// They are 0 until that first message arrives, so View shows "loading...".
	width  int
	height int

	// focus is where the cursor is: the tab bar or the list.
	// Starts as focusTabs (0). Changed by tab, ↓ from the tabs,
	// and ↑ from the top row of the list.
	focus focus

	// client talks to the cluster. nil in tests, so nothing is loaded there.
	client *k8s.Client

	// err is the last error from the cluster, shown in the panel. nil when all is fine.
	err error
}

// New returns a Model with its starting state.
// client may be nil (in tests); then nothing is loaded from a cluster.
func New(client *k8s.Client) Model {
	return Model{
		client: client,
		tabs:   []string{"Nodes", "Pods", "Deployments", "Services"},
		items: map[string][]string{
			"Nodes":       {"loading..."}, // replaced when the nodes arrive
			"Pods":        {"nginx-7c5ddbdf54-abcde", "redis-0", "api-6f9d8b7c9-xyz12"},
			"Deployments": {"nginx", "api"},
			"Services":    {"kubernetes", "nginx-svc", "redis"},
		},
	}
}

// Init runs once at start: begin loading the nodes from the cluster.
func (m Model) Init() tea.Cmd {
	if m.client == nil {
		return nil // tests: no cluster
	}
	return loadNodes(m.client)
}
