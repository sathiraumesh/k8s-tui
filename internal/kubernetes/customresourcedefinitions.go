package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// What custom resource definitions have of their own; the rest is the
// generic calls (generic.go). A CRD adds a kind to the API, e.g. a
// Certificate of cert-manager; its objects are then listed like any kind.

// customResourceDefinitions is the type of the CRDs themselves.
var customResourceDefinitions = ResourceType{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions", Kind: "CustomResourceDefinition"}

// CustomResourceType returns the kind the CRD called name adds, in the
// version it stores (else the first it serves), to list its objects with
// the generic calls.
func (c *Client) CustomResourceType(ctx context.Context, name string) (ResourceType, error) {
	client, err := c.resource(customResourceDefinitions, "")
	if err != nil {
		return ResourceType{}, err
	}
	crd, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ResourceType{}, err
	}
	str := func(path ...string) string {
		s, _, _ := unstructured.NestedString(crd.Object, path...)
		return s
	}
	t := ResourceType{
		Group:      str("spec", "group"),
		Resource:   str("spec", "names", "plural"),
		Kind:       str("spec", "names", "kind"),
		Namespaced: str("spec", "scope") == "Namespaced",
	}
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	for _, v := range versions {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		served, _ := m["served"].(bool)
		storage, _ := m["storage"].(bool)
		if served && (storage || t.Version == "") {
			t.Version = name
		}
	}
	if t.Version == "" || t.Resource == "" {
		return ResourceType{}, fmt.Errorf("the CRD %s serves no version", name)
	}
	return t, nil
}
