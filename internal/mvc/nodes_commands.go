package mvc

import (
	"context"
	"fmt"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for the Nodes tab. A command is slow work (an API call):
// Bubble Tea runs it in the background so the screen never freezes,
// then sends the message it returns (see messages.go) to Update.

// loadNodes calls the API for the nodes (like `kubectl get nodes`).
func loadNodes(c *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()

		nodes, err := c.ListNodes(ctx)
		return itemsLoadedMsg{tab: "Nodes", rows: nodeRows(nodes), err: err}
	}
}

// nodeRows turns each node into one line of text: NAME  STATUS  VERSION.
func nodeRows(nodes []k8s.Node) []string {
	rows := make([]string, 0, len(nodes))
	for _, n := range nodes {
		rows = append(rows, fmt.Sprintf("%-30s %-10s %s", n.Name, n.Status, n.Version))
	}
	return rows
}
