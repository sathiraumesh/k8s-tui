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

// ---------- actions ----------

// flagNoEvents leaves the events out of Describe.
const flagNoEvents = "--show-events=false"

// flagOverwrite lets Label and Taint replace a label's value, or a taint
// with the same key and effect, that the node already has.
const flagOverwrite = "--overwrite"

// nodeActions are the actions of the Nodes tab: enter on a node opens them.
// The cluster calls they make are in internal/kubernetes/nodes.go.
func nodeActions() []action {
	return []action{
		{
			key: "d", name: "Describe", about: "everything about the node, like kubectl describe",
			flags:   []flag{{name: flagNoEvents}},
			command: "kubectl describe node %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				return read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribeNode(ctx, target, !ticked(flags, flagNoEvents))
				})
			},
		},
		{
			key: "s", name: "Show", about: "the node's labels and taints",
			command: "kubectl get node %s --show-labels",
			run: func(c *k8s.Client, target, _ string, _ []flag) tea.Cmd {
				return read("Show", target, func(ctx context.Context) (string, error) {
					labels, taints, err := c.LabelsAndTaints(ctx, target)
					return labelsAndTaintsText(labels, taints), err
				})
			},
		},
		{
			key: "c", name: "Cordon", about: "stop new pods from being placed on the node",
			changes: true,
			flags:   []flag{{name: flagDryRun}},
			command: "kubectl cordon %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				return change("Cordon", target, flags, func(ctx context.Context) (string, error) {
					return "", c.Cordon(ctx, target, ticked(flags, flagDryRun))
				})
			},
		},
		{
			key: "u", name: "Uncordon", about: "let new pods onto the node again",
			changes: true,
			flags:   []flag{{name: flagDryRun}},
			command: "kubectl uncordon %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				return change("Uncordon", target, flags, func(ctx context.Context) (string, error) {
					return "", c.Uncordon(ctx, target, ticked(flags, flagDryRun))
				})
			},
		},
		{
			key: "l", name: "Label", about: "set labels (key=value) or remove them (key-), several with spaces",
			changes: true,
			input:   "labels",
			flags:   []flag{{name: flagOverwrite}, {name: flagDryRun}},
			command: "kubectl label node %s",
			run: func(c *k8s.Client, target, input string, flags []flag) tea.Cmd {
				return change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.Label(ctx, target, strings.Fields(input), ticked(flags, flagOverwrite), ticked(flags, flagDryRun))
					return changeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			key: "t", name: "Taint", about: "add taints (key=value:NoSchedule) or remove them (key:NoSchedule- or key-)",
			changes: true,
			input:   "taints",
			flags:   []flag{{name: flagOverwrite}, {name: flagDryRun}},
			command: "kubectl taint node %s",
			run: func(c *k8s.Client, target, input string, flags []flag) tea.Cmd {
				return change("Taint", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.Taint(ctx, target, strings.Fields(input), ticked(flags, flagOverwrite), ticked(flags, flagDryRun))
					return changeText("Taints", ch.Before, ch.After), err
				})
			},
		},
	}
}

// labelsAndTaintsText is the Show page:
//
//	Labels:
//
//	  env=prod
//	  kubernetes.io/os=linux
//
//	Taints:
//
//	  (none)
func labelsAndTaintsText(labels, taints []string) string {
	var lines []string
	for _, part := range []struct {
		title string
		items []string
	}{{"Labels", labels}, {"Taints", taints}} {
		lines = append(lines, part.title+":", "")
		if len(part.items) == 0 {
			lines = append(lines, "  (none)")
		}
		for _, item := range part.items {
			lines = append(lines, "  "+item)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
