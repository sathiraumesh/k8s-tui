package mvc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// podsClient is a fake cluster with one node and one pod.
func podsClient() *fake.Clientset {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web", CreationTimestamp: metav1.Now()},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	return fake.NewClientset(readyNode("worker1"), pod)
}

func TestLoadPods(t *testing.T) {
	msg := commands.LoadPods(k8s.NewClientFromClientset(podsClient()), false, "")().(commands.ItemsLoadedMsg)
	if msg.Err != nil || msg.Tab != tabPods || len(msg.Rows) != 1 || !strings.HasPrefix(msg.Rows[0], "default   web   0/1   Running") {
		t.Errorf("loadPods = %+v", msg)
	}
}

// podLists counts the calls that listed pods.
func podLists(cs *fake.Clientset) int {
	n := 0
	for _, a := range cs.Actions() {
		if a.GetVerb() == "list" && a.GetResource().Resource == "pods" {
			n++
		}
	}
	return n
}

// The pods are only listed while the Pods tab is on screen: not at start
// (on Nodes), not when Nodes refreshes, and again on every refresh of Pods.
func TestPodsOnlyLoadedOnPodsTab(t *testing.T) {
	cs := podsClient()
	m := New(k8s.NewClientFromClientset(cs))
	m.refreshEvery = time.Millisecond

	runBatch(t, m.Init())
	_, cmd := m.Update(refreshTickMsg{gen: m.tickGen})
	runBatch(t, cmd)
	if n := podLists(cs); n != 0 {
		t.Fatalf("pods listed %d times while on Nodes, want 0", n)
	}

	next, cmd := m.Update(right) // to Pods
	m = next.(Model)
	msgs := runBatch(t, cmd)
	if podLists(cs) != 1 || !hasLoaded(msgs, tabPods) {
		t.Fatalf("opening Pods listed pods %d times, want 1", podLists(cs))
	}
	_, cmd = m.Update(refreshTickMsg{gen: m.tickGen})
	runBatch(t, cmd)
	if podLists(cs) != 2 {
		t.Errorf("a refresh on Pods listed pods %d times in all, want 2", podLists(cs))
	}
}

// ---------- actions on a pod ----------

func TestPodTarget(t *testing.T) {
	if got := commands.NamespacedTarget("kube-system   coredns-abc   1/1   Running", ""); got != "kube-system/coredns-abc" {
		t.Errorf("podTarget = %q", got)
	}
	if got := commands.NamespacedArgs("kube-system/coredns-abc"); got != "coredns-abc -n kube-system" {
		t.Errorf("podArgs = %q", got)
	}
}

// enter on a pod opens its actions, for namespace/name.
func TestEnterOpensPodActions(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey) // tabs → buttons → list → enter
	if m.overlay != overlayActions || m.target != "default/web" || m.overlayTitle() != "Actions: default/web" {
		t.Fatalf("overlay = %d, target = %q, title = %q", m.overlay, m.target, m.overlayTitle())
	}
	if m.kubectlCommand() != "kubectl describe pod web -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// Describe a pod shows kubectl's describe text on a page.
func TestDescribePodShowsText(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("d"))
	if m.kubectlCommand() != "kubectl describe pod web -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := m.Update(enterKey)
	m = run(t, m, cmd)
	text := strings.Join(m.text, "\n")
	if m.overlay != overlayText || !strings.Contains(text, "Name:") || !strings.Contains(text, "web") {
		t.Errorf("overlay = %d, text:\n%s", m.overlay, text)
	}
	if m.overlayTitle() != "Describe default/web" {
		t.Errorf("title = %q", m.overlayTitle())
	}
}

// The run images keep their own target, the namespace, as it is.
func TestRunKeepsNamespaceTarget(t *testing.T) {
	m, _ := podsOnScreen(t)
	if m = press(t, m, key(KeyOwn)); m.kubectlCommand() != "kubectl run nginx --image=nginx:latest -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// openPodAction opens the action with key k for the pod "default/web".
func openPodAction(t *testing.T, k string) (Model, *fake.Clientset) {
	t.Helper()
	m, cs := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key(k))
	if k == "o" || k == "x" || k == "i" { // the pod's containers load first: it has one
		m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"web"}})
	}
	return m, cs
}

// A pod with several containers asks which one before it shows logs.
func TestPodLogsPickContainer(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("o"))
	m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"app", "sidecar"}})
	if m.overlay != overlayContainers {
		t.Fatalf("overlay = %d, want the container picker", m.overlay)
	}
	if got := m.containerRows(); len(got) != 3 || got[0] != "all containers" || got[2] != "sidecar" {
		t.Errorf("rows = %q", got)
	}
	m = press(t, m, down, down, enterKey) // sidecar
	if m.overlay != overlayConfirm || m.kubectlCommand() != "kubectl logs web -n default --container=sidecar --tail=500" {
		t.Errorf("overlay = %d, command = %q", m.overlay, m.kubectlCommand())
	}
}

