package mvc

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
// the one on screen (loadActive / refreshActive in refresh.go), so the
// pods aren't fetched while you look at another tab.

// loadPods calls the API for the pods of every namespace (like
// `kubectl get pods --all-namespaces`, or with wide, `-o wide`).
func loadPods(c *k8s.Client, wide bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()

		pods, err := c.ListPods(ctx, "") // "": all namespaces
		now := time.Now()
		return itemsLoadedMsg{tab: tabPods, rows: podRows(pods, wide, now), err: err, at: now, wide: wide}
	}
}

// podRows turns each pod into one line, with its columns lined up, as
// `kubectl get pods --all-namespaces` shows them:
//
//	NAMESPACE  NAME  READY  STATUS  RESTARTS  AGE
//
// With wide, the columns `-o wide` adds come after:
//
//	IP  NODE  NOMINATED-NODE  READINESS-GATES
//
// now is the time the ages are counted to.
func podRows(pods []k8s.Pod, wide bool, now time.Time) []string {
	cells := make([][]string, 0, len(pods))
	for _, p := range pods {
		row := []string{p.Namespace, p.Name, p.Ready, p.Status, restarts(p, now), age(p.CreatedAt, now)}
		if wide {
			row = append(row, orNone(p.IP), orNone(p.Node), orNone(p.NominatedNode), orNone(p.ReadinessGates))
		}
		cells = append(cells, row)
	}
	return alignColumns(cells)
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

// podTarget is what a pod row's actions are for: "NAMESPACE/NAME", from
// its first two columns.
func podTarget(row string) string {
	f := strings.Fields(row)
	if len(f) < 2 {
		return rowName(row)
	}
	return f[0] + "/" + f[1]
}

// splitTarget splits "NAMESPACE/NAME" into its two parts.
func splitTarget(target string) (namespace, name string) {
	namespace, name, _ = strings.Cut(target, "/")
	return namespace, name
}

// podArgs writes "NAMESPACE/NAME" the way kubectl takes it: "NAME -n NAMESPACE".
func podArgs(target string) string {
	namespace, name := splitTarget(target)
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

// podActions are the actions of the Pods tab: enter on a pod opens them.
// The keys are the same as for a node where the action is the same. The
// cluster calls they make are in internal/kubernetes/pods.go and
// terminal.go.
func podActions() []action {
	return []action{
		{
			key: "d", name: "Describe", about: "everything about the pod, like kubectl describe",
			flags:   []flag{{name: flagNoEvents}},
			command: "kubectl describe pod %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				return read("Describe", target, func(ctx context.Context) (string, error) {
					return c.DescribePod(ctx, namespace, name, !ticked(flags, flagNoEvents))
				})
			},
		},
		{
			key: "o", name: "Logs", about: fmt.Sprintf("the last %d lines of each container, newest at the bottom", logTail),
			flags:   []flag{{name: flagPrevious}, {name: flagTimestamps}},
			command: fmt.Sprintf("kubectl logs %%s --all-containers --tail=%d", logTail),
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				return readToEnd("Logs", target, func(ctx context.Context) (string, error) {
					return c.PodLogs(ctx, namespace, name, ticked(flags, flagPrevious), ticked(flags, flagTimestamps), logTail)
				})
			},
		},
		{
			key: "s", name: "Show", about: "the pod's labels and annotations",
			command: "kubectl get pod %s --show-labels",
			run: func(c *k8s.Client, target, _ string, _ []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				return read("Show", target, func(ctx context.Context) (string, error) {
					labels, annotations, err := c.LabelsAndAnnotations(ctx, namespace, name)
					return sectionsText(section{"Labels", labels}, section{"Annotations", annotations}), err
				})
			},
		},
		{
			key: "x", name: "Shell", about: "a shell in the pod's first container; the image needs sh (or bash)",
			flags:   []flag{{name: flagBash, off: flagSh}},
			command: "kubectl exec -it %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				cmd, err := c.PodShell(namespace, name, ticked(flags, flagBash))
				return inTerminal("Shell", target, "Shell in "+target+" closed", true, cmd, err)
			},
		},
		{
			key: "e", name: "Edit", about: "the pod's YAML in your editor; most of a running pod can't change",
			command: "kubectl edit pod %s",
			run: func(c *k8s.Client, target, _ string, _ []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				cmd, err := c.EditPod(namespace, name)
				return inTerminal("Edit", target, "Edit "+target+": editor closed", false, cmd, err)
			},
		},
		{
			key: "l", name: "Label", about: "set labels (key=value) or remove them (key-), several with spaces",
			changes: true,
			input:   "labels",
			flags:   []flag{{name: flagOverwrite}, {name: flagDryRun}},
			command: "kubectl label pod %s",
			run: func(c *k8s.Client, target, input string, flags []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				return change("Label", target, flags, func(ctx context.Context) (string, error) {
					ch, err := c.LabelPod(ctx, namespace, name, strings.Fields(input), ticked(flags, flagOverwrite), ticked(flags, flagDryRun))
					return changeText("Labels", ch.Before, ch.After), err
				})
			},
		},
		{
			key: "r", name: "Delete", about: "remove the pod; if a deployment or the like owns it, a new one starts",
			changes: true,
			flags:   []flag{{name: flagNow}, {name: flagDryRun}},
			command: "kubectl delete pod %s",
			run: func(c *k8s.Client, target, _ string, flags []flag) tea.Cmd {
				namespace, name := splitTarget(target)
				return change("Delete", target, flags, func(ctx context.Context) (string, error) {
					return "", c.DeletePod(ctx, namespace, name, ticked(flags, flagNow), ticked(flags, flagDryRun))
				})
			},
		},
	}
}

