package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
)

// Pod is one pod, with what `kubectl get pods -o wide` shows of it.
type Pod struct {
	Namespace string
	Name      string

	// Ready is the containers that are ready out of all of them, e.g. "1/2".
	Ready string

	// Status is what kubectl shows in its STATUS column: the phase
	// ("Running", "Pending", "Succeeded" shown as "Completed", …), or why it
	// isn't running ("CrashLoopBackOff", "Init:0/1", "Terminating", …).
	Status string

	// Restarts is how many times its containers were restarted, and
	// LastRestart when the latest restart was (zero if never).
	Restarts    int32
	LastRestart time.Time

	IP            string // "" before it has one
	Node          string // "" before it is scheduled
	NominatedNode string // the node it may go to after preemption, usually ""

	// ReadinessGates is the readiness gates that are met out of all of them,
	// e.g. "1/1", or "" if the pod has none.
	ReadinessGates string

	CreatedAt time.Time
}

// ListPods returns the pods in namespace, or in every namespace if it is
// "", sorted by namespace and name. It is the call
// `kubectl get pods --all-namespaces` makes: GET /api/v1/pods.
func (c *Client) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, 0, len(list.Items))
	for _, p := range list.Items {
		pods = append(pods, toPod(p))
	}
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})
	return pods, nil
}

// toPod converts client-go's corev1.Pod into our small Pod.
func toPod(p corev1.Pod) Pod {
	status, ready, restarts, lastRestart := podStatus(p)
	pod := Pod{
		Namespace:     p.Namespace,
		Name:          p.Name,
		Ready:         fmt.Sprintf("%d/%d", ready, len(p.Spec.Containers)),
		Status:        status,
		Restarts:      restarts,
		LastRestart:   lastRestart,
		IP:            p.Status.PodIP,
		Node:          p.Spec.NodeName,
		NominatedNode: p.Status.NominatedNodeName,
		CreatedAt:     p.CreationTimestamp.Time,
	}
	if gates := len(p.Spec.ReadinessGates); gates > 0 {
		pod.ReadinessGates = fmt.Sprintf("%d/%d", readinessGatesMet(p), gates)
	}
	return pod
}

// podStatus works out the STATUS column the way kubectl does (printPod in
// kubectl's printers), and with it the number of ready containers, the
// restarts and when the last one was.
//
// In order, the first that applies wins:
//
//	being deleted                         Terminating
//	an init container hasn't finished     Init:0/2, Init:CrashLoopBackOff, Init:Error, …
//	a container is waiting or has ended   CrashLoopBackOff, ImagePullBackOff, Error, Completed, …
//	otherwise                             the pod's reason, or its phase: Running, Pending, …
func podStatus(p corev1.Pod) (status string, ready int, restarts int32, lastRestart time.Time) {
	status = string(p.Status.Phase)
	if p.Status.Reason != "" {
		status = p.Status.Reason // e.g. "Evicted"
	}

	// noteRestarts adds a container's restarts to *count and keeps the
	// latest one in *last.
	noteRestarts := func(cs corev1.ContainerStatus, count *int32, last *time.Time) {
		*count += cs.RestartCount
		if t := cs.LastTerminationState.Terminated; t != nil && t.FinishedAt.After(*last) {
			*last = t.FinishedAt.Time
		}
	}

	// Init containers run one after the other before the others start.
	// Their restarts count while the pod is starting; once it has started,
	// only the sidecars' do, since they keep running.
	initializing := false
	var sidecarRestarts int32
	var sidecarLast time.Time
	for i, cs := range p.Status.InitContainerStatuses {
		noteRestarts(cs, &restarts, &lastRestart)
		if isSidecar(p, cs.Name) {
			noteRestarts(cs, &sidecarRestarts, &sidecarLast)
		}
		switch {
		case cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0:
			continue // this one has finished: on to the next
		case cs.State.Terminated != nil:
			status = "Init:" + terminatedReason(cs.State.Terminated)
		case cs.State.Waiting != nil && cs.State.Waiting.Reason != "" && cs.State.Waiting.Reason != "PodInitializing":
			status = "Init:" + cs.State.Waiting.Reason
		case isSidecar(p, cs.Name) && cs.Started != nil && *cs.Started:
			continue // a sidecar keeps running: it's started, so on to the next
		default:
			status = fmt.Sprintf("Init:%d/%d", i, len(p.Spec.InitContainers))
		}
		initializing = true
		break
	}

	if !initializing || conditionTrue(p, corev1.PodInitialized) {
		restarts, lastRestart = sidecarRestarts, sidecarLast
		hasRunning := false
		for i := len(p.Status.ContainerStatuses) - 1; i >= 0; i-- { // like kubectl: the first container's reason wins
			cs := p.Status.ContainerStatuses[i]
			noteRestarts(cs, &restarts, &lastRestart)
			switch {
			case cs.State.Waiting != nil && cs.State.Waiting.Reason != "":
				status = cs.State.Waiting.Reason
			case cs.State.Terminated != nil:
				status = terminatedReason(cs.State.Terminated)
			case cs.Ready && cs.State.Running != nil:
				hasRunning = true
				ready++
			}
		}
		// Some containers have completed but others still run.
		if status == "Completed" && hasRunning {
			status = "NotReady"
			if podReady(p) {
				status = "Running"
			}
		}
	}

	if p.DeletionTimestamp != nil {
		status = "Terminating"
		if p.Status.Reason == "NodeLost" {
			status = "Unknown"
		}
	}
	return status, ready, restarts, lastRestart
}

