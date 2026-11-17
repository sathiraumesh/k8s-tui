package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestScaleReplicaSet(t *testing.T) {
	cs := fake.NewClientset(&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "web-abc"}})
	ch, err := NewClientFromClientset(cs).ScaleReplicaSet(context.Background(), "apps", "web-abc", "2", false)
	if err != nil || ch.Before[0] != "replicas=1" || ch.After[0] != "replicas=2" {
		t.Errorf("scale: %+v, %v", ch, err) // unset replicas are 1
	}
}
