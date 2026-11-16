package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/util/duration"
)

// Commands for the Pods tab. Like every tab, it is only loaded while it is
// the one on screen (loadActive / refreshActive in mvc/refresh.go), so the
// pods aren't fetched while you look at another tab.

// LoadPods calls the API for the pods of every namespace (like
// `kubectl get pods --all-namespaces`, or with wide, `-o wide`).
func LoadPods(c *k8s.Client, wide bool, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		pods, err := c.ListPods(ctx, namespace) // "": all namespaces
		now := time.Now()
		rows := podRows(pods, wide, namespace == AllNamespaces, now)
		return ItemsLoadedMsg{Tab: TabPods, Rows: rows, Err: err, At: now, Wide: wide, Namespace: namespace}
	}
}

// podRows turns each pod into one line, with its columns lined up, as
// `kubectl get pods --all-namespaces` shows them:
//
//	NAMESPACE  NAME  READY  STATUS  RESTARTS  AGE
//
// With withNamespace false (one namespace is shown, like `kubectl get pods
// -n NAMESPACE`) the NAMESPACE column is left out. With wide, the columns
// `-o wide` adds come after:
//
//	IP  NODE  NOMINATED-NODE  READINESS-GATES
//
// now is the time the ages are counted to.
func podRows(pods []k8s.Pod, wide, withNamespace bool, now time.Time) []string {
	cells := make([][]string, 0, len(pods))
	for _, p := range pods {
		row := []string{p.Name, p.Ready, p.Status, restarts(p, now), age(p.CreatedAt, now)}
		if withNamespace {
			row = append([]string{p.Namespace}, row...)
		}
		if wide {
			row = append(row, orNone(p.IP), orNone(p.Node), orNone(p.NominatedNode), orNone(p.ReadinessGates))
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// restarts is the RESTARTS column: the count, and how long ago the last
// restart was, e.g. "2 (5m ago)", as kubectl shows it.
func restarts(p k8s.Pod, now time.Time) string {
	if p.Restarts == 0 || p.LastRestart.IsZero() {
		return fmt.Sprint(p.Restarts)
	}
	return fmt.Sprintf("%d (%s ago)", p.Restarts, age(p.LastRestart, now))
}

// age is how long ago t was, the way kubectl writes it: "45s", "5m", "3h",
// "8d", "2y". It is "<unknown>" if t isn't set.
func age(t, now time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(now.Sub(t))
}

// ---------- actions on a pod ----------

// PodTarget is what a pod row's actions are for: "NAMESPACE/NAME". While
// all namespaces are shown those are the row's first two columns; in one
// namespace the row starts with the name.
func PodTarget(row, namespace string) string {
	f := strings.Fields(row)
	switch {
	case namespace != AllNamespaces:
		return namespace + "/" + RowName(row)
	case len(f) < 2:
		return RowName(row)
	}
	return f[0] + "/" + f[1]
}

// SplitTarget splits "NAMESPACE/NAME" into its two parts.
func SplitTarget(target string) (namespace, name string) {
	namespace, name, _ = strings.Cut(target, "/")
	return namespace, name
}

// PodArgs writes "NAMESPACE/NAME" the way kubectl takes it: "NAME -n NAMESPACE".
func PodArgs(target string) string {
	namespace, name := SplitTarget(target)
	return name + " -n " + namespace
}

// Flags of the pod actions.
const (
	flagPrevious   = "--previous"   // Logs: the run before the last restart
	flagTimestamps = "--timestamps" // Logs: each line's time
	flagBash       = "-- bash"      // Shell: bash instead of sh
	flagSh         = "-- sh"
	flagNow        = "--now" // Delete: 1 second to stop, not the grace period
)

// logTail is how many of the last lines of each container Logs shows.
const logTail = 500

// PodActions are the actions of the Pods tab: enter on a pod opens them.
// The keys are the same as for a node where the action is the same. The
// cluster calls they make are in internal/kubernetes/pods.go and
// terminal.go.
func PodActions() []Action {
	return []Action{
		{
			Key: "d", Name: "Describe", About: "everything about the pod, like kubectl describe",
			Flags:   []Flag{{Name: flagNoEvents}},
			Command: "kubectl describe pod %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribePod(ctx, namespace, name, !Ticked(flags, flagNoEvents))
				})
			},
		},
		{
			Key: "o", Name: "Logs", About: fmt.Sprintf("the last %d lines of each container, newest at the bottom", logTail),
			Flags:      []Flag{{Name: flagPrevious}, {Name: flagTimestamps}},
			Containers: true,
			InputArgs:  ContainerArgs,
			Command:    fmt.Sprintf("kubectl logs %%s {input} --tail=%d", logTail),
			Run: func(c *k8s.Client, target, container string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				if container == AllContainers {
					container = ""
				}
				return ReadToEnd("Logs", target, func(ctx context.Context) (string, error) {
					return c.PodLogs(ctx, namespace, name, container, Ticked(flags, flagPrevious), Ticked(flags, flagTimestamps), logTail)
				})
			},
		},
		{
			Key: "s", Name: "Show", About: "the pod's labels and annotations",
			Command: "kubectl get pod %s --show-labels",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.LabelsAndAnnotations(ctx, namespace, name)
					return SectionsText(Section{"Labels", labels}, Section{"Annotations", annotations}), err
				})
			},
		},
		{
			Key: "x", Name: "Shell", About: "a shell in the pod's first container; the image needs sh (or bash)",
			Flags:   []Flag{{Name: flagBash, Off: flagSh}},
			Command: "kubectl exec -it %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.PodShell(namespace, name, Ticked(flags, flagBash))
				return InTerminal("Shell", target, "Shell in "+target+" closed", true, cmd, err)
			},
		},
		{
			Key: "e", Name: "Edit", About: "the pod's YAML in your editor; most of a running pod can't change",
			Command: "kubectl edit pod %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				cmd, err := c.EditPod(namespace, name)
				return InTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			Key: "l", Name: "Label", About: "set labels (key=value) or remove them (key-), several with spaces",
			Changes: true,
			Input:   "labels",
			Flags:   []Flag{{Name: FlagOverwrite}, {Name: FlagDryRun}},
			Command: "kubectl label pod %s",
			Run: func(c *k8s.Client, target, input string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelPod(ctx, namespace, name, strings.Fields(input), Ticked(flags, FlagOverwrite), Ticked(flags, FlagDryRun))
					return ChangeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			Key: "r", Name: "Delete", About: "remove the pod; if a deployment or the like owns it, a new one starts",
			Changes: true,
			Flags:   []Flag{{Name: flagNow}, {Name: FlagDryRun}},
			Command: "kubectl delete pod %s",
			Run: func(c *k8s.Client, target, _ string, flags []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeletePod(ctx, namespace, name, Ticked(flags, flagNow), Ticked(flags, FlagDryRun))
				})
			},
		},
	}
}