// terminatedReason is why a container ended: its reason ("Completed",
// "Error", "OOMKilled"), or else its signal or exit code.
func terminatedReason(t *corev1.ContainerStateTerminated) string {
	switch {
	case t.Reason != "":
		return t.Reason
	case t.Signal != 0:
		return fmt.Sprintf("Signal:%d", t.Signal)
	default:
		return fmt.Sprintf("ExitCode:%d", t.ExitCode)
	}
}

// isSidecar reports whether the init container called name is a sidecar:
// one that starts first and keeps running (restartPolicy: Always).
func isSidecar(p corev1.Pod, name string) bool {
	for _, c := range p.Spec.InitContainers {
		if c.Name == name {
			return c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
		}
	}
	return false
}

// podReady reports whether the pod's Ready condition is true.
func podReady(p corev1.Pod) bool {
	return conditionTrue(p, corev1.PodReady)
}

// readinessGatesMet counts the pod's readiness gates whose condition is true.
func readinessGatesMet(p corev1.Pod) int {
	met := 0
	for _, g := range p.Spec.ReadinessGates {
		if conditionTrue(p, g.ConditionType) {
			met++
		}
	}
	return met
}

// conditionTrue reports whether the pod has the condition t, set to true.
func conditionTrue(p corev1.Pod, t corev1.PodConditionType) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == t {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// RunPod starts a new pod called name in namespace, with one container from
// image, like `kubectl run NAME --image=IMAGE -n NAMESPACE`. As with
// kubectl, the pod gets the label run=NAME and its container the pod's
// name, and nothing manages it: once deleted, it isn't started again.
//
// With dryRun the cluster checks the pod but doesn't create it
// (`--dry-run=server`).
func (c *Client) RunPod(ctx context.Context, namespace, name, image string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("pod name %q: %s", name, strings.Join(errs, "; "))
	}
	if strings.TrimSpace(image) == "" || strings.ContainsAny(image, " \t") {
		return errors.New("an image is needed, with no spaces, e.g. nginx:latest")
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"run": name},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: name, Image: image}},
		},
	}
	opts := metav1.CreateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	_, err := c.clientset.CoreV1().Pods(namespace).Create(ctx, pod, opts)
	return err
}

