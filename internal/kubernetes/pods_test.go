package kubernetes

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// fakePod is a pod with one container named app, running and ready.
func fakePod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}, NodeName: "worker1"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.5",
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
}

func TestListPodsSorted(t *testing.T) {
	cs := fake.NewClientset(fakePod("b", "web"), fakePod("a", "zeta"), fakePod("a", "alpha"))
	pods, err := NewClientFromClientset(cs).ListPods(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pods {
		got = append(got, p.Namespace+"/"+p.Name)
	}
	want := []string{"a/alpha", "a/zeta", "b/web"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("pods = %v, want %v", got, want)
	}
}

func TestListPodsInNamespace(t *testing.T) {
	cs := fake.NewClientset(fakePod("a", "one"), fakePod("b", "two"))
	pods, err := NewClientFromClientset(cs).ListPods(context.Background(), "b")
	if err != nil || len(pods) != 1 || pods[0].Name != "two" {
		t.Errorf("pods = %+v, err = %v, want only b/two", pods, err)
	}
}

func TestListPodsError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := NewClientFromClientset(cs).ListPods(context.Background(), ""); err == nil {
		t.Error("err = nil, want forbidden")
	}
}

func TestToPod(t *testing.T) {
	p := fakePod("default", "web")
	p.Spec.ReadinessGates = []corev1.PodReadinessGate{{ConditionType: "example.com/ready"}}
	p.Status.Conditions = []corev1.PodCondition{{Type: "example.com/ready", Status: corev1.ConditionTrue}}
	got := toPod(*p)
	if got.Ready != "1/1" || got.Status != "Running" || got.IP != "10.0.0.5" || got.Node != "worker1" || got.ReadinessGates != "1/1" {
		t.Errorf("toPod = %+v", got)
	}
}

// waiting, terminated and running are container states for the cases below.
func waiting(reason string) corev1.ContainerState {
	return corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}
}
func terminated(reason string, code int32) corev1.ContainerState {
	return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reason, ExitCode: code}}
}

var running = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}

// The STATUS column, case by case, as kubectl shows it.
func TestPodStatus(t *testing.T) {
	always := corev1.ContainerRestartPolicyAlways
	for _, tc := range []struct {
		name  string
		edit  func(p *corev1.Pod)
		want  string
		ready int
	}{
		{"running", func(*corev1.Pod) {}, "Running", 1},
		{"pending, no status yet", func(p *corev1.Pod) {
			p.Status = corev1.PodStatus{Phase: corev1.PodPending}
		}, "Pending", 0},
		{"crash loop", func(p *corev1.Pod) {
			p.Status.ContainerStatuses[0] = corev1.ContainerStatus{Name: "app", State: waiting("CrashLoopBackOff")}
		}, "CrashLoopBackOff", 0},
		{"completed", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodSucceeded
			p.Status.ContainerStatuses[0] = corev1.ContainerStatus{Name: "app", State: terminated("Completed", 0)}
		}, "Completed", 0},
		{"exit code without reason", func(p *corev1.Pod) {
			p.Status.ContainerStatuses[0] = corev1.ContainerStatus{Name: "app", State: terminated("", 3)}
		}, "ExitCode:3", 0},
		{"evicted", func(p *corev1.Pod) {
			p.Status = corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"}
		}, "Evicted", 0},
		{"init container 1 of 2", func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "a"}, {Name: "b"}}
			p.Status.Phase = corev1.PodPending
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{
				{Name: "a", State: terminated("Completed", 0)},
				{Name: "b", State: running},
			}
		}, "Init:1/2", 0},
		{"init container crashing", func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "a"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "a", State: waiting("CrashLoopBackOff")}}
		}, "Init:CrashLoopBackOff", 0},
		{"init container failed", func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "a"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "a", State: terminated("Error", 1)}}
		}, "Init:Error", 0},
		{"started sidecar doesn't hold it back", func(p *corev1.Pod) {
			started := true
			p.Spec.InitContainers = []corev1.Container{{Name: "proxy", RestartPolicy: &always}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "proxy", State: running, Started: &started}}
		}, "Running", 1},
		{"one done, one running, pod ready", func(p *corev1.Pod) {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "job"})
			p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: "job", State: terminated("Completed", 0)})
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		}, "Running", 1},
		{"one done, one running, pod not ready", func(p *corev1.Pod) {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "job"})
			p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: "job", State: terminated("Completed", 0)})
		}, "NotReady", 1},
		{"being deleted", func(p *corev1.Pod) {
			now := metav1.Now()
			p.DeletionTimestamp = &now
		}, "Terminating", 1},
	} {
		p := fakePod("default", "web")
		tc.edit(p)
		got := toPod(*p)
		if got.Status != tc.want || got.Ready != fmtReady(tc.ready, len(p.Spec.Containers)) {
			t.Errorf("%s: status = %q, ready = %q, want %q, %d ready", tc.name, got.Status, got.Ready, tc.want, tc.ready)
		}
	}
}

func fmtReady(ready, all int) string {
	return string(rune('0'+ready)) + "/" + string(rune('0'+all))
}

// Once the pod has started, the restarts are its containers' and its
// sidecars', not those of the init containers that ran before.
func TestPodRestarts(t *testing.T) {
	early := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	late := metav1.NewTime(early.Add(time.Hour))
	p := fakePod("default", "web")
	p.Spec.InitContainers = []corev1.Container{{Name: "init"}}
	p.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "init", State: terminated("Completed", 0), RestartCount: 1,
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: early}},
	}}
	p.Status.ContainerStatuses[0].RestartCount = 2
	p.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: late}}

	got := toPod(*p)
	if got.Restarts != 2 || !got.LastRestart.Equal(late.Time) {
		t.Errorf("restarts = %d, last = %v, want 2 at %v", got.Restarts, got.LastRestart, late)
	}

	// A sidecar's restarts do count.
	always := corev1.ContainerRestartPolicyAlways
	started := true
	p.Spec.InitContainers[0].RestartPolicy = &always
	p.Status.InitContainerStatuses[0].State = running
	p.Status.InitContainerStatuses[0].Started = &started
	if got := toPod(*p); got.Restarts != 3 {
		t.Errorf("with a sidecar: restarts = %d, want 3", got.Restarts)
	}
}

// While an init container is still running, its restarts are the ones shown.
func TestPodRestartsWhileInitializing(t *testing.T) {
	p := fakePod("default", "web")
	p.Spec.InitContainers = []corev1.Container{{Name: "init"}}
	p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init", State: waiting("CrashLoopBackOff"), RestartCount: 6}}
	if got := toPod(*p); got.Restarts != 6 || got.Status != "Init:CrashLoopBackOff" {
		t.Errorf("restarts = %d, status = %q, want 6, Init:CrashLoopBackOff", got.Restarts, got.Status)
	}
}
