package kubernetes

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeDeployment is a deployment of 3 pods labelled app=NAME, 2 of them
// ready, with containers app and sidecar.
func fakeDeployment(namespace, name string) *appsv1.Deployment {
	replicas := int32(3)
	labels := map[string]string{"app": name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "setup", Image: "busybox"}},
					Containers:     []corev1.Container{{Name: "app", Image: "nginx:1.27"}, {Name: "sidecar", Image: "envoy"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 2, UpdatedReplicas: 3, AvailableReplicas: 2},
	}
}

func TestListDeployments(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("b", "web"), fakeDeployment("a", "api"))
	got, err := NewClientFromClientset(cs).ListDeployments(context.Background(), "")
	if err != nil || len(got) != 2 || got[0].Name != "api" || got[1].Name != "web" {
		t.Fatalf("deployments = %+v, err = %v", got, err)
	}
	d := got[0]
	if d.Ready != "2/3" || d.UpToDate != 3 || d.Available != 2 || d.Selector != "app=api" ||
		strings.Join(d.Containers, ",") != "app,sidecar" || strings.Join(d.Images, ",") != "nginx:1.27,envoy" {
		t.Errorf("deployment = %+v", d)
	}
}

func TestListDeploymentsInNamespace(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("a", "one"), fakeDeployment("b", "two"))
	got, err := NewClientFromClientset(cs).ListDeployments(context.Background(), "b")
	if err != nil || len(got) != 1 || got[0].Name != "two" {
		t.Errorf("deployments = %+v, err = %v, want only b/two", got, err)
	}
}

func TestDeploymentContainers(t *testing.T) {
	c := NewClientFromClientset(fake.NewClientset(fakeDeployment("apps", "web")))
	all, err := c.DeploymentContainers(context.Background(), "apps", "web", false)
	if err != nil || strings.Join(all, ",") != "setup,app,sidecar" {
		t.Errorf("containers = %v, err = %v", all, err)
	}
	running, _ := c.DeploymentContainers(context.Background(), "apps", "web", true)
	if strings.Join(running, ",") != "app,sidecar" {
		t.Errorf("running containers = %v, want no init container", running)
	}
}

func TestScaleDeployment(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	c := NewClientFromClientset(cs)
	ch, err := c.ScaleDeployment(context.Background(), "apps", "web", "5", false)
	if err != nil || ch.Before[0] != "replicas=3" || ch.After[0] != "replicas=5" {
		t.Fatalf("change = %+v, err = %v", ch, err)
	}
	d, _ := cs.AppsV1().Deployments("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if *d.Spec.Replicas != 5 {
		t.Errorf("replicas = %d, want 5", *d.Spec.Replicas)
	}
	for _, bad := range []string{"", "-1", "two", "1.5"} {
		if _, err := c.ScaleDeployment(context.Background(), "apps", "web", bad, false); err == nil {
			t.Errorf("replicas %q: err = nil", bad)
		}
	}
}

func TestRestartDeployment(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	if err := NewClientFromClientset(cs).RestartDeployment(context.Background(), "apps", "web", false); err != nil {
		t.Fatal(err)
	}
	d, _ := cs.AppsV1().Deployments("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if d.Spec.Template.Annotations[restartedAtAnnotation] == "" {
		t.Error("the template has no restartedAt annotation")
	}
}

func TestRestartPausedDeployment(t *testing.T) {
	d := fakeDeployment("apps", "web")
	d.Spec.Paused = true
	if err := NewClientFromClientset(fake.NewClientset(d)).RestartDeployment(context.Background(), "apps", "web", false); err == nil {
		t.Error("err = nil, want: paused")
	}
}

func TestLabelDeployment(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	ch, err := NewClientFromClientset(cs).LabelDeployment(context.Background(), "apps", "web", []string{"env=prod"}, false, false)
	if err != nil || len(ch.After) != 1 || ch.After[0] != "env=prod" {
		t.Errorf("change = %+v, err = %v", ch, err)
	}
}

func TestDeleteDeployment(t *testing.T) {
	cs := fake.NewClientset(fakeDeployment("apps", "web"))
	if err := NewClientFromClientset(cs).DeleteDeployment(context.Background(), "apps", "web", false); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.AppsV1().Deployments("apps").Get(context.Background(), "web", metav1.GetOptions{}); err == nil {
		t.Error("the deployment is still there")
	}
}

func TestCreateDeployment(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateDeployment(context.Background(), "apps", "web", "docker.io/library/nginx:latest", false); err != nil {
		t.Fatal(err)
	}
	d, err := cs.AppsV1().Deployments("apps").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil || d.Labels["app"] != "web" || d.Spec.Selector.MatchLabels["app"] != "web" ||
		d.Spec.Template.Spec.Containers[0].Name != "nginx" || *d.Spec.Replicas != 1 {
		t.Errorf("deployment = %+v, err = %v", d, err)
	}
	if err := c.CreateDeployment(context.Background(), "apps", "Bad_Name", "nginx", false); err == nil {
		t.Error("bad name: err = nil")
	}
	if err := c.CreateDeployment(context.Background(), "apps", "x", "", false); err == nil {
		t.Error("no image: err = nil")
	}
}

func TestDescribeDeploymentNeedsCluster(t *testing.T) {
	if _, err := NewClientFromClientset(fake.NewClientset()).DescribeDeployment(context.Background(), "apps", "web", true); err == nil {
		t.Error("err = nil on a pretend cluster")
	}
}
