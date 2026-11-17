package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestScaleReplicationController(t *testing.T) {
	cs := fake.NewClientset(&corev1.ReplicationController{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "old"}})
	ch, err := NewClientFromClientset(cs).ScaleReplicationController(context.Background(), "apps", "old", "3", false)
	if err != nil || ch.After[0] != "replicas=3" {
		t.Errorf("scale: %+v, %v", ch, err)
	}
}
