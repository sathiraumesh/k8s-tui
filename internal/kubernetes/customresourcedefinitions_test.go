package kubernetes

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestCustomResourceType(t *testing.T) {
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": "widgets.example.com"},
		"spec": map[string]any{
			"group": "example.com", "scope": "Namespaced",
			"names": map[string]any{"plural": "widgets", "kind": "Widget"},
			"versions": []any{
				map[string]any{"name": "v1alpha1", "served": true, "storage": false},
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}
	c := NewClientFromClientsets(nil, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), crd))
	got, err := c.CustomResourceType(context.Background(), "widgets.example.com")
	want := ResourceType{Group: "example.com", Version: "v1", Resource: "widgets", Kind: "Widget", Namespaced: true}
	if err != nil || got != want {
		t.Errorf("type = %+v, err = %v, want %+v", got, err, want)
	}
}
