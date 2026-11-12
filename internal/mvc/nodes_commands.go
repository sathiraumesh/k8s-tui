package mvc

import (
	"context"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for the Nodes tab. A command is slow work (an API call):
// Bubble Tea runs it in the background so the screen never freezes,
// then sends the message it returns (see messages.go) to Update.

// loadNodes calls the API for the nodes (like `kubectl get nodes`, or with
// wide, `kubectl get nodes -o wide`).
func loadNodes(c *k8s.Client, wide bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()

		nodes, err := c.ListNodes(ctx)
		return itemsLoadedMsg{tab: tabNodes, rows: nodeRows(nodes, wide), err: err, at: time.Now(), wide: wide}
	}
}

// columnGap is the number of spaces between two columns of a row.
const columnGap = 3

// nodeRows turns each node into one line of text, with its columns lined up:
//
//	NAME  STATUS  VERSION
//
// With wide, the columns `kubectl get nodes -o wide` adds come after:
//
//	NAME  STATUS  VERSION  INTERNAL-IP  EXTERNAL-IP  OS-IMAGE  KERNEL-VERSION  CONTAINER-RUNTIME
func nodeRows(nodes []k8s.Node, wide bool) []string {
	cells := make([][]string, 0, len(nodes))
	for _, n := range nodes {
		row := []string{n.Name, n.Status, n.Version}
		if wide {
			row = append(row, n.InternalIP, orNone(n.ExternalIP), n.OSImage, n.KernelVersion, n.Runtime)
		}
		cells = append(cells, row)
	}
	return alignColumns(cells)
}

// orNone returns s, or "<none>" if it is empty, as kubectl shows it.
func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// alignColumns joins each row's cells into one line, padding every column
// to its widest cell so the columns line up. The last column is not padded.
func alignColumns(rows [][]string) []string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(cell))
		}
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)+columnGap))
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}
