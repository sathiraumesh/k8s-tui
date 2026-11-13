package kubernetes

import (
	"context"
	"errors"
	"slices"
	"strings"
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

// ---------- actions ----------

// unschedulable reads back whether the node is cordoned on the fake cluster.
func unschedulable(t *testing.T, cs *fake.Clientset, name string) bool {
	t.Helper()
	n, err := cs.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return n.Spec.Unschedulable
}

func TestCordon(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, nil))
	if err := NewClientFromClientset(cs).Cordon(context.Background(), "worker1", false); err != nil {
		t.Fatal(err)
	}
	if !unschedulable(t, cs, "worker1") {
		t.Error("node is schedulable after Cordon, want unschedulable")
	}
}

func TestUncordon(t *testing.T) {
	n := fakeNode("worker1", corev1.ConditionTrue, nil)
	n.Spec.Unschedulable = true
	cs := fake.NewClientset(n)
	if err := NewClientFromClientset(cs).Uncordon(context.Background(), "worker1", false); err != nil {
		t.Fatal(err)
	}
	if unschedulable(t, cs, "worker1") {
		t.Error("node is unschedulable after Uncordon, want schedulable")
	}
}

// Cordon and uncordon send a patch of spec.unschedulable, with the dry-run
// option only when asked for.
func TestCordonPatch(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, nil))
		var got k8stesting.PatchAction
		cs.PrependReactor("patch", "nodes", func(a k8stesting.Action) (bool, runtime.Object, error) {
			got = a.(k8stesting.PatchAction)
			return false, nil, nil // let the fake cluster apply it too
		})
		if err := NewClientFromClientset(cs).Cordon(context.Background(), "worker1", dryRun); err != nil {
			t.Fatal(err)
		}
		if got == nil || got.GetName() != "worker1" || string(got.GetPatch()) != `{"spec":{"unschedulable":true}}` {
			t.Errorf("dryRun=%v: patch = %+v", dryRun, got)
		}
		if patch, ok := got.(k8stesting.PatchActionImpl); ok {
			isDryRun := len(patch.PatchOptions.DryRun) == 1 && patch.PatchOptions.DryRun[0] == metav1.DryRunAll
			if isDryRun != dryRun {
				t.Errorf("dryRun=%v: patch options = %+v", dryRun, patch.PatchOptions)
			}
		}
	}
}

func TestCordonMissingNode(t *testing.T) {
	err := NewClientFromClientset(fake.NewClientset()).Cordon(context.Background(), "nope", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want not found", err)
	}
}

func TestCordonError(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, nil))
	cs.PrependReactor("patch", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if err := NewClientFromClientset(cs).Cordon(context.Background(), "worker1", false); err == nil {
		t.Error("err = nil, want forbidden")
	}
}

func TestDescribeNode(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, map[string]string{"node-role.kubernetes.io/worker": ""}))
	text, err := NewClientFromClientset(cs).DescribeNode(context.Background(), "worker1", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Name:", "worker1", "Roles:", "worker", "v1.37.2"} {
		if !strings.Contains(text, want) {
			t.Errorf("describe output is missing %q:\n%s", want, text)
		}
	}
}

func TestDescribeMissingNode(t *testing.T) {
	if _, err := NewClientFromClientset(fake.NewClientset()).DescribeNode(context.Background(), "nope", true); err == nil {
		t.Error("err = nil, want an error for a node that doesn't exist")
	}
}

// ---------- labels and taints ----------

// getNode reads a node back from the fake cluster.
func getNode(t *testing.T, cs *fake.Clientset, name string) *corev1.Node {
	t.Helper()
	n, err := cs.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLabelSetsAndRemoves(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, map[string]string{"old": "x", "keep": "y"}))
	if _, err := NewClientFromClientset(cs).Label(context.Background(), "worker1", []string{"env=prod", "example.com/team=a", "old-"}, false, false); err != nil {
		t.Fatal(err)
	}
	got := getNode(t, cs, "worker1").Labels
	want := map[string]string{"env": "prod", "example.com/team": "a", "keep": "y"}
	if len(got) != len(want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("labels = %v, want %v", got, want)
		}
	}
}

// A node with no labels at all can still be labelled.
func TestLabelNodeWithoutLabels(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, nil))
	if _, err := NewClientFromClientset(cs).Label(context.Background(), "worker1", []string{"env=prod"}, false, false); err != nil {
		t.Fatal(err)
	}
	if got := getNode(t, cs, "worker1").Labels["env"]; got != "prod" {
		t.Errorf("env = %q, want prod", got)
	}
}

