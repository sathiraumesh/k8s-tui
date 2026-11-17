package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// A save refused because the object changed since it was read (its
// controller wrote its status) is tried again: read, edited, saved.
func TestUpdateRetriesOnConflict(t *testing.T) {
	cs := fake.NewClientset(fakePod("apps", "web"))
	conflicts := 2
	cs.PrependReactor("update", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		if conflicts > 0 {
			conflicts--
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "web", nil)
		}
		return false, nil, nil
	})
	edits := 0
	_, after, err := update(context.Background(), cs.CoreV1().Pods("apps"), "web", false, func(p *corev1.Pod) error {
		edits++
		p.Labels = map[string]string{"e2e": "yes"}
		return nil
	})
	if err != nil || after.Labels["e2e"] != "yes" || edits != 3 {
		t.Errorf("after = %v, edits = %d, err = %v", after, edits, err)
	}
}
