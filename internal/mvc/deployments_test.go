package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// deploymentsOnScreen is a model on the Deployments tab of a fake cluster
// with the deployment default/web, of 2 pods.
func deploymentsOnScreen(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	replicas := int32(2)
	labels := map[string]string{"app": "web"}
	cs := fake.NewClientset(readyNode("worker1"), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web", CreationTimestamp: metav1.Now()},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
			},
		},
	})
	m := sized(t, New(k8s.NewClientFromClientset(cs)), 120, 30)
	m.active = 2
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabDeployments, Rows: []string{"default   web   2/2   2   2   3h"}, At: time.Now()})
	return m, cs
}

func TestLoadDeployments(t *testing.T) {
	_, cs := deploymentsOnScreen(t)
	msg := commands.LoadDeployments(k8s.NewClientFromClientset(cs), false, "")().(commands.ItemsLoadedMsg)
	if msg.Err != nil || msg.Tab != tabDeployments || len(msg.Rows) != 1 || !strings.HasPrefix(msg.Rows[0], "default   web   0/2") {
		t.Errorf("LoadDeployments = %+v", msg)
	}
}

// openDeploymentAction opens the action with key k for default/web.
func openDeploymentAction(t *testing.T, k string) (Model, *fake.Clientset) {
	t.Helper()
	m, cs := deploymentsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key(k))
	if k == "i" { // its containers load first: it has one
		m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"app"}})
	}
	return m, cs
}

func TestDeploymentActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe deployment web -n default",
		"s": "kubectl get deployment web -n default --show-labels",
		"c": "kubectl scale deployment web -n default",
		"t": "kubectl rollout restart deployment web -n default",
		"l": "kubectl label deployment web -n default",
		"r": "kubectl delete deployment web -n default",
	} {
		if k == "s" {
			continue // Show runs straight away: no confirm page
		}
		if m, _ := openDeploymentAction(t, k); m.kubectlCommand() != want {
			t.Errorf("%s: command = %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

func TestDeploymentScaleRuns(t *testing.T) {
	m, cs := openDeploymentAction(t, "c")
	m = press(t, m, key("4"))
	if m.kubectlCommand() != "kubectl scale deployment web -n default --replicas=4" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	m = run(t, m, cmd)
	d, _ := cs.AppsV1().Deployments("default").Get(t.Context(), "web", metav1.GetOptions{})
	if *d.Spec.Replicas != 4 || !strings.Contains(m.notice, "done") {
		t.Errorf("replicas = %d, notice = %q", *d.Spec.Replicas, m.notice)
	}
}

// + opens the images to make a deployment from, like Run on Pods.
func TestCreateDeployment(t *testing.T) {
	m, cs := deploymentsOnScreen(t)
	if b := buttonNamed(t, m, buttonOwn); b.label != "+ create" {
		t.Errorf("button = %+v", b)
	}
	m = press(t, m, key(KeyOwn))
	if m.overlayTitle() != "Create a deployment in default" || m.kubectlCommand() != "kubectl create deployment nginx --image=nginx:latest -n default" {
		t.Fatalf("title = %q, command = %q", m.overlayTitle(), m.kubectlCommand())
	}
	m = press(t, m, key("n"))
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if _, err := cs.AppsV1().Deployments("default").Get(t.Context(), "nginx", metav1.GetOptions{}); err != nil {
		t.Errorf("no deployment made: %v", err)
	}
}

// Set image starts its field with the container picked, and sets it.
func TestDeploymentSetImage(t *testing.T) {
	m, cs := openDeploymentAction(t, "i") // one container: app
	if m.overlay != overlayConfirm || m.input != "app=" || !m.typing() {
		t.Fatalf("overlay = %d, input = %q, typing = %v", m.overlay, m.input, m.typing())
	}
	m = typeText(t, m, "nginx:1.28")
	if m.kubectlCommand() != "kubectl set image deployment/web -n default app=nginx:1.28" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	m = run(t, m, cmd)
	d, _ := cs.AppsV1().Deployments("default").Get(t.Context(), "web", metav1.GetOptions{})
	if d.Spec.Template.Spec.Containers[0].Image != "nginx:1.28" || m.notice != "Set image default/web: done" {
		t.Errorf("image = %q, notice = %q", d.Spec.Template.Spec.Containers[0].Image, m.notice)
	}
}

// With several containers the picker asks which; "all containers" is "*=".
func TestDeploymentSetImagePick(t *testing.T) {
	m, _ := deploymentsOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("i"))
	m = press(t, m, containersLoadedMsg{target: "default/web", names: []string{"app", "sidecar"}})
	if got := m.containerRows(); m.overlay != overlayContainers || len(got) != 3 || got[0] != "all containers" {
		t.Fatalf("overlay = %d, rows = %q", m.overlay, got)
	}
	if m = press(t, m, enterKey); m.input != "*=" {
		t.Errorf("all containers: input = %q, want *=", m.input)
	}
	m = press(t, m, escKey, key("i"), containersLoadedMsg{target: "default/web", names: []string{"app", "sidecar"}}, down, down, enterKey)
	if m.input != "sidecar=" {
		t.Errorf("sidecar: input = %q, want sidecar=", m.input)
	}
}
