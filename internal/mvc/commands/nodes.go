package commands

import (
	"context"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for the Nodes tab. A command is slow work (an API call):
// Bubble Tea runs it in the background so the screen never freezes,
// then sends the message it returns (ItemsLoadedMsg, in actions.go) to Update.

// LoadNodes calls the API for the nodes (like `kubectl get nodes`, or with
// wide, `kubectl get nodes -o wide`).
func LoadNodes(c *k8s.Client, wide bool, _ string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		nodes, err := c.ListNodes(ctx)
		return ItemsLoadedMsg{Tab: TabNodes, Rows: NodeRows(nodes, wide), Err: err, At: time.Now(), Wide: wide}
	}
}

// columnGap is the number of spaces between two columns of a row.
const columnGap = 3

// NodeRows turns each node into one line of text, with its columns lined up:
//
//	NAME  STATUS  VERSION
//
// With wide, the columns `kubectl get nodes -o wide` adds come after:
//
//	NAME  STATUS  VERSION  INTERNAL-IP  EXTERNAL-IP  OS-IMAGE  KERNEL-VERSION  CONTAINER-RUNTIME
func NodeRows(nodes []k8s.Node, wide bool) []string {
	cells := make([][]string, 0, len(nodes))
	for _, n := range nodes {
		row := []string{n.Name, n.Status, n.Version}
		if wide {
			row = append(row, n.InternalIP, orNone(n.ExternalIP), n.OSImage, n.KernelVersion, n.Runtime)
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// orNone returns s, or "<none>" if it is empty, as kubectl shows it.
func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// AlignColumns joins each row's cells into one line, padding every column
// to its widest cell so the columns line up. The last column is not padded.
func AlignColumns(rows [][]string) []string {
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

// The Shell flags: ubuntu instead of busybox, and a privileged pod.
const (
	flagUbuntu   = "--image=" + k8s.ShellImageUbuntu
	flagBusybox  = "--image=" + k8s.ShellImageBusybox
	flagSysadmin = "--profile=sysadmin"
)

// FlagOverwrite lets Label and Taint replace a label's value, or a taint
// with the same key and effect, that the node already has.
const FlagOverwrite = "--overwrite"

// NodeActions are the actions of the Nodes tab: enter on a node opens them.
// The cluster calls they make are in internal/kubernetes/nodes.go.
func NodeActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the node, like kubectl describe",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe node %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribeNode(ctx, target, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the node's labels and taints",
			Command: "kubectl get node %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, taints, err := c.LabelsAndTaints(ctx, target)
					return SectionsText(Section{"Labels", labels}, Section{"Taints", taints}), err
				})
			},
		},
		{
			Key: "x", Name: "Shell", About: "a shell on the node, in a debug pod; the node's files are in /host",
			Changes: true, // it starts a pod on the node
			Flags:   []Flag{{Name: flagUbuntu, Off: flagBusybox}, {Name: flagSysadmin}},
			Command: "kubectl debug node/%s -it",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				image := k8s.ShellImageBusybox
				if Ticked(flags, flagUbuntu) {
					image = k8s.ShellImageUbuntu
				}
				cmd, err := c.NodeShell(target, image, Ticked(flags, flagSysadmin))
				// Like kubectl, the debug pod is left behind.
				closed := "Shell on " + target + " closed; its debug pod is still there, delete it when you are done"
				return InTerminal("Shell", target, closed, true, cmd, err)
			},
		},
		{
			Key: "e", Name: "Edit", About: "the node's YAML in your editor ($KUBE_EDITOR, $EDITOR or vi); saving applies it",
			Command: "kubectl edit node %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				cmd, err := c.EditNode(target)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "c", Name: "Cordon", About: "stop new pods from being placed on the node",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl cordon %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Change("Cordon", target, flags, func(ctx context.Context) (string, error) {
					return "", c.Cordon(ctx, target, Ticked(flags, FlagDryRun))
				})
			},
		},
		{
			Key: "u", Name: "Uncordon", About: "let new pods onto the node again",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl uncordon %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				return Change("Uncordon", target, flags, func(ctx context.Context) (string, error) {
					return "", c.Uncordon(ctx, target, Ticked(flags, FlagDryRun))
				})
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-), several with spaces",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label node %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.Label(ctx, target, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "t", Name: "Taint", About: "add taints (key=value:NoSchedule) or remove them (key:NoSchedule- or key-)",
			Changes: true,
			Input:   "taints",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl taint node %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				return Change("Taint", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.Taint(ctx, target, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Taints", ch.Before, ch.After), err
				})
			},
		},
	}
}
