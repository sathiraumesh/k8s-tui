package kubernetes

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Node is one machine in the cluster (`kubectl get nodes -o wide`).
type Node struct {
	Name          string
	Status        string   // "Ready", "NotReady" or "Unknown"
	Unschedulable bool     // true when cordoned: shown as "Ready,SchedulingDisabled"
	Roles         []string // e.g. ["control-plane"]; empty means "<none>"
	Version       string   // kubelet version, e.g. "v1.31.2"
	InternalIP    string
	ExternalIP    string // "" when the node has none
	OSImage       string // e.g. "Ubuntu 22.04.4 LTS"
	KernelVersion string
	Runtime       string // container runtime, e.g. "containerd://1.7.13"
	CreatedAt     time.Time
}

// ListNodes returns every node in the cluster, sorted by name.
// It is the same call `kubectl get nodes` makes: GET /api/v1/nodes.
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	list, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(list.Items))
	for _, n := range list.Items {
		nodes = append(nodes, toNode(n))
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes, nil
}

// toNode converts client-go's big corev1.Node into our small Node.
func toNode(n corev1.Node) Node {
	node := Node{
		Name:          n.Name,
		Status:        nodeStatus(n),
		Unschedulable: n.Spec.Unschedulable,
		Roles:         nodeRoles(n),
		Version:       n.Status.NodeInfo.KubeletVersion,
		OSImage:       n.Status.NodeInfo.OSImage,
		KernelVersion: n.Status.NodeInfo.KernelVersion,
		Runtime:       n.Status.NodeInfo.ContainerRuntimeVersion,
		CreatedAt:     n.CreationTimestamp.Time,
	}

	// A node lists its addresses by type; pick out the two kubectl shows.
	for _, addr := range n.Status.Addresses {
		switch addr.Type {
		case corev1.NodeInternalIP:
			node.InternalIP = addr.Address
		case corev1.NodeExternalIP:
			node.ExternalIP = addr.Address
		}
	}
	return node
}

// nodeStatus reads the node's "Ready" condition, like kubectl does.
func nodeStatus(n corev1.Node) string {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			switch cond.Status {
			case corev1.ConditionTrue:
				return "Ready"
			case corev1.ConditionFalse:
				return "NotReady"
			}
		}
	}
	return "Unknown"
}

// nodeRoles reads roles from labels, like kubectl does:
//
//	node-role.kubernetes.io/control-plane: ""  → "control-plane"
//	kubernetes.io/role: worker                 → "worker"
func nodeRoles(n corev1.Node) []string {
	var roles []string
	for key, value := range n.Labels {
		if role, ok := strings.CutPrefix(key, "node-role.kubernetes.io/"); ok && role != "" {
			roles = append(roles, role)
		} else if key == "kubernetes.io/role" && value != "" {
			roles = append(roles, value)
		}
	}
	sort.Strings(roles) // map order is random; sort so the output is stable
	return roles
}
