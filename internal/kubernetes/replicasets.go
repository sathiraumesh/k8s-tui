package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
)

// What replica sets have of their own; the rest is the generic calls
// (generic.go). A replica set keeps a number of identical pods running; a
// deployment makes one per version of its template and scales them.

// ScaleReplicaSet sets how many pods the replica set runs, like `kubectl
// scale replicaset NAME --replicas=N -n NAMESPACE`. A deployment that owns
// it sets it back: scale the deployment instead.
func (c *Client) ScaleReplicaSet(ctx context.Context, namespace, name, replicas string, dryRun bool) (Change, error) {
	return scaleWorkload(ctx, c.clientset.AppsV1().ReplicaSets(namespace), name, replicas, dryRun,
		func(r *appsv1.ReplicaSet) **int32 { return &r.Spec.Replicas })
}
