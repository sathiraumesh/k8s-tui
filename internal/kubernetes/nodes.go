package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
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

// ---------- actions: things that can be done to one node ----------

// Each matches a kubectl command.

// Cordon marks the node as unschedulable, so no new pods are placed on it,
// like `kubectl cordon NAME`. The pods already on it keep running.
//
// With dryRun the cluster checks the change but doesn't save it, like
// `kubectl cordon NAME --dry-run=server`.
func (c *Client) Cordon(ctx context.Context, name string, dryRun bool) error {
	return c.setUnschedulable(ctx, name, true, dryRun)
}

// Uncordon lets new pods be placed on the node again, like
// `kubectl uncordon NAME` (and `--dry-run=server` with dryRun).
func (c *Client) Uncordon(ctx context.Context, name string, dryRun bool) error {
	return c.setUnschedulable(ctx, name, false, dryRun)
}

// setUnschedulable sets the node's spec.unschedulable, which is what
// cordon and uncordon change.
func (c *Client) setUnschedulable(ctx context.Context, name string, unschedulable, dryRun bool) error {
	patch := fmt.Appendf(nil, `{"spec":{"unschedulable":%t}}`, unschedulable)
	opts := metav1.PatchOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	_, err := c.clientset.CoreV1().Nodes().Patch(ctx, name, types.StrategicMergePatchType, patch, opts)
	return err
}

