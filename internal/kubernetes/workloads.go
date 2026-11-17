package kubernetes

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// What the kinds that run pods from a template share (deployments,
// statefulsets, daemonsets, ...): their replicas, a restart, and the
// template's containers.

// parseReplicas reads a number of replicas as typed: a whole number from 0.
func parseReplicas(replicas string) (int32, error) {
	var n int32
	s := strings.TrimSpace(replicas)
	if _, err := fmt.Sscan(s, &n); err != nil || n < 0 || fmt.Sprint(n) != s {
		return 0, fmt.Errorf("replicas %q: type a whole number, 0 or more", replicas)
	}
	return n, nil
}

// replicasText is replicas as "replicas=N"; nil is 1, the default.
func replicasText(replicas *int32) []string {
	n := int32(1)
	if replicas != nil {
		n = *replicas
	}
	return []string{fmt.Sprintf("replicas=%d", n)}
}

// restartedAtAnnotation is what `kubectl rollout restart` sets on the pod
// template: changing the template makes the controller replace every pod.
const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// stampRestart marks the template as restarted now, as `kubectl rollout
// restart` does, so its pods are replaced.
func stampRestart(t *corev1.PodTemplateSpec) {
	if t.Annotations == nil {
		t.Annotations = map[string]string{}
	}
	t.Annotations[restartedAtAnnotation] = time.Now().Format(time.RFC3339)
}

// templateContainers are the names of the template's containers: the init
// containers first, then the others. With running, only the others: an
// init container has ended by the time a pod runs.
func templateContainers(spec corev1.PodSpec, running bool) []string {
	var containers []string
	if !running {
		for _, ct := range spec.InitContainers {
			containers = append(containers, ct.Name)
		}
	}
	for _, ct := range spec.Containers {
		containers = append(containers, ct.Name)
	}
	return containers
}
