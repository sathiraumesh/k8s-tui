package kubernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateNetworkPolicy(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateNetworkPolicy(context.Background(), "apps", "deny", DenyAllIngress, false); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateNetworkPolicy(context.Background(), "apps", "same", AllowSameNamespace, false); err != nil {
		t.Fatal(err)
	}
	deny, _ := cs.NetworkingV1().NetworkPolicies("apps").Get(context.Background(), "deny", metav1.GetOptions{})
	same, _ := cs.NetworkingV1().NetworkPolicies("apps").Get(context.Background(), "same", metav1.GetOptions{})
	if len(deny.Spec.Ingress) != 0 || len(deny.Spec.PolicyTypes) != 1 || len(deny.Spec.PodSelector.MatchLabels) != 0 {
		t.Errorf("deny = %+v", deny.Spec)
	}
	if len(same.Spec.Ingress) != 1 || same.Spec.Ingress[0].From[0].PodSelector == nil {
		t.Errorf("same = %+v", same.Spec)
	}
}
