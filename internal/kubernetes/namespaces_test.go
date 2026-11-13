package kubernetes

import (
	"context"
	"errors"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func TestListNamespacesSorted(t *testing.T) {
	cs := fake.NewClientset(namespace("kube-system"), namespace("apps"), namespace("default"))
	got, err := NewClientFromClientset(cs).ListNamespaces(context.Background())
	if err != nil || !slices.Equal(got, []string{"apps", "default", "kube-system"}) {
		t.Errorf("namespaces = %v, err = %v", got, err)
	}
}

func TestListNamespacesError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := NewClientFromClientset(cs).ListNamespaces(context.Background()); err == nil {
		t.Error("err = nil, want forbidden")
	}
}
