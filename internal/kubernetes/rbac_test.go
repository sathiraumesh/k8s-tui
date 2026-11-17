package kubernetes

import (
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// rbacCluster is a fake cluster serving pods (v1) and deployments (apps/v1).
func rbacCluster() *fake.Clientset {
	cs := fake.NewClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}}},
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}},
	}
	return cs
}

func TestCreateRole(t *testing.T) {
	cs := rbacCluster()
	c := NewClientFromClientset(cs)
	if err := c.CreateRole(context.Background(), "apps", "reader", []string{"get", "list"}, []string{"pods", "deployments", "widgets.example.com"}, false); err != nil {
		t.Fatal(err)
	}
	role, _ := cs.RbacV1().Roles("apps").Get(context.Background(), "reader", metav1.GetOptions{})
	if len(role.Rules) != 3 {
		t.Fatalf("rules = %+v", role.Rules)
	}
	// One rule per group, sorted: core (""), apps, example.com.
	if role.Rules[0].APIGroups[0] != "" || !slices.Equal(role.Rules[0].Resources, []string{"pods"}) ||
		role.Rules[1].APIGroups[0] != "apps" || role.Rules[2].APIGroups[0] != "example.com" || !slices.Equal(role.Rules[0].Verbs, []string{"get", "list"}) {
		t.Errorf("rules = %+v", role.Rules)
	}
	if err := c.CreateRole(context.Background(), "apps", "x", []string{"get"}, []string{"nopes"}, false); err == nil {
		t.Error("unknown resource: err = nil")
	}
	if err := c.CreateRole(context.Background(), "", "admin", []string{"*"}, []string{"*"}, false); err != nil {
		t.Fatal(err)
	}
	if cr, _ := cs.RbacV1().ClusterRoles().Get(context.Background(), "admin", metav1.GetOptions{}); cr.Rules[0].APIGroups[0] != "*" {
		t.Errorf("cluster role = %+v", cr.Rules)
	}
}
