package mvc

import (
	"context"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// podsOnScreen is a model on the Pods tab of a fake cluster with one pod.
func podsOnScreen(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	cs := podsClient()
	m := sized(t, New(k8s.NewClientFromClientset(cs)), 120, 30)
	m.active = 1
	m = press(t, m, itemsLoadedMsg{tab: tabPods, rows: []string{"default   web   1/1   Running   0   3h"}, at: time.Now()})
	return m, cs
}

// Pods has a "+ run" button; the other tabs don't.
func TestRunButtonOnlyOnPods(t *testing.T) {
	m, _ := podsOnScreen(t)
	if b := buttonNamed(t, m, buttonOwn); b.label != "+ run" {
		t.Errorf("run button = %+v", b)
	}
	m.active = 0 // Nodes
	for _, b := range m.panelButtons() {
		if b.name == buttonOwn {
			t.Error("Nodes has a run button")
		}
	}
	if m = press(t, m, key(KeyOwn)); m.overlay != overlayNone {
		t.Error("+ opened something on Nodes, which has no own actions")
	}
}

// + (or the button) opens the images; enter picks one, with the name filled in.
func TestRunOpensImages(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, key(KeyOwn))
	if m.overlay != overlayActions || !m.tabActions || m.overlayTitle() != "Run a pod in default" {
		t.Fatalf("overlay = %d, tabActions = %v, title = %q", m.overlay, m.tabActions, m.overlayTitle())
	}
	out := m.View()
	for _, want := range []string{"nginx:latest", "busybox:latest", "other image"} {
		if !strings.Contains(out, want) {
			t.Errorf("image list is missing %q", want)
		}
	}
	if m.kubectlCommand() != "kubectl run nginx --image=nginx:latest -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}

	m = press(t, m, key("b")) // busybox
	if m.overlay != overlayConfirm || m.input != "busybox" || !m.typing() {
		t.Errorf("after b: overlay = %d, input = %q, typing = %v", m.overlay, m.input, m.typing())
	}
	if m.overlayTitle() != "busybox:latest in default" {
		t.Errorf("title = %q", m.overlayTitle())
	}
}

// The button opens the same list.
func TestRunButtonOpensImages(t *testing.T) {
	m, _ := podsOnScreen(t)
	for i, b := range m.panelButtons() {
		if b.name == buttonOwn {
			m.focus, m.button = focusButtons, i
		}
	}
	if m = press(t, m, enterKey); m.overlay != overlayActions || !m.tabActions {
		t.Errorf("run button: overlay = %d, tabActions = %v", m.overlay, m.tabActions)
	}
}

// y runs the pod on the cluster, then the list reloads.
func TestRunPodRuns(t *testing.T) {
	m, cs := podsOnScreen(t)
	m = press(t, m, key(KeyOwn), key("n"), backspaceKey, backspaceKey, backspaceKey, backspaceKey, backspaceKey)
	m = typeText(t, m, "site")
	if m.kubectlCommand() != "kubectl run site --image=nginx:latest -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	next, cmd := press(t, m, enterKey).Update(key("y"))
	m = next.(Model)
	if m.overlay != overlayNone || m.tabActions {
		t.Errorf("while running: overlay = %d, tabActions = %v", m.overlay, m.tabActions)
	}
	next, reload := m.Update(cmd())
	if m = next.(Model); m.notice != "Run default/site: done" || reload == nil {
		t.Errorf("notice = %q, reload = %v", m.notice, reload != nil)
	}
	p, err := cs.CoreV1().Pods("default").Get(context.Background(), "site", metav1.GetOptions{})
	if err != nil || p.Spec.Containers[0].Image != "nginx:latest" {
		t.Errorf("pod = %+v, err = %v", p, err)
	}
	// Back on the list, enter is for a pod's actions again, not the images.
	if len(m.actions()) != 0 {
		t.Error("the image list is still the tab's actions after running")
	}
}

// Another image: the name and the image are typed.
func TestRunOtherImage(t *testing.T) {
	m, cs := podsOnScreen(t)
	m = typeText(t, press(t, m, key(KeyOwn), key("o")), "db postgres:16")
	if m.kubectlCommand() != "kubectl run db --image=postgres:16 -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key("y"))
	m = run(t, m, cmd)
	if p, err := cs.CoreV1().Pods("default").Get(context.Background(), "db", metav1.GetOptions{}); err != nil || p.Spec.Containers[0].Image != "postgres:16" {
		t.Errorf("pod = %+v, err = %v", p, err)
	}
}

func TestRunOtherImageNeedsBoth(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = typeText(t, press(t, m, key(KeyOwn), key("o")), "db")
	_, cmd := press(t, m, enterKey).Update(key("y"))
	if m = run(t, m, cmd); !strings.Contains(m.notice, "failed: type a name and an image") {
		t.Errorf("notice = %q", m.notice)
	}
}

// esc goes back through the pages: confirm → images → list.
func TestRunEscBack(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, key(KeyOwn), key("n"), escKey)
	if m.overlay != overlayActions || !m.tabActions {
		t.Fatalf("esc from confirm: overlay = %d, tabActions = %v", m.overlay, m.tabActions)
	}
	if m = press(t, m, escKey); m.overlay != overlayNone || m.tabActions {
		t.Errorf("esc from images: overlay = %d, tabActions = %v", m.overlay, m.tabActions)
	}
}

func TestImageName(t *testing.T) {
	for image, want := range map[string]string{
		"nginx:latest":                   "nginx",
		"docker.io/library/nginx:latest": "nginx",
		"ghcr.io/org/app@sha256:abc":     "app",
		"redis":                          "redis",
	} {
		if got := imageName(image); got != want {
			t.Errorf("imageName(%q) = %q, want %q", image, got, want)
		}
	}
}

// Every image action has its own key, not one the action list moves with.
func TestRunPodActionsKeys(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range runPodActions() {
		if seen[a.key] || a.key == KeyJ || a.key == KeyK {
			t.Errorf("key %q of %s is used twice or moves the list", a.key, a.name)
		}
		seen[a.key] = true
	}
}

// The help line on Pods shows "+ run"; Nodes' doesn't.
func TestRunInHelp(t *testing.T) {
	m, _ := podsOnScreen(t)
	if h := m.helpText(); !strings.Contains(h, "• + run •") {
		t.Errorf("Pods help = %q", h)
	}
	m.active = 0
	if h := m.helpText(); strings.Contains(h, "+ run") {
		t.Errorf("Nodes help = %q", h)
	}
}
