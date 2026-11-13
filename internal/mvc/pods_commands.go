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
