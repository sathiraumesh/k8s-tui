package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What priority classes have of their own; the rest is the generic calls
// (generic.go). A pod naming a class gets its value as its priority: when
// a node is full, pods of a higher one may push out (preempt) lower ones.

// CreatePriorityClass makes a class of value, like `kubectl create
// priorityclass NAME --value=VALUE`. Values above 1000000000 are kept for
// the system's own classes.
func (c *Client) CreatePriorityClass(ctx context.Context, name, value string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("class name %q: %s", name, strings.Join(errs, "; "))
	}
	v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil || v > 1000000000 {
		return fmt.Errorf("value %q: type a whole number up to 1000000000", value)
	}
	pc := &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: name}, Value: int32(v)}
	_, err = c.clientset.SchedulingV1().PriorityClasses().Create(ctx, pc, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
