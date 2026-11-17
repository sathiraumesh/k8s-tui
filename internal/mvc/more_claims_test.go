package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// openClaims opens the claims from More on a fake cluster with the claim
// apps/data, bound, and loads them.
func openClaims(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources:  corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			VolumeName: "pvc-1",
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	})
	m := sized(t, onMore(t, New(k8s.NewClientFromClientset(cs))), 120, 30)
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabMore, Rows: commandsRows(
		"persistentvolumeclaims   pvc   v1   true    PersistentVolumeClaim",
		"persistentvolumes        pv    v1   false   PersistentVolume",
	), At: time.Now()})
	m = press(t, m, down, down) // tabs → buttons → row 0
	next, cmd := m.Update(enterKey)
	return press(t, next.(Model), cmd()), cs
}

func TestMoreOpensClaims(t *testing.T) {
	m, _ := openClaims(t)
	if !m.inKind() || m.activeName() != commands.TabPersistentVolumeClaims {
		t.Fatalf("inKind = %v, active = %q", m.inKind(), m.activeName())
	}
	if rows := m.rows(); len(rows) != 1 || !strings.HasPrefix(rows[0], "apps   data   Bound   pvc-1") {
		t.Errorf("rows = %q", rows)
	}
	if !strings.Contains(m.View(), "More › pvc") || m.kubectlCommand() != "kubectl get persistentvolumeclaims --all-namespaces" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	// Claims are in a namespace: the view has the ns button.
	if b := buttonNamed(t, m, buttonNamespace); b.label != "ns: all" {
		t.Errorf("ns button = %+v", b)
	}
}

func TestClaimActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe persistentvolumeclaim data -n apps",
		"c": "kubectl patch persistentvolumeclaim data -n apps",
		"l": "kubectl label persistentvolumeclaim data -n apps",
		"r": "kubectl delete persistentvolumeclaim data -n apps",
	} {
		m, _ := openClaims(t)
		m = press(t, m, enterKey)
		if m = press(t, m, key(k)); m.kubectlCommand() != want {
			t.Errorf("%s: command = %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

// Picking a namespace reloads the claims from it.
func TestClaimsFollowNamespace(t *testing.T) {
	m, _ := openClaims(t)
	m = press(t, m, key(KeyNamespace), namespacesLoadedMsg{names: []string{"apps", "default"}})
	next, cmd := press(t, m, down).Update(enterKey) // "apps"
	m = next.(Model)
	if m.namespace != "apps" || !m.inKind() {
		t.Fatalf("namespace = %q, inKind = %v", m.namespace, m.inKind())
	}
	msg := cmd().(commands.ItemsLoadedMsg)
	if msg.Tab != commands.TabPersistentVolumeClaims || msg.Namespace != "apps" || !strings.HasPrefix(msg.Rows[0], "data   Bound") {
		t.Errorf("reload = %+v", msg)
	}
}

// + makes a claim in the namespace picked, here default.
func TestCreateClaim(t *testing.T) {
	m, cs := openClaims(t)
	m = press(t, m, key(KeyOwn))
	if m.overlayTitle() != "Create a claim in default" {
		t.Errorf("title = %q", m.overlayTitle())
	}
	m = press(t, m, key("c"))
	m = typeText(t, m, " -") // "data 1Gi -": no class, for a hostPath volume
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	if m = run(t, m, cmd); m.notice != "Create default/data: done" {
		t.Errorf("notice = %q", m.notice)
	}
	pvc, err := cs.CoreV1().PersistentVolumeClaims("default").Get(t.Context(), "data", metav1.GetOptions{})
	if err != nil || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "" {
		t.Errorf("claim = %+v, err = %v", pvc, err)
	}
}
