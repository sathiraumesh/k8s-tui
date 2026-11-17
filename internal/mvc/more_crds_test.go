package mvc

import (
	"strings"
	"testing"

	k8s "k8s-tui/internal/kubernetes"
	"k8s-tui/internal/mvc/commands"

	"k8s.io/client-go/kubernetes/fake"
)

// Objects of a CRD open as a view in More, like any kind's.
func TestOpenCRDObjects(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "customresourcedefinitions   crd   apiextensions.k8s.io/v1   false   CustomResourceDefinition", "widgets.example.com   3h")
	if m = press(t, m, enterKey, key("o")); m.kubectlCommand() != "kubectl get widgets.example.com" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	widgets := commands.View{Type: k8s.ResourceType{Group: "example.com", Version: "v1", Resource: "widgets", Kind: "Widget", Namespaced: true}, Title: "widgets"}
	next, _ := m.Update(commands.OpenViewMsg{View: widgets})
	m = next.(Model)
	if !m.inKind() || m.activeName() != "widgets.example.com" || m.overlay != overlayNone || !strings.Contains(m.View(), "More › widgets") {
		t.Errorf("active = %q, overlay = %d", m.activeName(), m.overlay)
	}
	if m.kubectlCommand() != "kubectl get widgets.example.com --all-namespaces" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
