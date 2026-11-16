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
