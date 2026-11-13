package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var nsKey = key(KeyNamespace)

// nsCluster is a fake cluster with namespaces apps and default, and a pod
// in each.
func nsCluster() *fake.Clientset {
	pod := func(ns, name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, CreationTimestamp: metav1.Now()},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}
	return fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "apps"}},
		pod("default", "web"), pod("apps", "shop"),
	)
}

// onPodsWith is a model on the Pods tab of cs, its pods of every namespace loaded.
func onPodsWith(t *testing.T, cs *fake.Clientset) Model {
	t.Helper()
	m := sized(t, New(k8s.NewClientFromClientset(cs)), 120, 30)
	m.active = 1
	return press(t, m, loadPods(m.client, false, allNamespaces)())
}

// pickNs opens the picker, loads it, and picks the choice called name.
func pickNs(t *testing.T, m Model, name string) Model {
	t.Helper()
	next, cmd := m.Update(nsKey)
	m = run(t, next.(Model), cmd) // the namespaces arrive
	for i, c := range m.namespaceChoices() {
		if c == name {
			m.overlayCursor = i
		}
	}
	next, load := m.Update(enterKey)
	return run(t, next.(Model), load) // the tab reloads from it
}

func TestNamespaceButtonOnlyOnNamespacedTabs(t *testing.T) {
	m := New(nil)
	for i, want := range map[int]bool{0: false, 1: true, 2: true, 3: true, 4: false} {
		m.active = i
		has := false
		for _, b := range m.panelButtons() {
			has = has || b.name == buttonNamespace
		}
		if has != want {
			t.Errorf("%s: ns button = %v, want %v", m.activeName(), has, want)
		}
	}
	m.active = 1
	if b := m.panelButtons()[0]; b.label != "ns: all" || b.on {
		t.Errorf("first button on Pods = %+v, want ns: all, off", b)
	}
}

// n opens the picker: all namespaces first, then the cluster's, the
// one picked now marked.
func TestNamespacePickerLists(t *testing.T) {
	m := onPodsWith(t, nsCluster())
	next, cmd := m.Update(nsKey)
	if m = next.(Model); m.overlay != overlayNamespaces || m.overlayTitle() != "Namespace" {
		t.Fatalf("overlay = %d, title = %q", m.overlay, m.overlayTitle())
	}
	if rows, _ := m.namespaceRows(); rows[0] != "loading..." {
		t.Errorf("before they load: %q", rows)
	}
	m = run(t, m, cmd)
	rows, _ := m.namespaceRows()
	if strings.Join(rows, "|") != "✓ all namespaces|  apps|  default" {
		t.Errorf("rows = %q", rows)
	}
	if m.kubectlCommand() != "kubectl get namespaces" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// Picking a namespace shows only its pods, without the NAMESPACE column,
// with -n on the command.
func TestPickNamespace(t *testing.T) {
	m := pickNs(t, onPodsWith(t, nsCluster()), "apps")
	if m.namespace != "apps" || m.overlay != overlayNone {
		t.Fatalf("namespace = %q, overlay = %d", m.namespace, m.overlay)
	}
	rows := m.rows()
	if len(rows) != 1 || !strings.HasPrefix(rows[0], "shop   0/1   Running") {
		t.Errorf("rows = %q, want only shop, no namespace column", rows)
	}
	if m.kubectlCommand() != "kubectl get pods -n apps" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	if b := m.panelButtons()[0]; b.label != "ns: apps" || !b.on {
		t.Errorf("ns button = %+v", b)
	}
	// The pod's actions are for apps/shop.
	if m = press(t, m, down, down, down, enterKey); m.target != "apps/shop" {
		t.Errorf("target = %q", m.target)
	}
}

// Back to all namespaces brings back every pod and the NAMESPACE column.
func TestPickAllNamespaces(t *testing.T) {
	m := pickNs(t, pickNs(t, onPodsWith(t, nsCluster()), "apps"), allNamespacesChoice)
	if m.namespace != allNamespaces || len(m.rows()) != 2 || m.kubectlCommand() != "kubectl get pods --all-namespaces" {
		t.Errorf("namespace = %q, rows = %q, command = %q", m.namespace, m.rows(), m.kubectlCommand())
	}
}

// Rows on their way from the namespace picked before are dropped.
func TestStaleNamespaceRowsDropped(t *testing.T) {
	m := pickNs(t, onPodsWith(t, nsCluster()), "apps")
	m = press(t, m, itemsLoadedMsg{tab: tabPods, rows: []string{"old   row"}, at: time.Now(), namespace: allNamespaces})
	if got := m.rows(); len(got) != 1 || !strings.HasPrefix(got[0], "shop") {
		t.Errorf("rows = %q, want the apps rows kept", got)
	}
}

// Nodes isn't namespaced: n does nothing there, and it loads every node.
func TestNamespaceNotOnNodes(t *testing.T) {
	m := pickNs(t, onPodsWith(t, nsCluster()), "apps")
	m = press(t, m, left) // Nodes
	if m = press(t, m, nsKey); m.overlay != overlayNone {
		t.Error("n opened the picker on Nodes")
	}
	if m.kubectlCommand() != "kubectl get nodes" || m.tabNamespace() != allNamespaces {
		t.Errorf("command = %q, namespace = %q", m.kubectlCommand(), m.tabNamespace())
	}
}

// esc closes the picker without changing anything.
func TestNamespaceEsc(t *testing.T) {
	m := onPodsWith(t, nsCluster())
	next, cmd := m.Update(nsKey)
	m = run(t, next.(Model), cmd)
	m = press(t, m, down, escKey)
	if m.overlay != overlayNone || m.namespace != allNamespaces {
		t.Errorf("overlay = %d, namespace = %q", m.overlay, m.namespace)
	}
}

// A new pod goes into the namespace picked.
func TestRunInPickedNamespace(t *testing.T) {
	m := pickNs(t, onPodsWith(t, nsCluster()), "apps")
	m = press(t, m, key(KeyOwn))
	if m.overlayTitle() != "Run a pod in apps" || m.kubectlCommand() != "kubectl run nginx --image=nginx:latest -n apps" {
		t.Errorf("title = %q, command = %q", m.overlayTitle(), m.kubectlCommand())
	}
}

func TestPodTargetInNamespace(t *testing.T) {
	if got := podTarget("shop   1/1   Running", "apps"); got != "apps/shop" {
		t.Errorf("podTarget = %q", got)
	}
}
