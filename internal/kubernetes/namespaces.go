package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ListNamespaces returns the names of the cluster's namespaces, sorted,
// like `kubectl get namespaces`: GET /api/v1/namespaces.
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

// CreateNamespace makes a namespace, like `kubectl create namespace NAME`.
func (c *Client) CreateNamespace(ctx context.Context, name string, dryRun bool) error {
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("namespace name %q: %s", name, strings.Join(errs, "; "))
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
