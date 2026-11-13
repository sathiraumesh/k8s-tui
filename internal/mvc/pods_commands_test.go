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

var podsNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func TestPodRows(t *testing.T) {
	pods := []k8s.Pod{
		{Namespace: "default", Name: "web", Ready: "1/1", Status: "Running", CreatedAt: podsNow.Add(-3 * time.Hour), IP: "10.0.0.5", Node: "worker1"},
		{Namespace: "kube-system", Name: "dns", Ready: "0/1", Status: "CrashLoopBackOff", Restarts: 4, LastRestart: podsNow.Add(-5 * time.Minute), CreatedAt: podsNow.Add(-8 * 24 * time.Hour)},
	}
	got := podRows(pods, false, podsNow)
	want := []string{
		"default       web   1/1   Running            0            3h",
		"kube-system   dns   0/1   CrashLoopBackOff   4 (5m ago)   8d",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	wide := podRows(pods, true, podsNow)
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
	msg := loadPods(k8s.NewClientFromClientset(podsClient()), false)().(itemsLoadedMsg)
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
