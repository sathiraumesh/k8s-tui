package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for the Deployments tab: the same as for Pods where a
// deployment can do the same (Describe, Show, Edit, Label,
// Delete, Set image, and making a new one from an image), and Scale and
// Restart, which only a deployment has. The cluster calls are in
// internal/kubernetes/deployments.go and terminal.go.

// LoadDeployments calls the API for the deployments (like `kubectl get
// deployments --all-namespaces`, or with wide, `-o wide`).
func LoadDeployments(c *k8s.Client, wide bool, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		deployments, err := c.ListDeployments(ctx, namespace) // "": all namespaces
		now := time.Now()
		rows := deploymentRows(deployments, wide, namespace == AllNamespaces, now)
		return ItemsLoadedMsg{Tab: TabDeployments, Rows: rows, Err: err, At: now, Wide: wide, Namespace: namespace}
	}
}

// deploymentRows turns each deployment into one line, with its columns
// lined up, as `kubectl get deployments --all-namespaces` shows them:
//
//	NAMESPACE  NAME  READY  UP-TO-DATE  AVAILABLE  AGE
//
// With withNamespace false the NAMESPACE column is left out. With wide,
// the columns `-o wide` adds come after:
//
//	CONTAINERS  IMAGES  SELECTOR
//
// now is the time the ages are counted to.
func deploymentRows(deployments []k8s.Deployment, wide, withNamespace bool, now time.Time) []string {
	cells := make([][]string, 0, len(deployments))
	for _, d := range deployments {
		row := []string{d.Name, d.Ready, fmt.Sprint(d.UpToDate), fmt.Sprint(d.Available), age(d.CreatedAt, now)}
		if withNamespace {
			row = append([]string{d.Namespace}, row...)
		}
		if wide {
			row = append(row, orNone(strings.Join(d.Containers, ",")), orNone(strings.Join(d.Images, ",")), orNone(d.Selector))
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// DeploymentActions are the actions of the Deployments tab: enter on a
// deployment opens them. The keys are those of Pods for the same action.
func DeploymentActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the deployment, like kubectl describe",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe deployment %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribeDeployment(ctx, namespace, name, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the deployment's labels and annotations",
			Command: "kubectl get deployment %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.DeploymentLabelsAndAnnotations(ctx, namespace, name)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}), err
				})
			},
		},
		{
			Key: "e", Name: "Edit", About: "the deployment's YAML in your editor; a changed template rolls out new pods",
			Command: "kubectl edit deployment %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.EditDeployment(namespace, name)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "c", Name: "Scale", About: "set how many pods it runs; 0 stops them all",
			Changes:   true,
			Input:     "replicas",
			InputArgs: func(n string) string { return "--replicas=" + n },
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl scale deployment %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Scale", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.ScaleDeployment(ctx, namespace, name, input, Ticked(flags, FlagDryRun))
					return ChangeText("Replicas", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "i", Name: "Set image", About: "change a container's image; the deployment rolls out new pods with it",
			Changes:    true,
			Input:      "images (CONTAINER=IMAGE)",
			Containers: (*k8s.Client).DeploymentContainers,
			Flags:      []Flag{{Name: FlagDryRun}},
			Command:    "kubectl set image deployment/%s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Set image", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.SetDeploymentImage(ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagDryRun))
					return ChangeText("Images", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "t", Name: "Restart", About: "replace its pods with new ones, a few at a time, like kubectl rollout restart",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl rollout restart deployment %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Restart", target, flags, func(ctx context.Context) (string, error) {
					return "", c.RestartDeployment(ctx, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-) on the deployment, not its pods",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label deployment %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelDeployment(ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: "remove the deployment, and with it its pods",
			Changes: true,
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl delete deployment %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeleteDeployment(ctx, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// CreateDeploymentActions are the Deployments tab's own actions: one per
// image, to make a deployment of one pod from it like `kubectl create
// deployment NAME --image=IMAGE -n default`, and one for any other image.
// Scale it afterwards for more pods.
func CreateDeploymentActions() []Action {
	return imageActions("Create", "kubectl create deployment", (*k8s.Client).CreateDeployment)
}
