package commands

import (
	"strings"
	"testing"

	k8s "k8s-tui/internal/kubernetes"
)

var someResources = []k8s.Resource{
	{Name: "deployments", ShortNames: []string{"deploy"}, APIVersion: "apps/v1", Namespaced: true,
		Kind: "Deployment", Verbs: []string{"get", "list", "watch"}, Categories: []string{"all"}},
	{Name: "nodes", ShortNames: []string{"no"}, APIVersion: "v1", Kind: "Node", Verbs: []string{"get", "list"}},
	{Name: "customthings", APIVersion: "example.com/v1", Namespaced: true, Kind: "CustomThing"},
}

// ---------- rows ----------

func TestResourceRows(t *testing.T) {
	rows := resourceRows(someResources, false)
	for i, want := range [][]string{
		{"deployments", "deploy", "apps/v1", "true", "Deployment"},
		{"nodes", "no", "v1", "false", "Node"},
		{"customthings", "example.com/v1", "true", "CustomThing"},
	} {
		for _, cell := range want {
			if !strings.Contains(rows[i], cell) {
				t.Errorf("row %d %q is missing %q", i, rows[i], cell)
			}
		}
	}
	if strings.Contains(rows[0], "watch") {
		t.Errorf("narrow row %q has the verbs, want them only with wide", rows[0])
	}
}

func TestResourceRowsWide(t *testing.T) {
	rows := resourceRows(someResources, true)
	if !strings.Contains(rows[0], "get,list,watch") || !strings.Contains(rows[0], "all") {
		t.Errorf("wide row %q, want verbs get,list,watch and category all", rows[0])
	}
}

// Columns line up, even when a resource has no short names.
func TestResourceRowsAligned(t *testing.T) {
	rows := resourceRows(someResources, false)
	for _, kind := range []string{"Deployment", "Node", "CustomThing"} {
		if !strings.Contains(strings.Join(rows, "\n"), kind) {
			t.Fatalf("rows are missing %s", kind)
		}
	}
	col := strings.Index(rows[0], "Deployment")
	if strings.Index(rows[1], "Node") != col || strings.Index(rows[2], "CustomThing") != col {
		t.Errorf("KIND column not aligned:\n%s", strings.Join(rows, "\n"))
	}
}