// Changing a label's value needs overwrite; setting the same value doesn't.
func TestLabelOverwrite(t *testing.T) {
	cs := fake.NewClientset(fakeNode("worker1", corev1.ConditionTrue, map[string]string{"env": "dev"}))
	c := NewClientFromClientset(cs)
	_, err := c.Label(context.Background(), "worker1", []string{"env=prod"}, false, false)
	if err == nil || !strings.Contains(err.Error(), "--overwrite is false") {
		t.Errorf("err = %v, want the --overwrite error", err)
	}
	if _, err := c.Label(context.Background(), "worker1", []string{"env=dev"}, false, false); err != nil {
		t.Errorf("same value: err = %v, want nil", err)
	}
	if _, err := c.Label(context.Background(), "worker1", []string{"env=prod"}, true, false); err != nil {
		t.Fatal(err)
	}
	if got := getNode(t, cs, "worker1").Labels["env"]; got != "prod" {
		t.Errorf("env = %q after overwrite, want prod", got)
	}
}

func TestParseLabelsErrors(t *testing.T) {
	for _, changes := range [][]string{
		nil,               // nothing given
		{"env"},           // neither =value nor -
		{"bad key=x"},     // space in the key
		{"=x"},            // no key
		{"env=a b"},       // space in the value
		{"env=x", "env-"}, // set and removed
	} {
		if _, _, err := parseLabels(changes); err == nil {
			t.Errorf("parseLabels(%q): err = nil, want an error", changes)
		}
	}
}

// taintsOf is the node's taints written like kubectl takes them.
func taintsOf(t *testing.T, cs *fake.Clientset, name string) []string {
	t.Helper()
	var out []string
	for _, taint := range getNode(t, cs, name).Spec.Taints {
		out = append(out, taint.ToString())
	}
	return out
}

func taintedNode(taints ...corev1.Taint) *corev1.Node {
	n := fakeNode("worker1", corev1.ConditionTrue, nil)
	n.Spec.Taints = taints
	return n
}

func TestTaintAdds(t *testing.T) {
	cs := fake.NewClientset(taintedNode())
	if _, err := NewClientFromClientset(cs).Taint(context.Background(), "worker1", []string{"gpu=yes:NoSchedule", "spot:PreferNoSchedule"}, false, false); err != nil {
		t.Fatal(err)
	}
	got := taintsOf(t, cs, "worker1")
	if !slices.Equal(got, []string{"gpu=yes:NoSchedule", "spot:PreferNoSchedule"}) {
		t.Errorf("taints = %v", got)
	}
}

func TestTaintRemoves(t *testing.T) {
	start := []corev1.Taint{
		{Key: "gpu", Value: "yes", Effect: corev1.TaintEffectNoSchedule},
		{Key: "gpu", Effect: corev1.TaintEffectNoExecute},
		{Key: "spot", Effect: corev1.TaintEffectPreferNoSchedule},
	}
	for _, tc := range []struct {
		change string
		want   []string
	}{
		{"gpu:NoSchedule-", []string{"gpu:NoExecute", "spot:PreferNoSchedule"}}, // that effect only
		{"gpu-", []string{"spot:PreferNoSchedule"}},                             // every effect
	} {
		cs := fake.NewClientset(taintedNode(start...))
		if _, err := NewClientFromClientset(cs).Taint(context.Background(), "worker1", []string{tc.change}, false, false); err != nil {
			t.Fatal(err)
		}
		if got := taintsOf(t, cs, "worker1"); !slices.Equal(got, tc.want) {
			t.Errorf("%s: taints = %v, want %v", tc.change, got, tc.want)
		}
	}
}

func TestTaintRemoveMissing(t *testing.T) {
	cs := fake.NewClientset(taintedNode())
	_, err := NewClientFromClientset(cs).Taint(context.Background(), "worker1", []string{"gpu-"}, false, false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want not found", err)
	}
}

