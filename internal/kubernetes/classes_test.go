package kubernetes

import (
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const defaultClass = "storageclass.kubernetes.io/is-default-class"

func storageClass(name, isDefault string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "storage.k8s.io/v1", "kind": "StorageClass",
		"metadata": map[string]any{"name": name}, "provisioner": "x"}}
	if isDefault != "" {
		obj.SetAnnotations(map[string]string{defaultClass: isDefault})
	}
	return obj
}

func TestMakeDefaultClass(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "StorageClassList"},
		storageClass("standard", "true"), storageClass("fast", ""))
	c := NewClientFromClientsets(nil, dyn)
	was, err := c.MakeDefaultClass(context.Background(), storageClasses, "fast", defaultClass, false)
	if err != nil || !slices.Equal(was, []string{"standard"}) {
		t.Fatalf("was %q, err %v", was, err)
	}
	for name, want := range map[string]string{"fast": "true", "standard": "false"} {
		obj, _ := dyn.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
		if got := obj.GetAnnotations()[defaultClass]; got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
