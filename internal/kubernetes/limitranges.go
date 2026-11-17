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

// What limit ranges have of their own; the rest is the generic calls
// (generic.go). A limit range gives the containers of a namespace that ask
// for no CPU or memory a default, and can bound what one may ask for.

// CreateDefaultLimits makes a limit range giving every container of the
// namespace that sets none the cpu and memory (e.g. "500m", "256Mi") as
// both its request and its limit. kubectl has no command for it; it is
// the YAML `kubectl apply -f` would take for
//
//	kind: LimitRange
//	spec:
//	  limits:
//	  - type: Container
//	    default:        {cpu: CPU, memory: MEMORY}
//	    defaultRequest: {cpu: CPU, memory: MEMORY}
func (c *Client) CreateDefaultLimits(ctx context.Context, namespace, name, cpu, memory string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("limit range name %q: %s", name, strings.Join(errs, "; "))
	}
	c1, err1 := resource.ParseQuantity(cpu)
	m1, err2 := resource.ParseQuantity(memory)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("cpu %q, memory %q: type amounts, e.g. 500m 256Mi", cpu, memory)
	}
	defaults := corev1.ResourceList{corev1.ResourceCPU: c1, corev1.ResourceMemory: m1}
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer, Default: defaults, DefaultRequest: defaults.DeepCopy(),
		}}},
	}
	_, err := c.clientset.CoreV1().LimitRanges(namespace).Create(ctx, lr, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
