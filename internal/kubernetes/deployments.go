package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
)

// Deployment is one deployment, with what `kubectl get deployments -o wide`
// shows of it.
type Deployment struct {
	Namespace string
	Name      string

	// Ready is the pods that are ready out of those wanted, e.g. "2/3".
	// UpToDate is how many pods run the latest template, Available how many
	// have been ready long enough to take traffic.
	Ready     string
	UpToDate  int32
	Available int32

	// Containers and Images are the template's containers and their images,
	// in order; Selector is how it finds its pods, e.g. "app=web".
	Containers []string
	Images     []string
	Selector   string

	CreatedAt time.Time
}

// ListDeployments returns the deployments in namespace, or in every
// namespace if it is "", sorted by namespace and name. It is the call
// `kubectl get deployments --all-namespaces` makes.
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]Deployment, error) {
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	deployments := make([]Deployment, 0, len(list.Items))
	for _, d := range list.Items {
		deployments = append(deployments, toDeployment(d))
	}
	sort.Slice(deployments, func(i, j int) bool {
		if deployments[i].Namespace != deployments[j].Namespace {
			return deployments[i].Namespace < deployments[j].Namespace
		}
		return deployments[i].Name < deployments[j].Name
	})
	return deployments, nil
}

// toDeployment converts client-go's appsv1.Deployment into our Deployment.
func toDeployment(d appsv1.Deployment) Deployment {
	wanted := int32(1) // the default when replicas isn't set
	if d.Spec.Replicas != nil {
		wanted = *d.Spec.Replicas
	}
	dep := Deployment{
		Namespace: d.Namespace,
		Name:      d.Name,
		Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, wanted),
		UpToDate:  d.Status.UpdatedReplicas,
		Available: d.Status.AvailableReplicas,
		CreatedAt: d.CreationTimestamp.Time,
	}
	for _, ct := range d.Spec.Template.Spec.Containers {
		dep.Containers = append(dep.Containers, ct.Name)
		dep.Images = append(dep.Images, ct.Image)
	}
	if d.Spec.Selector != nil {
		dep.Selector = metav1.FormatLabelSelector(d.Spec.Selector)
	}
	return dep
}

// DescribeDeployment returns what `kubectl describe deployment NAME -n
// NAMESPACE` prints, using kubectl's own describer: replicas, strategy, the
// pod template, conditions, its replica sets and the events.
//
// kubectl's deployment describer can only be made from the connection
// settings, so a pretend cluster in tests can't describe one.
func (c *Client) DescribeDeployment(_ context.Context, namespace, name string, showEvents bool) (string, error) {
	if c.config == nil {
		return "", errors.New("describing a deployment needs a real cluster connection")
	}
	d, ok := describe.DescriberFor(schema.GroupKind{Group: appsv1.GroupName, Kind: "Deployment"}, c.config)
	if !ok {
		return "", errors.New("kubectl has no describer for deployments")
	}
	return d.Describe(namespace, name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// DeploymentContainers returns the names of the containers in the
// deployment's pod template: the init containers first, then the others.
// With running, only the others: an init container has ended by the time a
// pod runs.
func (c *Client) DeploymentContainers(ctx context.Context, namespace, name string, running bool) ([]string, error) {
	return workloadContainers(ctx, c.clientset.AppsV1().Deployments(namespace), name, running, deploymentTemplate)
}

// deploymentTemplate is a deployment's pod template.
func deploymentTemplate(d *appsv1.Deployment) *corev1.PodTemplateSpec { return &d.Spec.Template }

// DeploymentLabelsAndAnnotations returns the deployment's labels and
// annotations, as key=value sorted by key.
func (c *Client) DeploymentLabelsAndAnnotations(ctx context.Context, namespace, name string) (labels, annotations []string, err error) {
	return labelsAndAnnotations[appsv1.Deployment](ctx, c.clientset.AppsV1().Deployments(namespace), name)
}

// LabelDeployment sets and removes labels on the deployment, like
// `kubectl label deployment NAME -n NAMESPACE env=prod old-`, with the same
// rules as for a node or a pod. Only the deployment's own labels change,
// not its pods'.
func (c *Client) LabelDeployment(ctx context.Context, namespace, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	return labelObject[appsv1.Deployment](ctx, c.clientset.AppsV1().Deployments(namespace), name, changes, overwrite, dryRun)
}

// ScaleDeployment sets how many pods the deployment runs, like
// `kubectl scale deployment NAME -n NAMESPACE --replicas=N`. replicas is
// as typed, a whole number from 0. It returns the replicas before and after.
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name, replicas string, dryRun bool) (Change, error) {
	return scaleWorkload(ctx, c.clientset.AppsV1().Deployments(namespace), name, replicas, dryRun,
		func(d *appsv1.Deployment) **int32 { return &d.Spec.Replicas })
}

// RestartDeployment replaces the deployment's pods one by one, like
// `kubectl rollout restart deployment NAME -n NAMESPACE`: it stamps the
// time on the pod template, so the deployment rolls out new pods.
func (c *Client) RestartDeployment(ctx context.Context, namespace, name string, dryRun bool) error {
	_, _, err := update(ctx, c.clientset.AppsV1().Deployments(namespace), name, dryRun, func(d *appsv1.Deployment) error {
		if d.Spec.Paused {
			return errors.New("the deployment is paused: resume it first")
		}
		stampRestart(deploymentTemplate(d))
		return nil
	})
	return err
}

// DeleteDeployment deletes the deployment, like `kubectl delete deployment
// NAME -n NAMESPACE`. Its replica sets and pods are deleted after it, in the
// background.
func (c *Client) DeleteDeployment(ctx context.Context, namespace, name string, dryRun bool) error {
	return deleteObject(ctx, c.clientset.AppsV1().Deployments(namespace), name, dryRun)
}

// CreateDeployment makes a deployment called name in namespace, running
// one pod from image, like `kubectl create deployment NAME --image=IMAGE -n
// NAMESPACE`. As with kubectl, it and its pods get the label app=NAME, which
// is also its selector, and its container is named after the image.
func (c *Client) CreateDeployment(ctx context.Context, namespace, name, image string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("deployment name %q: %s", name, strings.Join(errs, "; "))
	}
	if strings.TrimSpace(image) == "" || strings.ContainsAny(image, " \t") {
		return errors.New("an image is needed, with no spaces, e.g. nginx:latest")
	}
	labels := map[string]string{"app": name}
	replicas := int32(1)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: containerName(image), Image: image}}},
			},
		},
	}
	_, err := c.clientset.AppsV1().Deployments(namespace).Create(ctx, d, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}

// containerName is the container's name kubectl gives an image: its own
// name, without registry, path, tag or digest, e.g. "nginx" for
// "docker.io/library/nginx:latest".
func containerName(image string) string {
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	if i := strings.IndexAny(image, ":@"); i >= 0 {
		image = image[:i]
	}
	return image
}
