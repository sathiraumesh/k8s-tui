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

// moreWithVolumes is a model on More, with persistentvolumes as its second
// row and the cursor on it, on a fake cluster with the Released volume old.
func moreWithVolumes(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "old"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			ClaimRef:                      &corev1.ObjectReference{Namespace: "apps", Name: "gone"},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased},
	})
	m := sized(t, onMore(t, New(k8s.NewClientFromClientset(cs))), 120, 30)
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabMore, Rows: commandsRows(
		"nodes               no   v1   false   Node",
		"persistentvolumes   pv   v1   false   PersistentVolume",
	), At: time.Now()})
	m = press(t, m, down, down, down) // tabs → buttons → row 0 → row 1
	return m, cs
}

// openVolumes opens the volumes from More and loads them.
func openVolumes(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	m, cs := moreWithVolumes(t)
	next, cmd := m.Update(enterKey)
	m = next.(Model)
	return press(t, m, cmd()), cs
}

func TestMoreOpensVolumes(t *testing.T) {
	m, _ := openVolumes(t)
	if !m.inKind() || m.activeName() != commands.TabPersistentVolumes || m.focus != focusList {
		t.Fatalf("inKind = %v, active = %q, focus = %d", m.inKind(), m.activeName(), m.focus)
	}
	if rows := m.rows(); len(rows) != 1 || !strings.HasPrefix(rows[0], "old   1Gi   RWO   Retain   Released   apps/gone") {
		t.Errorf("rows = %q", rows)
	}
	if !strings.Contains(m.View(), "More › pv") || m.kubectlCommand() != "kubectl get persistentvolumes" {
		t.Errorf("tab label or command wrong: %q", m.kubectlCommand())
	}
	if !strings.Contains(m.helpText(), "esc back to More") {
		t.Errorf("help = %q", m.helpText())
	}
}

// esc goes back to More's list, on the row the volumes were opened from.
func TestVolumesEscBack(t *testing.T) {
	m, _ := openVolumes(t)
	if m = press(t, m, escKey); m.inKind() || m.activeName() != tabMore || m.cursor != 1 {
		t.Errorf("inKind = %v, active = %q, cursor = %d", m.inKind(), m.activeName(), m.cursor)
	}
}

// A click on the More tab, or a right click, goes back too.
func TestVolumesClickBack(t *testing.T) {
	m, _ := openVolumes(t)
	if m = click(t, m, "More › pv"); m.inKind() {
		t.Error("a click on More didn't go back")
	}
	m, _ = openVolumes(t)
	if m = press(t, m, rightClick); m.inKind() {
		t.Error("a right click didn't go back")
	}
}

func TestVolumeActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe persistentvolume old",
		"p": `kubectl patch persistentvolume old -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'`,
		"a": `kubectl patch persistentvolume old --type=json -p '[{"op":"remove","path":"/spec/claimRef"}]'`,
		"l": "kubectl label persistentvolume old",
		"r": "kubectl delete persistentvolume old",
	} {
		m, _ := openVolumes(t)
		if m = press(t, m, enterKey, key(k)); m.kubectlCommand() != want {
			t.Errorf("%s: command = %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

// Make available frees the Released volume.
func TestVolumeMakeAvailable(t *testing.T) {
	m, cs := openVolumes(t)
	m = press(t, m, enterKey, key("a"))
	_, cmd := m.Update(key(KeyYes))
	if m = run(t, m, cmd); m.notice != "Make available old: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if pv, _ := cs.CoreV1().PersistentVolumes().Get(t.Context(), "old", metav1.GetOptions{}); pv.Spec.ClaimRef != nil {
		t.Error("the claim is still there")
	}
}

// + makes a hostPath volume; its page has no namespace in its title.
func TestCreateVolume(t *testing.T) {
	m, cs := openVolumes(t)
	m = press(t, m, key(KeyOwn))
	if m.overlayTitle() != "Create a persistent volume" {
		t.Errorf("title = %q", m.overlayTitle())
	}
	m = press(t, m, key("h"))
	if m.input != "data 1Gi /mnt/data" {
		t.Errorf("input = %q", m.input)
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if _, err := cs.CoreV1().PersistentVolumes().Get(t.Context(), "data", metav1.GetOptions{}); err != nil {
		t.Errorf("no volume made: %v", err)
	}
}

// Going to another tab and back keeps the volumes open.
func TestVolumesKeptAcrossTabs(t *testing.T) {
	m, _ := openVolumes(t)
	m = press(t, m, key(KeyShiftTab)) // previous tab
	if m.inKind() {
		t.Fatal("still in the volumes on another tab")
	}
	m = click(t, m, "More")
	if !m.inKind() || m.activeName() != commands.TabPersistentVolumes {
		t.Errorf("back on More: inKind = %v, active = %q", m.inKind(), m.activeName())
	}
}
