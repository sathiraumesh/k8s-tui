package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateClusterRoleCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "clusterroles   rbac.authorization.k8s.io/v1   false   ClusterRole")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "node-reader get,list nodes")
	if m.kubectlCommand() != "kubectl create clusterrole node-reader --verb=get,list --resource=nodes" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