// DescribePod returns what `kubectl describe pod NAME -n NAMESPACE` prints,
// using kubectl's own describer, so the text is the same: containers,
// their state and restarts, volumes, conditions and the events. With
// showEvents false the events are left out (`--show-events=false`).
//
// As for DescribeNode, ctx can't cancel kubectl's describer.
func (c *Client) DescribePod(_ context.Context, namespace, name string, showEvents bool) (string, error) {
	d := &describe.PodDescriber{Interface: c.clientset}
	return d.Describe(namespace, name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// ---------- actions: things that can be done to one pod ----------

// PodLogs returns the last tail lines each container of the pod wrote,
// like `kubectl logs NAME -n NAMESPACE --all-containers --tail=TAIL`. With
// previous, the logs are those of the container's run before its last
// restart (`--previous`); with timestamps, every line starts with its time
// (`--timestamps`).
//
// A pod with more than one container has a heading per container:
//
//	==> app <==
//	…
//	==> sidecar <==
//	…
//
// A container whose logs can't be read (with previous, one that never
// restarted) says why under its heading; the others are still shown.
func (c *Client) PodLogs(ctx context.Context, namespace, name string, previous, timestamps bool, tail int64) (string, error) {
	pods := c.clientset.CoreV1().Pods(namespace)
	pod, err := pods.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var containers []string
	for _, ct := range pod.Spec.InitContainers {
		containers = append(containers, ct.Name)
	}
	for _, ct := range pod.Spec.Containers {
		containers = append(containers, ct.Name)
	}

	var b strings.Builder
	for _, container := range containers {
		opts := &corev1.PodLogOptions{Container: container, Previous: previous, Timestamps: timestamps, TailLines: &tail}
		logs, err := pods.GetLogs(name, opts).DoRaw(ctx)
		if len(containers) == 1 {
			return string(logs), err
		}
		fmt.Fprintf(&b, "==> %s <==\n", container)
		if err != nil {
			fmt.Fprintf(&b, "error: %v\n", err)
		} else if len(logs) == 0 {
			b.WriteString("(no logs)\n")
		} else {
			b.Write(logs)
			if logs[len(logs)-1] != '\n' {
				b.WriteByte('\n')
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// LabelsAndAnnotations returns the pod's labels and annotations, as
// key=value sorted by key.
func (c *Client) LabelsAndAnnotations(ctx context.Context, namespace, name string) (labels, annotations []string, err error) {
	p, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return pairs(p.Labels), pairs(p.Annotations), nil
}

// LabelPod sets and removes labels on the pod, like
// `kubectl label pod NAME -n NAMESPACE env=prod old-`, with the same rules
// as Label for a node. It returns the pod's labels before and after.
func (c *Client) LabelPod(ctx context.Context, namespace, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	set, remove, err := parseLabels(changes)
	if err != nil {
		return Change{}, err
	}
	pods := c.clientset.CoreV1().Pods(namespace)
	before, err := pods.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Change{}, err
	}
	edited := before.DeepCopy()
	if err := applyLabels(&edited.Labels, set, remove, overwrite); err != nil {
		return Change{}, err
	}
	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	after, err := pods.Update(ctx, edited, opts)
	if err != nil {
		return Change{}, err
	}
	return Change{Before: pairs(before.Labels), After: pairs(after.Labels)}, nil
}

// DeletePod deletes the pod, like `kubectl delete pod NAME -n NAMESPACE`.
// Its containers are asked to stop and given their grace period (30s
// unless the pod says otherwise); with now they get 1 second (`--now`).
// If something manages the pod, such as a deployment, it starts a new one.
//
// With dryRun the cluster checks it but deletes nothing.
func (c *Client) DeletePod(ctx context.Context, namespace, name string, now, dryRun bool) error {
	opts := metav1.DeleteOptions{}
	if now {
		grace := int64(1)
		opts.GracePeriodSeconds = &grace
	}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	return c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, opts)
}
