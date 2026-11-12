package kubernetes

import (
	"context"
	"errors"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// withResources returns a client on a fake cluster that serves lists.
func withResources(lists ...*metav1.APIResourceList) (*Client, *fake.Clientset) {
	cs := fake.NewClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).Resources = lists
	return NewClientFromClientset(cs), cs
}

var (
	coreV1 = &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", ShortNames: []string{"po"}, Namespaced: true, Kind: "Pod",
				Verbs: []string{"get", "list", "watch"}, Categories: []string{"all"}},
			{Name: "pods/log", Namespaced: true, Kind: "Pod", Verbs: []string{"get"}},
			{Name: "nodes", ShortNames: []string{"no"}, Kind: "Node", Verbs: []string{"get", "list"}},
		},
	}
	appsV1 = &metav1.APIResourceList{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", ShortNames: []string{"deploy"}, Namespaced: true, Kind: "Deployment",
				Verbs: []string{"get", "list"}},
		},
	}
	// The fake cluster prefers the first version it is given for a group.
	autoscalingV2 = &metav1.APIResourceList{
		GroupVersion: "autoscaling/v2",
		APIResources: []metav1.APIResource{
			{Name: "horizontalpodautoscalers", ShortNames: []string{"hpa"}, Namespaced: true, Kind: "HorizontalPodAutoscaler"},
		},
	}
	autoscalingV1 = &metav1.APIResourceList{
		GroupVersion: "autoscaling/v1",
		APIResources: []metav1.APIResource{
			{Name: "horizontalpodautoscalers", ShortNames: []string{"hpa"}, Namespaced: true, Kind: "HorizontalPodAutoscaler"},
		},
	}
)

func names(rs []Resource) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name+" "+r.APIVersion)
	}
	return out
}

func TestListResourcesSortedByName(t *testing.T) {
	c, _ := withResources(coreV1, appsV1)
	rs, err := c.ListResources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"deployments apps/v1", "nodes v1", "pods v1"}
	if got := names(rs); !slices.Equal(got, want) {
		t.Errorf("resources = %q, want %q", got, want)
	}
}

func TestListResourcesCopiesFields(t *testing.T) {
	c, _ := withResources(coreV1)
	rs, _ := c.ListResources(context.Background())
	pods := rs[slices.IndexFunc(rs, func(r Resource) bool { return r.Name == "pods" })]
	if !slices.Equal(pods.ShortNames, []string{"po"}) || pods.APIVersion != "v1" || !pods.Namespaced ||
		pods.Kind != "Pod" || !slices.Equal(pods.Verbs, []string{"get", "list", "watch"}) ||
		!slices.Equal(pods.Categories, []string{"all"}) {
		t.Errorf("pods = %+v", pods)
	}
	nodes := rs[slices.IndexFunc(rs, func(r Resource) bool { return r.Name == "nodes" })]
	if nodes.Namespaced {
		t.Error("nodes is namespaced, want cluster-wide")
	}
}

func TestListResourcesSkipsSubresources(t *testing.T) {
	c, _ := withResources(coreV1)
	rs, _ := c.ListResources(context.Background())
	for _, r := range rs {
		if r.Name == "pods/log" {
			t.Error("pods/log listed, want subresources left out")
		}
	}
}

// Only the version the cluster prefers is listed for a group.
func TestListResourcesOnlyPreferredVersion(t *testing.T) {
	c, _ := withResources(autoscalingV2, autoscalingV1)
	rs, _ := c.ListResources(context.Background())
	if got := names(rs); !slices.Equal(got, []string{"horizontalpodautoscalers autoscaling/v2"}) {
		t.Errorf("resources = %q, want only autoscaling/v2", got)
	}
}

func TestListResourcesEmpty(t *testing.T) {
	c, _ := withResources()
	rs, err := c.ListResources(context.Background())
	if err != nil || len(rs) != 0 {
		t.Errorf("got %v, %v, want no resources and no error", rs, err)
	}
}

func TestListResourcesError(t *testing.T) {
	c, cs := withResources(coreV1)
	cs.PrependReactor("get", "group", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if rs, err := c.ListResources(context.Background()); err == nil || rs != nil {
		t.Errorf("got %v, %v, want an error and no resources", rs, err)
	}
}
