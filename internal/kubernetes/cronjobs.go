package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What cronjobs have of their own; the rest is the generic calls
// (generic.go). A cronjob makes a job on a schedule, e.g. every night.

// SuspendCronJob stops it making jobs (suspend true), or lets it go on
// (false), like `kubectl patch cronjob NAME -p '{"spec":{"suspend":true}}'`.
// Jobs already running go on.
func (c *Client) SuspendCronJob(ctx context.Context, namespace, name string, suspend, dryRun bool) error {
	_, _, err := update(ctx, c.clientset.BatchV1().CronJobs(namespace), name, dryRun, func(cj *batchv1.CronJob) error {
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend == suspend {
			if suspend {
				return errors.New("it is suspended already")
			}
			return errors.New("it isn't suspended")
		}
		cj.Spec.Suspend = &suspend
		return nil
	})
	return err
}

// RunCronJob makes a job from the cronjob now, without waiting for its
// schedule, like `kubectl create job NAME-manual-XXXXX
// --from=cronjob/NAME`. It returns the job's name.
func (c *Client) RunCronJob(ctx context.Context, namespace, name string, dryRun bool) (string, error) {
	cj, err := c.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	jobName := fmt.Sprintf("%s-manual-%d", name, time.Now().Unix()%100000)
	if len(jobName) > validation.DNS1123LabelMaxLength {
		jobName = jobName[len(jobName)-validation.DNS1123LabelMaxLength:]
	}
	isController := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Namespace:   namespace,
			Labels:      cj.Spec.JobTemplate.Labels,
			Annotations: map[string]string{"cronjob.kubernetes.io/instantiate": "manual"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1", Kind: "CronJob", Name: cj.Name, UID: cj.UID, Controller: &isController,
			}},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}
	_, err = c.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return jobName, err
}

// CreateCronJob makes a cronjob running image (and command, if given) on
// schedule, a cron schedule like "*/5 * * * *" or "@hourly", like
// `kubectl create cronjob NAME --image=IMAGE --schedule=SCHEDULE -n
// NAMESPACE -- COMMAND`.
func (c *Client) CreateCronJob(ctx context.Context, namespace, name, image, schedule string, command []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("cronjob name %q: %s", name, strings.Join(errs, "; "))
	}
	if image == "" || schedule == "" {
		return errors.New("type a name, an image and a schedule, e.g. backup busybox @hourly")
	}
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule,
			JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers:    []corev1.Container{{Name: name, Image: image, Command: command}},
			}}}},
		},
	}
	_, err := c.clientset.BatchV1().CronJobs(namespace).Create(ctx, cj, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
