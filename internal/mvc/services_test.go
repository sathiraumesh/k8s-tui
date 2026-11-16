package mvc

import (
	"strings"
	"testing"
	"time"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// servicesOnScreen is a model on the Services tab of a fake cluster with
// the service default/web.
func servicesOnScreen(t *testing.T) (Model, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(readyNode("worker1"), &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web", CreationTimestamp: metav1.Now()},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.96.0.10", Selector: map[string]string{"app": "web"}, Ports: []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}}},
	})
	m := sized(t, New(k8s.NewClientFromClientset(cs)), 120, 30)
	m.active = 3
	m = press(t, m, commands.ItemsLoadedMsg{Tab: tabServices, Rows: []string{"default   web   ClusterIP   10.96.0.10   <none>   80/TCP   3h"}, At: time.Now()})
	return m, cs
}

func TestLoadServices(t *testing.T) {
	_, cs := servicesOnScreen(t)
	msg := commands.LoadServices(k8s.NewClientFromClientset(cs), false, "")().(commands.ItemsLoadedMsg)
	if msg.Err != nil || msg.Tab != tabServices || len(msg.Rows) != 1 || !strings.HasPrefix(msg.Rows[0], "default   web   ClusterIP   10.96.0.10") {
		t.Errorf("LoadServices = %+v", msg)
	}
}

func TestServiceActionCommands(t *testing.T) {
	for k, want := range map[string]string{
		"d": "kubectl describe service web -n default",
		"f": "kubectl port-forward service/web -n default",
		"l": "kubectl label service web -n default",
		"r": "kubectl delete service web -n default",
	} {
		m, _ := servicesOnScreen(t)
		if m = press(t, m, down, down, down, enterKey, key(k)); m.kubectlCommand() != want {
			t.Errorf("%s: command = %q, want %q", k, m.kubectlCommand(), want)
		}
	}
}

func TestServiceDescribeRuns(t *testing.T) {
	m, _ := servicesOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("d"))
	_, cmd := m.Update(enterKey)
	if m = run(t, m, cmd); m.overlay != overlayText || !strings.Contains(strings.Join(m.text, "\n"), "10.96.0.10") {
		t.Errorf("overlay = %d, text = %q, notice = %q", m.overlay, m.text, m.notice)
	}
}

func TestServicePortForwardCommand(t *testing.T) {
	m, _ := servicesOnScreen(t)
	m = press(t, m, down, down, down, enterKey, key("f"))
	m = typeText(t, m, "8080:80")
	if m.kubectlCommand() != "kubectl port-forward service/web -n default 8080:80" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}

// + opens the service types; picking one and typing a name and ports makes it.
func TestCreateServiceFromTab(t *testing.T) {
	m, cs := servicesOnScreen(t)
	m = press(t, m, key(KeyOwn))
	if m.overlayTitle() != "Create a service in default" {
		t.Fatalf("title = %q", m.overlayTitle())
	}
	m = press(t, m, key("n")) // NodePort
	m = typeText(t, m, "api 80:8080")
	if m.kubectlCommand() != "kubectl create service nodeport api --tcp=80:8080 -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	s, err := cs.CoreV1().Services("default").Get(t.Context(), "api", metav1.GetOptions{})
	if err != nil || s.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("service = %+v, err = %v", s, err)
	}
}

// Expose on a deployment makes its service.
func TestExposeFromDeployments(t *testing.T) {
	m, cs := openDeploymentAction(t, "p")
	m = typeText(t, m, "80:8080")
	if m.kubectlCommand() != "kubectl expose deployment web -n default --port=80 --target-port=8080" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	if m = run(t, m, cmd); m.notice != "Expose default/web: done" {
		t.Errorf("notice = %q", m.notice)
	}
	if _, err := cs.CoreV1().Services("default").Get(t.Context(), "web", metav1.GetOptions{}); err != nil {
		t.Errorf("no service: %v", err)
	}
}
