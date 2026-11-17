package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// openKindFromMore opens the view of the More row row, with rows loaded
// into it, on a model with client cs.
func openKindFromMore(t *testing.T, cs *fake.Clientset, row string, rows ...string) Model {
	t.Helper()
	m := sized(t, onMore(t, New(k8s.NewClientFromClientset(cs))), 120, 30)
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabMore, Rows: commandsRows(row), At: time.Now()})
	m = press(t, m, down, down, enterKey) // tabs → buttons → the row → open
	if !m.inKind() {
		t.Fatalf("enter on %q didn't open a view", row)
	}
	return press(t, m, commands.ItemsLoadedMsg{Tab: m.activeName(), Rows: commandsRows(rows...), At: time.Now()})
}

func TestConfigMapsView(t *testing.T) {
	cs := fake.NewClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "cfg"}, Data: map[string]string{"mode": "fast"}})
	m := openKindFromMore(t, cs, "configmaps   cm   v1   true   ConfigMap", "apps   cfg   1   3h")
	if !strings.Contains(m.View(), "More › cm") {
		t.Fatal("not in More › cm")
	}
	m = press(t, m, enterKey, key("a")) // Data
	if m.kubectlCommand() != "kubectl get configmaps cfg -n apps -o jsonpath='{.data}'" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

func TestConfigMapsDataRuns(t *testing.T) {
	cs := fake.NewClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "cfg"}, Data: map[string]string{"mode": "fast"}})
	m := openKindFromMore(t, cs, "configmaps   cm   v1   true   ConfigMap", "apps   cfg   1   3h")
	next, cmd := press(t, m, enterKey).Update(key("a"))
	if m = run(t, next.(Model), cmd); strings.Join(m.text, "\n") != "mode:\n  fast" {
		t.Errorf("text = %q", m.text)
	}
}

func TestCreateConfigMapFromView(t *testing.T) {
	cs := fake.NewClientset()
	m := openKindFromMore(t, cs, "configmaps   cm   v1   true   ConfigMap")
	m = press(t, m, key(KeyOwn), key("c"))
	m = typeText(t, m, "cfg mode=fast")
	if m.kubectlCommand() != "kubectl create configmap cfg --from-literal=mode=fast -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if cm, err := cs.CoreV1().ConfigMaps("default").Get(t.Context(), "cfg", metav1.GetOptions{}); err != nil || cm.Data["mode"] != "fast" {
		t.Errorf("config map = %+v, err = %v", cm, err)
	}
}
