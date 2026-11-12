package kubernetes

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// fakeNode builds a client-go node for tests.
// ready is the status of its "Ready" condition ("" means no condition at all).
func fakeNode(name string, ready corev1.ConditionStatus, labels map[string]string) *corev1.Node {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.37.2",
				OSImage:                 "Ubuntu 24.04 LTS",
				KernelVersion:           "6.8.0",
				ContainerRuntimeVersion: "containerd://2.0.0",
			},
		},
	}
	if ready != "" {
		n.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
			{Type: corev1.NodeReady, Status: ready},
		}
	}
	return n
}

func TestNodeStatus(t *testing.T) {
	tests := []struct {
		ready corev1.ConditionStatus
		want  string
	}{
		{corev1.ConditionTrue, "Ready"},
		{corev1.ConditionFalse, "NotReady"},
		{corev1.ConditionUnknown, "Unknown"},
		{"", "Unknown"}, // no Ready condition reported yet
	}
	for _, tt := range tests {
		t.Run(tt.want+"/"+string(tt.ready), func(t *testing.T) {
			if got := nodeStatus(*fakeNode("n", tt.ready, nil)); got != tt.want {
				t.Errorf("nodeStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeRoles(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{"no labels", nil, nil},
		{"control plane", map[string]string{"node-role.kubernetes.io/control-plane": ""}, []string{"control-plane"}},
		{"old role label", map[string]string{"kubernetes.io/role": "worker"}, []string{"worker"}},
		{"several roles are sorted", map[string]string{
			"node-role.kubernetes.io/worker":        "",
			"node-role.kubernetes.io/control-plane": "",
		}, []string{"control-plane", "worker"}},
		{"empty role name ignored", map[string]string{"node-role.kubernetes.io/": ""}, nil},
		{"empty old role ignored", map[string]string{"kubernetes.io/role": ""}, nil},
		{"unrelated labels ignored", map[string]string{"kubernetes.io/os": "linux"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeRoles(*fakeNode("n", corev1.ConditionTrue, tt.labels))
			if !slices.Equal(got, tt.want) {
				t.Errorf("nodeRoles = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToNode(t *testing.T) {
	n := fakeNode("worker1", corev1.ConditionTrue, map[string]string{"node-role.kubernetes.io/worker": ""})
	n.Spec.Unschedulable = true
	n.Status.Addresses = []corev1.NodeAddress{
		{Type: corev1.NodeHostName, Address: "worker1"},
		{Type: corev1.NodeInternalIP, Address: "10.0.0.11"},
		{Type: corev1.NodeExternalIP, Address: "203.0.113.11"},
	}

	got := toNode(*n)

	want := Node{
		Name:          "worker1",
		Status:        "Ready",
		Unschedulable: true,
		Roles:         []string{"worker"},
		Version:       "v1.37.2",
		InternalIP:    "10.0.0.11",
		ExternalIP:    "203.0.113.11",
		OSImage:       "Ubuntu 24.04 LTS",
		KernelVersion: "6.8.0",
		Runtime:       "containerd://2.0.0",
		CreatedAt:     time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if got.Name != want.Name || got.Status != want.Status || got.Unschedulable != want.Unschedulable ||
		!slices.Equal(got.Roles, want.Roles) || got.Version != want.Version ||
		got.InternalIP != want.InternalIP || got.ExternalIP != want.ExternalIP ||
		got.OSImage != want.OSImage || got.KernelVersion != want.KernelVersion ||
		got.Runtime != want.Runtime || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("toNode =\n  %+v\nwant\n  %+v", got, want)
	}
}

func TestToNodeWithoutExternalIP(t *testing.T) {
	n := fakeNode("n", corev1.ConditionTrue, nil)
	n.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}}
	if got := toNode(*n); got.ExternalIP != "" {
		t.Errorf("ExternalIP = %q, want empty", got.ExternalIP)
	}
}

func TestListNodesSortedByName(t *testing.T) {
	cs := fake.NewClientset(
		fakeNode("worker2", corev1.ConditionTrue, nil),
		fakeNode("controlplane", corev1.ConditionTrue, nil),
		fakeNode("worker1", corev1.ConditionFalse, nil),
	)
	c := NewClientFromClientset(cs)

	nodes, err := c.ListNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if want := []string{"controlplane", "worker1", "worker2"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if nodes[1].Status != "NotReady" {
		t.Errorf("worker1 status = %q, want NotReady", nodes[1].Status)
	}
}

func TestListNodesEmptyCluster(t *testing.T) {
	nodes, err := NewClientFromClientset(fake.NewClientset()).ListNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Errorf("got %d nodes, want 0", len(nodes))
	}
}

func TestListNodesError(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	nodes, err := NewClientFromClientset(cs).ListNodes(context.Background())
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("err = %v, want forbidden", err)
	}
	if nodes != nil {
		t.Errorf("nodes = %v, want nil on error", nodes)
	}
}
