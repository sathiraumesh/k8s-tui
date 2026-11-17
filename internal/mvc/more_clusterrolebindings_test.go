package mvc

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateClusterRoleBindingCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "clusterrolebindings   rbac.authorization.k8s.io/v1   false   ClusterRoleBinding")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "admins clusterrole:cluster-admin user:alice")
	if m.kubectlCommand() != "kubectl create clusterrolebinding admins --clusterrole=cluster-admin --user=alice" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
}
