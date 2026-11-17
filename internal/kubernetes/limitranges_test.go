package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateDefaultLimits(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateDefaultLimits(context.Background(), "apps", "defaults", "500m", "256Mi", false); err != nil {
		t.Fatal(err)
	}
	lr, _ := cs.CoreV1().LimitRanges("apps").Get(context.Background(), "defaults", metav1.GetOptions{})
	item := lr.Spec.Limits[0]
	cpu, mem := item.Default[corev1.ResourceCPU], item.DefaultRequest[corev1.ResourceMemory]
	if item.Type != corev1.LimitTypeContainer || cpu.String() != "500m" || mem.String() != "256Mi" {
		t.Errorf("limits = %+v", item)
	}
	if err := c.CreateDefaultLimits(context.Background(), "apps", "x", "lots", "1Gi", false); err == nil {
		t.Error("bad cpu: err = nil")
	}
}
