package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestBindPod(t *testing.T) {
	var bound *corev1.Binding
	cs := fake.NewClientset()
	cs.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "binding" {
			return false, nil, nil
		}
		bound = a.(k8stesting.CreateAction).GetObject().(*corev1.Binding)
		return true, bound, nil
	})
	if err := NewClientFromClientset(cs).BindPod(context.Background(), "apps", "web", "worker1", false); err != nil {
		t.Fatal(err)
	}
	if bound.Name != "web" || bound.Target.Kind != "Node" || bound.Target.Name != "worker1" {
		t.Errorf("binding = %+v", bound)
	}
	if err := NewClientFromClientset(cs).BindPod(context.Background(), "apps", "web", "", false); err == nil {
		t.Error("no node: err = nil")
	}
}
