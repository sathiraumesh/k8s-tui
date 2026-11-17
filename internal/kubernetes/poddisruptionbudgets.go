package kubernetes

import (
	"context"
	"fmt"
	"strings"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What pod disruption budgets have of their own; the rest is the generic
// calls (generic.go). A budget keeps enough of an app's pods up while
// nodes are drained: an eviction that would break it waits.

// CreatePodDisruptionBudget makes a budget keeping minAvailable (a number,
// or a percent like "50%") of the pods matching selector (key=value,...)
// up, like `kubectl create poddisruptionbudget NAME --selector=SELECTOR
// --min-available=N -n NAMESPACE`.
func (c *Client) CreatePodDisruptionBudget(ctx context.Context, namespace, name, selector, minAvailable string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("budget name %q: %s", name, strings.Join(errs, "; "))
	}
	sel, err := metav1.ParseToLabelSelector(selector)
	if err != nil || selector == "" {
		return fmt.Errorf("selector %q: type labels of the pods, e.g. app=web", selector)
	}
	min := intstr.Parse(minAvailable)
	if v, err := intstr.GetScaledValueFromIntOrPercent(&min, 100, true); err != nil || v < 0 || minAvailable == "" {
		return fmt.Errorf("min available %q: type a number of pods or a percent, e.g. 1 or 50%%", minAvailable)
	}
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       policyv1.PodDisruptionBudgetSpec{Selector: sel, MinAvailable: &min},
	}
	_, err = c.clientset.PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