// Shell asks which container too, without "all containers".
func TestPodShellPickContainer(t *testing.T) {
	m, _ := podsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("x"))
	m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"app", "sidecar"}})
	if got := m.containerRows(); m.overlay != overlayContainers || len(got) != 2 || got[0] != "app" {
		t.Fatalf("overlay = %d, rows = %q", m.overlay, got)
	}
	m = press(t, m, down, enterKey) // sidecar
	if m.kubectlCommand() != "kubectl exec -it web -n default --container=sidecar -- sh" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// Every action writes the pod as kubectl takes it, NAME -n NAMESPACE.
func TestPodActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe pod web -n default",
		"o": "kubectl logs web -n default --all-containers --tail=500",
		"x": "kubectl exec -it web -n default -- sh",
		"l": "kubectl label pod web -n default",
		"i": "kubectl set image pod/web -n default web=",
		"r": "kubectl delete pod web -n default",
	} {
		if m, _ := openPodAction(t, k); m.kubectlCommand() != want {
			t.Errorf("%s: command = %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

// Logs open scrolled to the end: the newest lines are on screen.
func TestPodLogsOpenAtEnd(t *testing.T) {
	m, _ := openPodAction(t, "o")
	var lines []string
	for i := range 100 {
		lines = append(lines, fmt.Sprint("line ", i))
	}
	m = press(t, m, commands.ActionDoneMsg{Action: "Logs", Target: "default/web", Text: strings.Join(lines, "\n"), AtEnd: true})
	if m.overlay != overlayText || m.textOffset != 100-m.listHeight() {
		t.Errorf("overlay = %d, offset = %d, want the end", m.overlay, m.textOffset)
	}
	if !strings.Contains(m.View(), "line 99") {
		t.Error("the last line isn't on screen")
	}
}

func TestPodLogsRun(t *testing.T) {
	m, _ := openPodAction(t, "o")
	m = press(t, m, spaceKey, down, spaceKey) // --previous, --timestamps
	if m.kubectlCommand() != "kubectl logs web -n default --all-containers --tail=500 --previous --timestamps" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := m.Update(enterKey)
	if m = run(t, m, cmd); m.overlay != overlayText || !strings.Contains(strings.Join(m.text, "\n"), "fake logs") {
		t.Errorf("overlay = %d, text = %q", m.overlay, m.text)
	}
}

func TestPodShowRuns(t *testing.T) {
	// Show has no flags: it runs straight away.
	m, _ := podsOnScreen(t)
	next, cmd := press(t, m, down, down, down, enterKey).Update(key("s"))
	m = run(t, next.(Model), cmd)
	if got := strings.Join(m.text, "\n"); !strings.Contains(got, "Labels:") || !strings.Contains(got, "Annotations:") {
		t.Errorf("Show page:\n%s", got)
	}
}

func TestPodShellBash(t *testing.T) {
	m, _ := openPodAction(t, "x")
	if m = press(t, m, spaceKey); m.kubectlCommand() != "kubectl exec -it web -n default -- bash" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	// The shell doesn't change the cluster: enter opens it.
	if _, cmd := m.Update(enterKey); cmd == nil {
		t.Error("enter didn't open the shell")
	}
}

func TestPodLabelRuns(t *testing.T) {
	m, cs := openPodAction(t, "l")
	m = typeText(t, m, "env=prod")
	_, cmd := press(t, m, enterKey).Update(key("y"))
	if m = run(t, m, cmd); m.notice != "Label default/web: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if p, _ := cs.CoreV1().Pods("default").Get(context.Background(), "web", metav1.GetOptions{}); p.Labels["env"] != "prod" {
		t.Errorf("labels = %v", p.Labels)
	}
}

// Delete asks for y, then the pod is gone and the list reloads.
func TestPodDeleteRuns(t *testing.T) {
	m, cs := openPodAction(t, "r")
	if _, cmd := m.Update(enterKey); cmd != nil {
		t.Fatal("enter deleted the pod, want only y to")
	}
	next, cmd := m.Update(key("y"))
	next, reload := next.(Model).Update(cmd())
	if m = next.(Model); m.notice != "Delete default/web: done" || reload == nil {
		t.Errorf("notice = %q, reload = %v", m.notice, reload != nil)
	}
	if _, err := cs.CoreV1().Pods("default").Get(context.Background(), "web", metav1.GetOptions{}); err == nil {
		t.Error("the pod is still there")
	}
}

func TestPodDeleteNow(t *testing.T) {
	m, _ := openPodAction(t, "r")
	if m = press(t, m, spaceKey); m.kubectlCommand() != "kubectl delete pod web -n default --now" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// Every pod action has its own key, a kubectl command and a run.
func TestPodActionsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range commands.PodActions() {
		if a.Key == "" || seen[a.Key] || a.Key == KeyJ || a.Key == KeyK || !strings.HasPrefix(a.Command, "kubectl ") || a.Run == nil {
			t.Errorf("action %q: key %q used twice, moves the list, or is incomplete", a.Name, a.Key)
		}
		seen[a.Key] = true
	}
}
