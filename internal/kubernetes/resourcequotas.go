package kubernetes

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What resource quotas have of their own; the rest is the generic calls
// (generic.go). A quota caps what a namespace may use in all: pods, CPU,
// memory, storage, objects of a kind.

// parseLimits reads resource=amount entries, e.g. "pods=10",
// "requests.cpu=4", "limits.memory=8Gi".
func parseLimits(entries []string) (corev1.ResourceList, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no limits given: type resource=amount, e.g. pods=10 requests.cpu=4")
	}
	list := corev1.ResourceList{}
	for _, e := range entries {
		name, amount, ok := strings.Cut(e, "=")
		q, err := resource.ParseQuantity(amount)
		if !ok || name == "" || err != nil {
			return nil, fmt.Errorf("%q: type resource=amount, e.g. pods=10 or limits.memory=8Gi", e)
		}
		list[corev1.ResourceName(name)] = q
	}
	return list, nil
}

// CreateResourceQuota makes a quota of the limits (see parseLimits), like
// `kubectl create quota NAME --hard=pods=10,requests.cpu=4 -n NAMESPACE`.
// Once a quota limits CPU or memory, every pod must ask for them.
func (c *Client) CreateResourceQuota(ctx context.Context, namespace, name string, limits []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("quota name %q: %s", name, strings.Join(errs, "; "))
	}
	hard, err := parseLimits(limits)
	if err != nil {
		return err
	}
	q := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: corev1.ResourceQuotaSpec{Hard: hard}}
	_, err = c.clientset.CoreV1().ResourceQuotas(namespace).Create(ctx, q, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
