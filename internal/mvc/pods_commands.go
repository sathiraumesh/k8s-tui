package mvc

import (
	"context"
	"fmt"
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
