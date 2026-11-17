package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// What statefulsets have of their own; the rest is the generic calls
// (generic.go). A statefulset runs pods with fixed names (db-0, db-1, …)
// and each its own storage, started and stopped in order.

// statefulSetTemplate is a statefulset's pod template.
func statefulSetTemplate(s *appsv1.StatefulSet) *corev1.PodTemplateSpec { return &s.Spec.Template }

// StatefulSetContainers returns the names of the containers in the
// statefulset's pod template (see templateContainers).
func (c *Client) StatefulSetContainers(ctx context.Context, namespace, name string, running bool) ([]string, error) {
	return workloadContainers(ctx, c.clientset.AppsV1().StatefulSets(namespace), name, running, statefulSetTemplate)
}

// ScaleStatefulSet sets how many pods the statefulset runs, like `kubectl
// scale statefulset NAME --replicas=N -n NAMESPACE`. Scaling down removes
// the highest-numbered pods first; their claims (and data) stay.
func (c *Client) ScaleStatefulSet(ctx context.Context, namespace, name, replicas string, dryRun bool) (Change, error) {
	n, err := parseReplicas(replicas)
	if err != nil {
		return Change{}, err
	}
	before, after, err := update(ctx, c.clientset.AppsV1().StatefulSets(namespace), name, dryRun, func(s *appsv1.StatefulSet) error {
		s.Spec.Replicas = &n
		return nil
	})
	if err != nil {
		return Change{}, err
	}
	return Change{Before: replicasText(before.Spec.Replicas), After: replicasText(after.Spec.Replicas)}, nil
}

// RestartStatefulSet replaces its pods one at a time, the highest first,
// like `kubectl rollout restart statefulset NAME -n NAMESPACE`.
func (c *Client) RestartStatefulSet(ctx context.Context, namespace, name string, dryRun bool) error {
	return restartWorkload(ctx, c.clientset.AppsV1().StatefulSets(namespace), name, dryRun, statefulSetTemplate)
}

// SetStatefulSetImage changes container images of the statefulset's pod
// template, like `kubectl set image statefulset/NAME app=IMAGE`; its pods
// are then replaced in order.
func (c *Client) SetStatefulSetImage(ctx context.Context, namespace, name string, changes []string, dryRun bool) (Change, error) {
	return setWorkloadImage(ctx, c.clientset.AppsV1().StatefulSets(namespace), name, changes, dryRun, statefulSetTemplate)
}
