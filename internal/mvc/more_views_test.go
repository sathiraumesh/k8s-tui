package mvc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	"k8s.io/client-go/rest"
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

// Opened from a row of an older version, the view lists that version.
func TestViewOpensServedVersion(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`{"kind":"Table","apiVersion":"meta.k8s.io/v1","columnDefinitions":[],"rows":[]}`))
	}))
	defer srv.Close()
	c, _ := k8s.NewClientFromConfig(&rest.Config{Host: srv.URL}, "")
	m := New(c).servedVersion("statefulsets   sts   apps/v1beta2   true   StatefulSet")
	sts, _ := m.kindTab("statefulsets.apps")
	sts.load(c, false, "")()
	if path != "/apis/apps/v1beta2/statefulsets" {
		t.Errorf("asked %q, want the version the row said", path)
	}
}
