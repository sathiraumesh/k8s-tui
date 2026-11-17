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

func TestResourceOf(t *testing.T) {
	rows := resourceRows(append(someResources, k8s.Resource{Name: "pods", APIVersion: "metrics.k8s.io/v1beta1", Namespaced: true, Kind: "PodMetrics"}), true)
	for i, want := range [][3]string{
		{"deployments", "apps/v1", "Deployment"},
		{"nodes", "v1", "Node"},
		{"customthings", "example.com/v1", "CustomThing"}, // no short names
		{"pods", "metrics.k8s.io/v1beta1", "PodMetrics"},
	} {
		name, version, kind, ok := ResourceOf(rows[i])
		if !ok || [3]string{name, version, kind} != want {
			t.Errorf("row %q: %q %q %q %v, want %q", rows[i], name, version, kind, ok, want)
		}
	}
}

func TestResourceTab(t *testing.T) {
	for row, want := range map[string]string{
		"pods          po       v1                       true    Pod":        TabPods,
		"nodes         no       v1                       false   Node":       TabNodes,
		"deployments   deploy   apps/v1                  true    Deployment": TabDeployments,
		"services      svc      v1                       true    Service":    TabServices,
	} {
		if tab, _, ok := ResourceTab(row); !ok || tab != want {
			t.Errorf("%q: tab %q, %v, want %q", row, tab, ok, want)
		}
	}
	for _, row := range []string{
		"widgets   example.com/v1   true   Widget",          // a custom resource
		"pods   metrics.k8s.io/v1beta1   true   PodMetrics", // not the Pods tab's pods
	} {
		if tab, name, ok := ResourceTab(row); ok || name == "" {
			t.Errorf("%q: tab %q, name %q, %v, want no tab but the name", row, tab, name, ok)
		}
	}
}

func TestAnyView(t *testing.T) {
	v, ok := AnyView("certificates   cert,certs   cert-manager.io/v1   true   Certificate")
	if !ok || v.Name() != "certificates.cert-manager.io" || !v.Type.Namespaced || v.Type.Version != "v1" || v.Title != "certificates" {
		t.Errorf("view = %+v, %v", v.Type, ok)
	}
	if v, _ := AnyView("clusterissuers   cert-manager.io/v1   false   ClusterIssuer"); v.Type.Namespaced {
		t.Error("a cluster-scoped kind is namespaced")
	}
}
