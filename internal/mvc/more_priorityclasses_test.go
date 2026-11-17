package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

// A kind not in a namespace makes its own with no -n.
func TestCreatePriorityClassCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "priorityclasses   pc   scheduling.k8s.io/v1   false   PriorityClass")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "high 1000")
	if m.kubectlCommand() != "kubectl create priorityclass high --value=1000" || m.overlayTitle() != "Create a priority class" {
		t.Errorf("command = %q, title = %q", m.kubectlCommand(), m.overlayTitle())
	}
}
