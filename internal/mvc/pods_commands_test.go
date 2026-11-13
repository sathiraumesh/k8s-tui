package mvc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var podsNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func TestPodRows(t *testing.T) {
	pods := []k8s.Pod{
		{Namespace: "default", Name: "web", Ready: "1/1", Status: "Running", CreatedAt: podsNow.Add(-3 * time.Hour), IP: "10.0.0.5", Node: "worker1"},
		{Namespace: "kube-system", Name: "dns", Ready: "0/1", Status: "CrashLoopBackOff", Restarts: 4, LastRestart: podsNow.Add(-5 * time.Minute), CreatedAt: podsNow.Add(-8 * 24 * time.Hour)},
	}
	got := podRows(pods, false, true, podsNow)
	want := []string{
		"default       web   1/1   Running            0            3h",
		"kube-system   dns   0/1   CrashLoopBackOff   4 (5m ago)   8d",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	wide := podRows(pods, true, true, podsNow)
	if !strings.HasSuffix(wide[0], "10.0.0.5   worker1   <none>   <none>") || !strings.HasSuffix(wide[1], "8d   <none>     <none>    <none>   <none>") {
		t.Errorf("wide rows =\n%s", strings.Join(wide, "\n"))
	}
}

func TestPodAge(t *testing.T) {
	if got := age(time.Time{}, podsNow); got != "<unknown>" {
		t.Errorf("age of zero time = %q", got)
	}
	if got := age(podsNow.Add(-45*time.Second), podsNow); got != "45s" {
		t.Errorf("age = %q, want 45s", got)
	}
}

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
	msg := loadPods(k8s.NewClientFromClientset(podsClient()), false, "")().(itemsLoadedMsg)
	if msg.err != nil || msg.tab != tabPods || len(msg.rows) != 1 || !strings.HasPrefix(msg.rows[0], "default   web   0/1   Running") {
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
	if got := podTarget("kube-system   coredns-abc   1/1   Running", ""); got != "kube-system/coredns-abc" {
		t.Errorf("podTarget = %q", got)
	}
	if got := podArgs("kube-system/coredns-abc"); got != "coredns-abc -n kube-system" {
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
	return press(t, m, down, down, down, enterKey, key(k)), cs
}

// Every action writes the pod as kubectl takes it, NAME -n NAMESPACE.
func TestPodActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe pod web -n default",
		"o": "kubectl logs web -n default --all-containers --tail=500",
		"x": "kubectl exec -it web -n default -- sh",
		"l": "kubectl label pod web -n default",
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
	m = press(t, m, actionDoneMsg{action: "Logs", target: "default/web", text: strings.Join(lines, "\n"), atEnd: true})
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
	for _, a := range podActions() {
		if a.key == "" || seen[a.key] || a.key == KeyJ || a.key == KeyK || !strings.HasPrefix(a.command, "kubectl ") || a.run == nil {
			t.Errorf("action %q: key %q used twice, moves the list, or is incomplete", a.name, a.key)
		}
		seen[a.key] = true
	}
}
