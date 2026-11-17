package kubernetes

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDaemonSetActions(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "logs"},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "fluentbit", Image: "fluent-bit:3"}},
		}}},
	}
	cs := fake.NewClientset(ds)
	c := NewClientFromClientset(cs)
	ctx := context.Background()
	if got, _ := c.DaemonSetContainers(ctx, "kube-system", "logs", false); !slices.Equal(got, []string{"fluentbit"}) {
		t.Errorf("containers = %q", got)
	}
	if err := c.RestartDaemonSet(ctx, "kube-system", "logs", false); err != nil {
		t.Fatal(err)
	}
	got, _ := cs.AppsV1().DaemonSets("kube-system").Get(ctx, "logs", metav1.GetOptions{})
	if got.Spec.Template.Annotations[restartedAtAnnotation] == "" {
		t.Error("no restart stamp")
	}
	ch, err := c.SetDaemonSetImage(ctx, "kube-system", "logs", []string{"fluentbit=fluent-bit:4"}, false)
	if err != nil || !slices.Equal(ch.After, []string{"fluentbit=fluent-bit:4"}) {
		t.Errorf("set image: %+v, %v", ch, err)
	}
}