// ---------- running a new pod ----------

// RunNamespace is where a new pod goes: the tab shows every namespace, and
// a pod has to be in one.
const RunNamespace = "default"

// podImages are the images offered for a new pod, in the order shown, with
// the key that picks each. Any other image can be typed (runOtherImage).
var podImages = []struct{ key, image, about string }{
	{"n", "nginx:latest", "a web server"},
	{"b", "busybox:latest", "a small set of shell tools"},
	{"a", "alpine:latest", "a small Linux with a package manager"},
	{"u", "ubuntu:latest", "a full Linux with more tools"},
	{"r", "redis:latest", "an in-memory data store"},
	{"p", "httpd:latest", "the Apache web server"},
}

// RunPodActions are the Pods tab's own actions: one per image, to run a
// new pod from it like `kubectl run NAME --image=IMAGE -n default`, and one
// for any other image. The cluster call is RunPod in internal/kubernetes/pods.go.
//
// The name is typed on the confirm page, and starts as the image's name.
func RunPodActions() []Action {
	actions := make([]Action, 0, len(podImages)+1)
	for _, img := range podImages {
		actions = append(actions, Action{
			Key: img.key, Name: img.image, About: img.about,
			Changes:      true,
			Input:        "name",
			InputDefault: ImageName(img.image),
			Flags:        []Flag{{Name: FlagDryRun}},
			Command:      "kubectl run " + InputHere + " --image=" + img.image + " -n %s",
			Run: func(c *k8s.Client, namespace, name string, flags []Flag) tea.Cmd {
				name = strings.TrimSpace(name)
				return Change("Run", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.RunPod(ctx, namespace, name, img.image, Ticked(flags, FlagDryRun))
				})
			},
		})
	}
	return append(actions, runOtherImage())
}

// runOtherImage runs a pod from an image that isn't in the list: the name
// and the image are typed, e.g. "db postgres:16".
func runOtherImage() Action {
	return Action{
		Key: "o", Name: "other image", About: "type a name and any image, e.g. db postgres:16",
		Changes:   true,
		Input:     "name and image",
		InputArgs: runArgs,
		Flags:     []Flag{{Name: FlagDryRun}},
		Command:   "kubectl run " + InputHere + " -n %s",
		Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
			name, image, ok := splitNameImage(input)
			return Change("Run", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
				if !ok {
					return "", fmt.Errorf("type a name and an image, e.g. db postgres:16")
				}
				return "", c.RunPod(ctx, namespace, name, image, Ticked(flags, FlagDryRun))
			})
		},
	}
}

// splitNameImage reads "NAME IMAGE", as typed for another image.
func splitNameImage(input string) (name, image string, ok bool) {
	f := strings.Fields(input)
	if len(f) != 2 {
		return "", "", false
	}
	return f[0], f[1], true
}

// runArgs writes "db postgres:16" as kubectl takes it: "db --image=postgres:16".
// While only the name is typed it is shown as it is.
func runArgs(input string) string {
	if name, image, ok := splitNameImage(input); ok {
		return name + " --image=" + image
	}
	return input
}

// ImageName is the name a pod from image starts with: the image's own name,
// without registry, path, tag or digest, e.g. "nginx" for "docker.io/library/nginx:latest".
func ImageName(image string) string {
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	if i := strings.IndexAny(image, ":@"); i >= 0 {
		image = image[:i]
	}
	return image
}

// AllContainers is the picker's first choice, and the input for it.
const AllContainers = "*"

// ContainerArgs writes the choice as kubectl takes it.
func ContainerArgs(input string) string {
	if input == AllContainers {
		return "--all-containers"
	}
	return "--container=" + input
}
