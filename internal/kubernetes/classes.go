package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The "classes" that one of is the default, marked by an annotation:
// StorageClass (storageclass.kubernetes.io/is-default-class) and
// IngressClass (ingressclass.kubernetes.io/is-default-class). Making one the
// default is the same for both, with the dynamic client (generic.go).

// MakeDefaultClass makes the object called name the default of its kind:
// annotation "true" on it, and "false" on any other that had it, as two
// defaults at once leave the choice to chance. It returns the names that
// stopped being the default.
func (c *Client) MakeDefaultClass(ctx context.Context, r ResourceType, name, annotation string, dryRun bool) (was []string, err error) {
	client, err := c.resource(r, "")
	if err != nil {
		return nil, err
	}
	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	setTo := func(value string) func(*unstructured.Unstructured) error {
		return func(obj *unstructured.Unstructured) error {
			a := obj.GetAnnotations()
			if a == nil {
				a = map[string]string{}
			}
			a[annotation] = value
			obj.SetAnnotations(a)
			return nil
		}
	}
	if _, _, err := c.updateResource(ctx, r, "", name, dryRun, setTo("true")); err != nil {
		return nil, err
	}
	for _, obj := range list.Items {
		if obj.GetName() != name && obj.GetAnnotations()[annotation] == "true" {
			if _, _, err := c.updateResource(ctx, r, "", obj.GetName(), dryRun, setTo("false")); err != nil {
				return was, err
			}
			was = append(was, obj.GetName())
		}
	}
	return was, nil
}
