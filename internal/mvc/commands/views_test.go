package commands

import (
	"strings"
	"testing"

	k8s "k8s-tui/internal/kubernetes"
)

var testView = View{
	Type:  k8s.ResourceType{Version: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true},
	Title: "cm",
	Actions: []Action{
		{Key: "c", Name: "Mine", Command: "kubectl mine %s"},
		{Key: "r", Name: "My delete", Command: "kubectl my-delete %s"},
	},
}

// A view's own actions come first; one with a generic key replaces it.
func TestViewAllActions(t *testing.T) {
	var keys, names []string
	for _, a := range testView.AllActions() {
		keys = append(keys, a.Key)
		names = append(names, a.Name)
	}
	if got := strings.Join(keys, ""); got != "crdysel" {
		t.Errorf("keys = %q, want crdysel", got)
	}
	if names[1] != "My delete" {
		t.Errorf("r is %q, want the view's own", names[1])
	}
}

func TestViewNamesAndTargets(t *testing.T) {
	if testView.Name() != "configmaps" || testView.Command() != "kubectl get configmaps" {
		t.Errorf("name %q, command %q", testView.Name(), testView.Command())
	}
	if got := testView.RowTarget("apps   app-config   2   3h", AllNamespaces); got != "apps/app-config" {
		t.Errorf("target = %q", got)
	}
	if got := testView.CommandTarget("apps/app-config"); got != "app-config -n apps" {
		t.Errorf("command target = %q", got)
	}
	cluster := View{Type: k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", Kind: "StorageClass"}}
	if cluster.RowTarget("standard   rancher.io/local-path   Delete", AllNamespaces) != "standard" || cluster.CommandTarget("standard") != "standard" {
		t.Error("a kind not in a namespace targets its name")
	}
	for _, a := range cluster.AllActions() {
		if a.Key == "d" && a.CommandFor(cluster.CommandTarget("standard"), "", nil) != "kubectl describe storageclasses.storage.k8s.io standard" {
			t.Errorf("describe = %q", a.CommandFor("standard", "", nil))
		}
	}
}

// No action of a view can have j or k for its key: they move the cursor in
// an action list. And no two actions of a list share a key.
func TestViewKeysUsable(t *testing.T) {
	for _, v := range Views() {
		for _, list := range [][]Action{v.AllActions(), v.Own} {
			seen := map[string]bool{}
			for _, a := range list {
				if a.Key == "j" || a.Key == "k" {
					t.Errorf("%s: %s has key %q, which moves the cursor", v.Name(), a.Name, a.Key)
				}
				if seen[a.Key] {
					t.Errorf("%s: two actions have key %q", v.Name(), a.Key)
				}
				seen[a.Key] = true
			}
		}
	}
}
