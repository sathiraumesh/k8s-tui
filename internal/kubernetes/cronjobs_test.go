package kubernetes

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func fakeCronJob() *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "backup", UID: "uid-1"},
		Spec: batchv1.CronJobSpec{
			Schedule: "@daily",
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backup"}},
				Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "backup", Image: "busybox"}},
				}}},
			},
		},
	}
}

func TestRunCronJob(t *testing.T) {
	cs := fake.NewClientset(fakeCronJob())
	name, err := NewClientFromClientset(cs).RunCronJob(context.Background(), "apps", "backup", false)
	if err != nil || !strings.HasPrefix(name, "backup-manual-") {
		t.Fatalf("job %q, err %v", name, err)
	}
	j, err := cs.BatchV1().Jobs("apps").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil || j.Labels["app"] != "backup" || j.OwnerReferences[0].Name != "backup" || j.Spec.Template.Spec.Containers[0].Image != "busybox" {
		t.Errorf("job = %+v, err = %v", j, err)
	}
}

func TestSuspendCronJob(t *testing.T) {
	cs := fake.NewClientset(fakeCronJob())
	c := NewClientFromClientset(cs)
	if err := c.SuspendCronJob(context.Background(), "apps", "backup", true, false); err != nil {
		t.Fatal(err)
	}
	cj, _ := cs.BatchV1().CronJobs("apps").Get(context.Background(), "backup", metav1.GetOptions{})
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Error("not suspended")
	}
	if err := c.SuspendCronJob(context.Background(), "apps", "backup", true, false); err == nil {
		t.Error("twice: err = nil")
	}
}

func TestCreateCronJob(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateCronJob(context.Background(), "apps", "tick", "busybox", "*/5 * * * *", []string{"date"}, false); err != nil {
		t.Fatal(err)
	}
	cj, _ := cs.BatchV1().CronJobs("apps").Get(context.Background(), "tick", metav1.GetOptions{})
	if cj.Spec.Schedule != "*/5 * * * *" || cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command[0] != "date" {
		t.Errorf("cronjob = %+v", cj.Spec)
	}
	if err := c.CreateCronJob(context.Background(), "apps", "x", "busybox", "", nil, false); err == nil {
		t.Error("no schedule: err = nil")
	}
}
