package kubernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreatePriorityClass(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreatePriorityClass(context.Background(), "high", "1000", false); err != nil {
		t.Fatal(err)
	}
	if pc, _ := cs.SchedulingV1().PriorityClasses().Get(context.Background(), "high", metav1.GetOptions{}); pc.Value != 1000 {
		t.Errorf("value = %d", pc.Value)
	}
	for _, bad := range []string{"", "high", "2000000000"} {
		if err := c.CreatePriorityClass(context.Background(), "x", bad, false); err == nil {
			t.Errorf("%q: err = nil", bad)
		}
	}
}
