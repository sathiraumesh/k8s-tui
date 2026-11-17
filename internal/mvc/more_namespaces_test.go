package mvc

import (
	"strings"
	"testing"

	"k8s-tui/internal/mvc/commands"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Use makes the namespace the one the tabs show.
func TestNamespacesUse(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "namespaces   ns   v1   false   Namespace", "default   Active   86d", "shop   Active   1d")
	if !strings.Contains(m.View(), "More › ns") {
		t.Fatal("not in More › ns")
	}
	m = press(t, m, down)                               // shop
	next, cmd := press(t, m, enterKey).Update(key("u")) // Use runs at once
	m = next.(Model)
	m = press(t, m, cmd())
	if m.namespace != "shop" || m.overlay != overlayNone || !strings.Contains(m.notice, "namespace: shop") {
		t.Errorf("namespace = %q, overlay = %d, notice = %q", m.namespace, m.overlay, m.notice)
	}
	if got := m.items[tabPods]; len(got) != 1 || got[0] != "loading..." {
		t.Errorf("Pods rows = %q, want them to load again", got)
	}
}

func TestNamespacesStatusAndDelete(t *testing.T) {
	if commands.NamespaceStatus.Level("Terminating") != commands.LevelWarn || commands.NamespaceStatus.Level("Active") != commands.LevelOK {
		t.Error("Active green, Terminating yellow")
	}
	m := openKindFromMore(t, fake.NewClientset(), "namespaces   ns   v1   false   Namespace", "shop   Active   1d")
	m = press(t, m, enterKey, key("r"))
	if m.kubectlCommand() != "kubectl delete namespaces shop" || !strings.Contains(m.View(), "EVERYTHING in it") {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

func TestCreateNamespaceFromView(t *testing.T) {
	cs := fake.NewClientset()
	m := openKindFromMore(t, cs, "namespaces   ns   v1   false   Namespace")
	m = press(t, m, key(KeyOwn), key("n"))
	if m.overlayTitle() != "Create a namespace" {
		t.Errorf("title = %q", m.overlayTitle())
	}
	m = typeText(t, m, "shop")
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if _, err := cs.CoreV1().Namespaces().Get(t.Context(), "shop", metav1.GetOptions{}); err != nil {
		t.Errorf("no namespace: %v", err)
	}
}
