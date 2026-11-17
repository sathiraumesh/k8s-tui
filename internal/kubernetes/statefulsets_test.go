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

func fakeStatefulSet() *appsv1.StatefulSet {
	replicas := int32(3)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "db"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{Name: "init", Image: "busybox"}},
				Containers:     []corev1.Container{{Name: "postgres", Image: "postgres:16"}},
			}},
		},
	}
}

func TestStatefulSetActions(t *testing.T) {
	cs := fake.NewClientset(fakeStatefulSet())
	c := NewClientFromClientset(cs)
	ctx := context.Background()

	if got, _ := c.StatefulSetContainers(ctx, "apps", "db", true); !slices.Equal(got, []string{"postgres"}) {
		t.Errorf("running containers = %q", got)
	}
	ch, err := c.ScaleStatefulSet(ctx, "apps", "db", "1", false)
	if err != nil || ch.Before[0] != "replicas=3" || ch.After[0] != "replicas=1" {
		t.Errorf("scale: %+v, %v", ch, err)
	}
	if _, err := c.ScaleStatefulSet(ctx, "apps", "db", "-1", false); err == nil {
		t.Error("scale to -1: err = nil")
	}
	if err := c.RestartStatefulSet(ctx, "apps", "db", false); err != nil {
		t.Fatal(err)
	}
	s, _ := cs.AppsV1().StatefulSets("apps").Get(ctx, "db", metav1.GetOptions{})
	if s.Spec.Template.Annotations[restartedAtAnnotation] == "" {
		t.Error("no restart stamp")
	}
	ch, err = c.SetStatefulSetImage(ctx, "apps", "db", []string{"postgres=postgres:17"}, false)
	if err != nil || !slices.Contains(ch.After, "postgres=postgres:17") {
		t.Errorf("set image: %+v, %v", ch, err)
	}
}
