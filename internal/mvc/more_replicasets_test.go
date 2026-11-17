package mvc

import (
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestReplicaSetScaleCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "replicasets   rs   apps/v1   true   ReplicaSet", "apps   web-abc   2   2   2   3h")
	if m = press(t, m, enterKey, key("c")); m.kubectlCommand() != "kubectl scale replicasets.apps web-abc -n apps" || !strings.Contains(m.View(), "scale the deployment") {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
