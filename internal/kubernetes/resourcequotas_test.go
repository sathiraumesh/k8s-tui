package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateResourceQuota(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateResourceQuota(context.Background(), "apps", "team", []string{"pods=10", "limits.memory=8Gi"}, false); err != nil {
		t.Fatal(err)
	}
	q, _ := cs.CoreV1().ResourceQuotas("apps").Get(context.Background(), "team", metav1.GetOptions{})
	pods, mem := q.Spec.Hard[corev1.ResourcePods], q.Spec.Hard[corev1.ResourceLimitsMemory]
	if pods.String() != "10" || mem.String() != "8Gi" {
		t.Errorf("hard = %v", q.Spec.Hard)
	}
	for _, bad := range [][]string{nil, {"pods"}, {"pods=lots"}} {
		if err := c.CreateResourceQuota(context.Background(), "apps", "x", bad, false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
}