// DescribeNode returns what `kubectl describe node NAME` prints, using
// kubectl's own describer, so the text is the same. With showEvents false
// the events at the end are left out (`--show-events=false`).
//
// kubectl's describer takes no context, so ctx can't cancel it; the
// client's own timeouts still apply.
func (c *Client) DescribeNode(_ context.Context, name string, showEvents bool) (string, error) {
	d := &describe.NodeDescriber{Interface: c.clientset}
	return d.Describe("", name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// Label sets and removes labels on the node, like
// `kubectl label node NAME env=prod old-`. Each change is one of:
//
//	key=value   set the label
//	key-        remove it
//
// A label that already has a different value is only changed with
// overwrite (`--overwrite`), and with dryRun the cluster checks the change
// but doesn't save it (`--dry-run=server`).
//
// It returns the node's labels before and after, as key=value. After a dry
// run, after is what the cluster would have saved.
func (c *Client) Label(ctx context.Context, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	set, remove, err := parseLabels(changes)
	if err != nil {
		return Change{}, err
	}
	before, after, err := c.updateNode(ctx, name, dryRun, func(n *corev1.Node) error {
		return applyLabels(&n.Labels, set, remove, overwrite)
	})
	if err != nil {
		return Change{}, err
	}
	return Change{Before: labelStrings(before), After: labelStrings(after)}, nil
}

// LabelsAndTaints returns the node's labels, as key=value sorted by key, and
// its taints, as key=value:Effect. The labels are what
// `kubectl get node NAME --show-labels` shows.
func (c *Client) LabelsAndTaints(ctx context.Context, name string) (labels, taints []string, err error) {
	n, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return labelStrings(n), taintStrings(n), nil
}

// labelStrings is the node's labels as key=value, sorted by key.
func labelStrings(n *corev1.Node) []string {
	return pairs(n.Labels)
}

// label is one key=value to set.
type label struct{ key, value string }

// parseLabels splits label changes into the labels to set and the keys to
// remove, checking each the way the API server would.
func parseLabels(changes []string) (set []label, remove []string, err error) {
	if len(changes) == 0 {
		return nil, nil, errors.New("no labels given: key=value sets one, key- removes it")
	}
	for _, ch := range changes {
		// "=" first: kubectl reads "a=b-" as a value, not a removal.
		if key, value, ok := strings.Cut(ch, "="); ok {
			if err := checkKey(key); err != nil {
				return nil, nil, err
			}
			if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
				return nil, nil, fmt.Errorf("label value %q: %s", value, strings.Join(errs, "; "))
			}
			set = append(set, label{key, value})
			continue
		}
		key, ok := strings.CutSuffix(ch, "-")
		if !ok {
			return nil, nil, fmt.Errorf("%q: want key=value to set a label, or key- to remove it", ch)
		}
		if err := checkKey(key); err != nil {
			return nil, nil, err
		}
		remove = append(remove, key)
	}
	for _, l := range set {
		if slices.Contains(remove, l.key) {
			return nil, nil, fmt.Errorf("label %q is both set and removed", l.key)
		}
	}
	return set, remove, nil
}

// Taint adds and removes taints on the node, like
// `kubectl taint node NAME key=value:NoSchedule other:NoExecute-`. A taint
// keeps pods off the node unless they tolerate it. Each change is one of:
//
//	key=value:Effect   add a taint (the value can be left out: key:Effect)
//	key:Effect-        remove the key's taint with that effect
//	key-               remove all of the key's taints
//
// Effect is NoSchedule, PreferNoSchedule or NoExecute. A taint with the
// same key and effect as one the node has is only replaced with overwrite
// (`--overwrite`); dryRun is `--dry-run=server`.
//
// It returns the node's taints before and after, as key=value:Effect.
func (c *Client) Taint(ctx context.Context, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	add, remove, err := parseTaints(changes)
	if err != nil {
		return Change{}, err
	}
	before, after, err := c.updateNode(ctx, name, dryRun, func(n *corev1.Node) error {
		for _, r := range remove {
			var kept []corev1.Taint
			for _, t := range n.Spec.Taints {
				if t.Key != r.Key || (r.Effect != "" && t.Effect != r.Effect) {
					kept = append(kept, t)
				}
			}
			if len(kept) == len(n.Spec.Taints) {
				return fmt.Errorf("taint %q not found", r.ToString())
			}
			n.Spec.Taints = kept
		}
		for _, a := range add {
			i := slices.IndexFunc(n.Spec.Taints, func(t corev1.Taint) bool { return t.MatchTaint(&a) }) // same key and effect
			switch {
			case i < 0:
				n.Spec.Taints = append(n.Spec.Taints, a)
			case !overwrite:
				return fmt.Errorf("node %s already has a %q taint with effect %s, and --overwrite is false", name, a.Key, a.Effect)
			default:
				n.Spec.Taints[i] = a
			}
		}
		return nil
	})
	if err != nil {
		return Change{}, err
	}
	return Change{Before: taintStrings(before), After: taintStrings(after)}, nil
}

// taintStrings is the node's taints as key=value:Effect, in the node's order.
func taintStrings(n *corev1.Node) []string {
	out := make([]string, 0, len(n.Spec.Taints))
	for _, t := range n.Spec.Taints {
		out = append(out, t.ToString())
	}
	return out
}

// parseTaints splits taint changes into the taints to add and the ones to
// remove. A taint to remove may have no effect: then it means every effect.
func parseTaints(changes []string) (add, remove []corev1.Taint, err error) {
	if len(changes) == 0 {
		return nil, nil, errors.New("no taints given: key=value:Effect adds one, key:Effect- removes it")
	}
	for _, ch := range changes {
		spec, removing := strings.CutSuffix(ch, "-")
		t, err := parseTaint(spec, removing)
		if err != nil {
			return nil, nil, fmt.Errorf("%q: %w", ch, err)
		}
		if removing {
			remove = append(remove, t)
		} else {
			add = append(add, t)
		}
	}
	return add, remove, nil
}

// parseTaint reads one taint, "key[=value][:Effect]". The effect may only
// be left out of a taint that is being removed.
func parseTaint(spec string, removing bool) (corev1.Taint, error) {
	keyValue, effect, hasEffect := strings.Cut(spec, ":")
	key, value, _ := strings.Cut(keyValue, "=")
	t := corev1.Taint{Key: key, Value: value, Effect: corev1.TaintEffect(effect)}

	if err := checkKey(key); err != nil {
		return t, err
	}
	if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
		return t, fmt.Errorf("taint value %q: %s", value, strings.Join(errs, "; "))
	}
	switch {
	case !hasEffect && !removing:
		return t, errors.New("want key=value:Effect, the effect is NoSchedule, PreferNoSchedule or NoExecute")
	case hasEffect && !slices.Contains(taintEffects, t.Effect):
		return t, fmt.Errorf("unknown effect %q, want NoSchedule, PreferNoSchedule or NoExecute", effect)
	}
	return t, nil
}

// taintEffects are the effects a taint can have.
var taintEffects = []corev1.TaintEffect{corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute}

// checkKey checks a label or taint key, e.g. "env" or "example.com/env".
func checkKey(key string) error {
	if errs := validation.IsQualifiedName(key); len(errs) > 0 {
		return fmt.Errorf("key %q: %s", key, strings.Join(errs, "; "))
	}
	return nil
}

// updateNode reads the node, lets change edit it, and saves it: a GET then
// a PUT, which is what kubectl label and kubectl taint do. If the node
// changed in between, the PUT fails with a conflict instead of undoing
// the other change.
//
// It returns the node as it was read, and as the cluster answered the PUT:
// as saved, or after a dry run as it would have been saved.
func (c *Client) updateNode(ctx context.Context, name string, dryRun bool, change func(*corev1.Node) error) (before, after *corev1.Node, err error) {
	nodes := c.clientset.CoreV1().Nodes()
	before, err = nodes.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	edited := before.DeepCopy() // change edits a copy, so before stays as it was
	if err := change(edited); err != nil {
		return nil, nil, err
	}
	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	after, err = nodes.Update(ctx, edited, opts)
	if err != nil {
		return nil, nil, err
	}
	return before, after, nil
}