// ---------- running a new pod ----------

// runNamespace is where a new pod goes: the tab shows every namespace, and
// a pod has to be in one.
const runNamespace = "default"

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

// runPodActions are the Pods tab's own actions: one per image, to run a
// new pod from it like `kubectl run NAME --image=IMAGE -n default`, and one
// for any other image. The cluster call is RunPod in internal/kubernetes/pods.go.
//
// The name is typed on the confirm page, and starts as the image's name.
func runPodActions() []action {
	actions := make([]action, 0, len(podImages)+1)
	for _, img := range podImages {
		actions = append(actions, action{
			key: img.key, name: img.image, about: img.about,
			changes:      true,
			input:        "name",
			inputDefault: imageName(img.image),
			flags:        []flag{{name: flagDryRun}},
			command:      "kubectl run " + inputHere + " --image=" + img.image + " -n %s",
			run: func(c *k8s.Client, namespace, name string, flags []flag) tea.Cmd {
				name = strings.TrimSpace(name)
				return change("Run", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.RunPod(ctx, namespace, name, img.image, ticked(flags, flagDryRun))
				})
			},
		})
	}
	return append(actions, runOtherImage())
}

// runOtherImage runs a pod from an image that isn't in the list: the name
// and the image are typed, e.g. "db postgres:16".
func runOtherImage() action {
	return action{
		key: "o", name: "other image", about: "type a name and any image, e.g. db postgres:16",
		changes:   true,
		input:     "name and image",
		inputArgs: runArgs,
		flags:     []flag{{name: flagDryRun}},
		command:   "kubectl run " + inputHere + " -n %s",
		run: func(c *k8s.Client, namespace, input string, flags []flag) tea.Cmd {
			name, image, ok := splitNameImage(input)
			return change("Run", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
				if !ok {
					return "", fmt.Errorf("type a name and an image, e.g. db postgres:16")
				}
				return "", c.RunPod(ctx, namespace, name, image, ticked(flags, flagDryRun))
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

// imageName is the name a pod from image starts with: the image's own name,
// without registry, path, tag or digest, e.g. "nginx" for "docker.io/library/nginx:latest".
func imageName(image string) string {
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	if i := strings.IndexAny(image, ":@"); i >= 0 {
		image = image[:i]
	}
	return image
}