// A taint with the same key and effect is only replaced with overwrite.
func TestTaintOverwrite(t *testing.T) {
	cs := fake.NewClientset(taintedNode(corev1.Taint{Key: "gpu", Value: "no", Effect: corev1.TaintEffectNoSchedule}))
	c := NewClientFromClientset(cs)
	if _, err := c.Taint(context.Background(), "worker1", []string{"gpu=yes:NoSchedule"}, false, false); err == nil {
		t.Error("err = nil, want the --overwrite error")
	}
	if _, err := c.Taint(context.Background(), "worker1", []string{"gpu=yes:NoSchedule"}, true, false); err != nil {
		t.Fatal(err)
	}
	if got := taintsOf(t, cs, "worker1"); !slices.Equal(got, []string{"gpu=yes:NoSchedule"}) {
		t.Errorf("taints = %v, want the value replaced", got)
	}
}

func TestParseTaintsErrors(t *testing.T) {
	for _, changes := range [][]string{
		nil,                   // nothing given
		{"gpu=yes"},           // adding needs an effect
		{"gpu=yes:Sometimes"}, // unknown effect
		{"bad key:NoSchedule"},
		{"gpu=a b:NoSchedule"},
	} {
		if _, _, err := parseTaints(changes); err == nil {
			t.Errorf("parseTaints(%q): err = nil, want an error", changes)
		}
	}
}

// Label and taint read the node and save it back with an update, with the
// dry-run option only when asked for. A bad change sends nothing.
func TestLabelAndTaintUpdate(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		cs := fake.NewClientset(taintedNode())
		var updates []k8stesting.UpdateActionImpl
		cs.PrependReactor("update", "nodes", func(a k8stesting.Action) (bool, runtime.Object, error) {
			updates = append(updates, a.(k8stesting.UpdateActionImpl))
			return false, nil, nil
		})
		c := NewClientFromClientset(cs)
		if _, err := c.Label(context.Background(), "worker1", []string{"env=prod"}, false, dryRun); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Taint(context.Background(), "worker1", []string{"gpu:NoSchedule"}, false, dryRun); err != nil {
			t.Fatal(err)
		}
		_, _ = c.Label(context.Background(), "worker1", []string{"bad key=x"}, false, dryRun) // a bad change: nothing must be sent
		if len(updates) != 2 {
			t.Fatalf("dryRun=%v: %d updates, want 2", dryRun, len(updates))
		}
		for _, u := range updates {
			isDryRun := len(u.UpdateOptions.DryRun) == 1 && u.UpdateOptions.DryRun[0] == metav1.DryRunAll
			if isDryRun != dryRun {
				t.Errorf("dryRun=%v: update options = %+v", dryRun, u.UpdateOptions)
			}
		}
	}
}

func TestLabelMissingNode(t *testing.T) {
	_, err := NewClientFromClientset(fake.NewClientset()).Label(context.Background(), "nope", []string{"env=prod"}, false, false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want not found", err)
	}
}

// Label and Taint return the labels or taints before and after the change.
func TestLabelAndTaintReturnChange(t *testing.T) {
	cs := fake.NewClientset(taintedNode(corev1.Taint{Key: "spot", Effect: corev1.TaintEffectPreferNoSchedule}))
	c := NewClientFromClientset(cs)

	got, err := c.Label(context.Background(), "worker1", []string{"env=prod"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Before) != 0 || !slices.Equal(got.After, []string{"env=prod"}) {
		t.Errorf("label change = %+v", got)
	}

	got, err = c.Taint(context.Background(), "worker1", []string{"gpu=yes:NoSchedule", "spot-"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Before, []string{"spot:PreferNoSchedule"}) || !slices.Equal(got.After, []string{"gpu=yes:NoSchedule"}) {
		t.Errorf("taint change = %+v", got)
	}
}

func TestLabelsAndTaints(t *testing.T) {
	cs := fake.NewClientset(taintedNode(corev1.Taint{Key: "gpu", Value: "yes", Effect: corev1.TaintEffectNoSchedule}))
	c := NewClientFromClientset(cs)
	if _, err := c.Label(context.Background(), "worker1", []string{"zone=b", "env=prod"}, false, false); err != nil {
		t.Fatal(err)
	}
	labels, taints, err := c.LabelsAndTaints(context.Background(), "worker1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(labels, []string{"env=prod", "zone=b"}) || !slices.Equal(taints, []string{"gpu=yes:NoSchedule"}) {
		t.Errorf("labels = %v, taints = %v", labels, taints)
	}
	if _, _, err := c.LabelsAndTaints(context.Background(), "nope"); err == nil {
		t.Error("err = nil for a node that doesn't exist")
	}
}
