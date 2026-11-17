package commands

import (
	"context"
	"strconv"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands for the More tab: every kind of thing the cluster serves.

// LoadResources asks the cluster which resource types it serves (like
// `kubectl api-resources`, or with wide, `kubectl api-resources -o wide`).
func LoadResources(c *k8s.Client, wide bool, _ string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		resources, err := c.ListResources(ctx)
		return ItemsLoadedMsg{Tab: TabMore, Rows: resourceRows(resources, wide), Err: err, At: time.Now(), Wide: wide}
	}
}

// resourceRows turns each resource type into one line of text, with its
// columns lined up, as `kubectl api-resources` shows them:
//
//	NAME  SHORTNAMES  APIVERSION  NAMESPACED  KIND
//
// With wide, the columns `-o wide` adds come after:
//
//	NAME  SHORTNAMES  APIVERSION  NAMESPACED  KIND  VERBS  CATEGORIES
//
// Several short names, verbs or categories are joined with commas.
func resourceRows(resources []k8s.Resource, wide bool) []string {
	cells := make([][]string, 0, len(resources))
	for _, r := range resources {
		row := []string{
			r.Name,
			strings.Join(r.ShortNames, ","),
			r.APIVersion,
			strconv.FormatBool(r.Namespaced),
			r.Kind,
		}
		if wide {
			row = append(row, strings.Join(r.Verbs, ","), strings.Join(r.Categories, ","))
		}
		cells = append(cells, row)
	}
	return AlignColumns(cells)
}

// ResourceOf reads a row of the More tab: the resource's name, API version
// and kind. The row is
//
//	NAME  [SHORTNAMES]  APIVERSION  NAMESPACED  KIND  [VERBS  CATEGORIES]
//
// where the short names may be missing, so the columns are found from
// NAMESPACED, the first one that is "true" or "false".
func ResourceOf(row string) (name, apiVersion, kind string, ok bool) {
	f := strings.Fields(row)
	for i := 2; i+1 < len(f); i++ {
		if f[i] == "true" || f[i] == "false" {
			return f[0], f[i-1], f[i+1], true
		}
	}
	return "", "", "", false
}

// resourceTabs are the tabs that show a kind of resource, by its API
// version and kind: "v1 Pod" is the Pods tab. Matching both, and not the
// name, keeps another API's "pods" (metrics.k8s.io) out.
var resourceTabs = map[string]string{
	"v1 Node":            TabNodes,
	"v1 Pod":             TabPods,
	"apps/v1 Deployment": TabDeployments,
	"v1 Service":         TabServices,

	// Kinds without a tab of their own are opened inside More.
	"v1 PersistentVolume":      TabPersistentVolumes,
	"v1 PersistentVolumeClaim": TabPersistentVolumeClaims,
}

// ResourceTab is the tab that shows the resource of a More row, e.g.
// Pods for "pods  po  v1  true  Pod", and the resource's name. ok is false
// if no tab shows it (yet); name is still set if the row could be read.
func ResourceTab(row string) (tab, name string, ok bool) {
	name, apiVersion, kind, read := ResourceOf(row)
	if !read {
		return "", RowName(row), false
	}
	if tab, ok = resourceTabs[apiVersion+" "+kind]; ok {
		return tab, name, true
	}
	if v, ok := viewFor(apiVersion, kind); ok {
		return v.Name(), name, true
	}
	return "", name, false
}
