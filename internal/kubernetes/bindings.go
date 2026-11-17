package kubernetes

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Bindings: how a pod is put on a node. The scheduler makes one for each
// pod it places; it can't be listed. kubectl has no command for it.

// BindPod puts the Pending pod on node, as the scheduler would: a Binding
// of the pod to the node. The scheduler's checks (does it fit, taints,
// affinity) are skipped; the kubelet may still refuse a pod that doesn't
// fit.
func (c *Client) BindPod(ctx context.Context, namespace, pod, node string, dryRun bool) error {
	if pod == "" || node == "" {
		return errors.New("type a pod and a node")
	}
	binding := &corev1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: pod, Namespace: namespace},
		Target:     corev1.ObjectReference{Kind: "Node", Name: node},
	}
	return c.clientset.CoreV1().Pods(namespace).Bind(ctx, binding, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
}
