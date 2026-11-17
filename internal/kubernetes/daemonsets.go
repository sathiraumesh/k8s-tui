package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// What daemonsets have of their own; the rest is the generic calls
// (generic.go). A daemonset runs one pod on every node (or every node its
// selector picks), e.g. a log collector or a network plugin.

// daemonSetTemplate is a daemonset's pod template.
func daemonSetTemplate(d *appsv1.DaemonSet) *corev1.PodTemplateSpec { return &d.Spec.Template }

// DaemonSetContainers returns the names of the containers in the
// daemonset's pod template (see templateContainers).
func (c *Client) DaemonSetContainers(ctx context.Context, namespace, name string, running bool) ([]string, error) {
	return workloadContainers(ctx, c.clientset.AppsV1().DaemonSets(namespace), name, running, daemonSetTemplate)
}

// RestartDaemonSet replaces its pod on each node, a node at a time, like
// `kubectl rollout restart daemonset NAME -n NAMESPACE`.
func (c *Client) RestartDaemonSet(ctx context.Context, namespace, name string, dryRun bool) error {
	return restartWorkload(ctx, c.clientset.AppsV1().DaemonSets(namespace), name, dryRun, daemonSetTemplate)
}

// SetDaemonSetImage changes container images of the daemonset's pod
// template, like `kubectl set image daemonset/NAME app=IMAGE`.
func (c *Client) SetDaemonSetImage(ctx context.Context, namespace, name string, changes []string, dryRun bool) (Change, error) {
	return setWorkloadImage(ctx, c.clientset.AppsV1().DaemonSets(namespace), name, changes, dryRun, daemonSetTemplate)
}
