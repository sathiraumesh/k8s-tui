package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Setting the image of containers, the same way for a pod and a
// deployment, like `kubectl set image`. What is typed is one or more
// CONTAINER=IMAGE, with spaces between, e.g. "app=nginx:1.28 sidecar=envoy";
// "*=IMAGE" sets every container.

// imageChange is one CONTAINER=IMAGE as typed.
type imageChange struct{ container, image string }

// parseImages reads what was typed for Set Image.
func parseImages(changes []string) ([]imageChange, error) {
	if len(changes) == 0 {
		return nil, errors.New("no image given: type CONTAINER=IMAGE, e.g. app=nginx:1.28")
	}
	out := make([]imageChange, 0, len(changes))
	for _, c := range changes {
		container, image, ok := strings.Cut(c, "=")
		if !ok || container == "" || image == "" {
			return nil, fmt.Errorf("%q: type CONTAINER=IMAGE, e.g. app=nginx:1.28", c)
		}
		out = append(out, imageChange{container, image})
	}
	return out, nil
}

// setImages changes the images of spec's containers (init containers
// included) as asked. A container that isn't there is an error.
func setImages(spec *corev1.PodSpec, changes []imageChange) error {
	for _, ch := range changes {
		found := false
		for _, list := range [][]corev1.Container{spec.InitContainers, spec.Containers} {
			for i := range list {
				if ch.container == "*" || list[i].Name == ch.container {
					list[i].Image = ch.image
					found = true
				}
			}
		}
		if !found {
			return fmt.Errorf("there is no container %q", ch.container)
		}
	}
	return nil
}

// imagesOf is every container of spec as CONTAINER=IMAGE, init containers first.
func imagesOf(spec corev1.PodSpec) []string {
	var out []string
	for _, ct := range append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...) {
		out = append(out, ct.Name+"="+ct.Image)
	}
	return out
}

// SetDeploymentImage changes container images of the deployment's pod
// template, like `kubectl set image deployment/NAME -n NAMESPACE
// app=nginx:1.28`. The deployment then rolls out new pods with them. It
// returns the images before and after.
func (c *Client) SetDeploymentImage(ctx context.Context, namespace, name string, changes []string, dryRun bool) (Change, error) {
	return setWorkloadImage(ctx, c.clientset.AppsV1().Deployments(namespace), name, changes, dryRun, deploymentTemplate)
}

// SetPodImage changes container images of the pod, like `kubectl set image
// pod/NAME -n NAMESPACE app=nginx:1.28`. The image is the one thing of a
// running pod that can change: the kubelet restarts the container with it.
// If a deployment or the like owns the pod, its next pods still get the
// old image: set it on the deployment instead.
func (c *Client) SetPodImage(ctx context.Context, namespace, name string, changes []string, dryRun bool) (Change, error) {
	images, err := parseImages(changes)
	if err != nil {
		return Change{}, err
	}
	before, after, err := update(ctx, c.clientset.CoreV1().Pods(namespace), name, dryRun, func(p *corev1.Pod) error {
		return setImages(&p.Spec, images)
	})
	if err != nil {
		return Change{}, err
	}
	return Change{Before: imagesOf(before.Spec), After: imagesOf(after.Spec)}, nil
}
