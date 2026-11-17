package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What jobs have of their own; the rest is the generic calls (generic.go).
// A job runs pods until enough of them finish well.

// jobPodLabel is the label a job puts on its pods.
const jobPodLabel = "batch.kubernetes.io/job-name"

// JobLogs returns the last tail lines each of the job's pods wrote, like
// `kubectl logs job/NAME --all-containers -n NAMESPACE` but for every pod
// (a failed try, then the one that worked), each under a heading:
//
//	==> pi-abcde/pi <==
func (c *Client) JobLogs(ctx context.Context, namespace, name string, tail int64) (string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: jobPodLabel + "=" + name})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", errors.New("the job has no pods (they may have been cleaned up)")
	}
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].CreationTimestamp.Before(&pods.Items[j].CreationTimestamp) })
	var b strings.Builder
	for _, p := range pods.Items {
		for _, ct := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
			logs, err := c.containerLogs(ctx, namespace, p.Name, ct.Name, false, false, tail)
			writeLogs(&b, p.Name+"/"+ct.Name, logs, err)
		}
	}
	return b.String(), nil
}

// SuspendJob stops the job's pods and starts no more (suspend true), or
// lets it go on (false), like `kubectl patch job NAME -p
// '{"spec":{"suspend":true}}'`.
func (c *Client) SuspendJob(ctx context.Context, namespace, name string, suspend, dryRun bool) error {
	_, _, err := update(ctx, c.clientset.BatchV1().Jobs(namespace), name, dryRun, func(j *batchv1.Job) error {
		if j.Spec.Suspend != nil && *j.Spec.Suspend == suspend {
			if suspend {
				return errors.New("it is suspended already")
			}
			return errors.New("it isn't suspended")
		}
		j.Spec.Suspend = &suspend
		return nil
	})
	return err
}

// CreateJob makes a job of one pod from image, running command if it is
// given (else the image's own), like `kubectl create job NAME
// --image=IMAGE -n NAMESPACE -- COMMAND`. The pod isn't restarted when it
// fails: the job makes a new one, up to its backoff limit (6).
func (c *Client) CreateJob(ctx context.Context, namespace, name, image string, command []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("job name %q: %s", name, strings.Join(errs, "; "))
	}
	if strings.TrimSpace(image) == "" {
		return errors.New("an image is needed, e.g. busybox")
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: name, Image: image, Command: command}},
		}}},
	}
	_, err := c.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
