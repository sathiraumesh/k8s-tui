package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"
)

// withView is a model on More with view open in it, its rows loaded.
func withView(t *testing.T, view commands.View, rows ...string) Model {
	t.Helper()
	m := sized(t, onMore(t, New(nil)), 120, 30)
	m.kinds = append(m.kinds, viewTab(view))
	m.moreKind = view.Name()
	m.focus = focusList
	return press(t, m, commands.ItemsLoadedMsg{Tab: view.Name(), Rows: commandsRows(rows...), At: time.Now()})
}

var configMapsView = commands.View{
	Type:  k8s.ResourceType{Version: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true},
	Title: "cm",
}

// A generic view works like a tab: its rows, its namespace, its actions.
func TestGenericView(t *testing.T) {
	m := withView(t, configMapsView, "apps   app-config   2   3h")
	if !strings.Contains(m.View(), "More › cm") || m.kubectlCommand() != "kubectl get configmaps --all-namespaces" {
		t.Fatalf("command = %q", m.kubectlCommand())
	}
	if b := buttonNamed(t, m, buttonNamespace); b.label != "ns: all" {
		t.Errorf("ns button = %+v", b)
	}
	m = press(t, m, enterKey)
	if m.target != "apps/app-config" || m.overlay != overlayActions {
		t.Fatalf("target = %q, overlay = %d", m.target, m.overlay)
	}
	for k, want := range map[string]string{
		"d": "kubectl describe configmaps app-config -n apps",
		"y": "kubectl get configmaps app-config -n apps -o yaml",
		"r": "kubectl delete configmaps app-config -n apps",
	} {
		next := m
		next.overlayCursor = 0
		for i, a := range next.actions() {
			if a.Key == k {
				next.overlayCursor = i
			}
		}
		if got := next.kubectlCommand(); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
}
