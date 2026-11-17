package kubernetes

import (
	"context"
	"slices"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestJobLogs(t *testing.T) {
	pod := fakePod("apps", "pi-abc")
	pod.Labels = map[string]string{jobPodLabel: "pi"}
	other := fakePod("apps", "web")
	logs, err := NewClientFromClientset(fake.NewClientset(pod, other)).JobLogs(context.Background(), "apps", "pi", 10)
	if err != nil || !strings.Contains(logs, "==> pi-abc/app <==") || strings.Contains(logs, "web") {
		t.Errorf("logs = %q, err = %v", logs, err)
	}
	if _, err := NewClientFromClientset(fake.NewClientset()).JobLogs(context.Background(), "apps", "pi", 10); err == nil {
		t.Error("no pods: err = nil")
	}
}

func TestSuspendJob(t *testing.T) {
	cs := fake.NewClientset(&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "pi"}})
	c := NewClientFromClientset(cs)
	if err := c.SuspendJob(context.Background(), "apps", "pi", true, false); err != nil {
		t.Fatal(err)
	}
	j, _ := cs.BatchV1().Jobs("apps").Get(context.Background(), "pi", metav1.GetOptions{})
	if j.Spec.Suspend == nil || !*j.Spec.Suspend {
		t.Error("not suspended")
	}
	if err := c.SuspendJob(context.Background(), "apps", "pi", true, false); err == nil {
		t.Error("suspending twice: err = nil")
	}
	if err := c.SuspendJob(context.Background(), "apps", "pi", false, false); err != nil {
		t.Errorf("resume: %v", err)
	}
}

func TestCreateJob(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateJob(context.Background(), "apps", "hello", "busybox", []string{"echo", "hi"}, false); err != nil {
		t.Fatal(err)
	}
	j, _ := cs.BatchV1().Jobs("apps").Get(context.Background(), "hello", metav1.GetOptions{})
	ct := j.Spec.Template.Spec.Containers[0]
	if ct.Image != "busybox" || !slices.Equal(ct.Command, []string{"echo", "hi"}) || j.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("job = %+v", j.Spec)
	}
	if err := c.CreateJob(context.Background(), "apps", "x", "", nil, false); err == nil {
		t.Error("no image: err = nil")
	}
}
