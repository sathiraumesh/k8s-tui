package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// What replication controllers have of their own; the rest is the generic
// calls (generic.go). The old way to keep pods running, before replica
// sets and deployments.

// ScaleReplicationController sets how many pods it runs, like `kubectl
// scale replicationcontroller NAME --replicas=N -n NAMESPACE`.
func (c *Client) ScaleReplicationController(ctx context.Context, namespace, name, replicas string, dryRun bool) (Change, error) {
	return scaleWorkload(ctx, c.clientset.CoreV1().ReplicationControllers(namespace), name, replicas, dryRun,
		func(rc *corev1.ReplicationController) **int32 { return &rc.Spec.Replicas })
}
