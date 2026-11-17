package mvc

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateRoleBindingFromView(t *testing.T) {
	cs := fake.NewClientset()
	m := openKindFromMore(t, cs, "rolebindings   rbac.authorization.k8s.io/v1   true   RoleBinding")
	m = typeText(t, press(t, m, key(KeyOwn), key("c")), "viewers clusterrole:view user:alice")
	if m.kubectlCommand() != "kubectl create rolebinding viewers --clusterrole=view --user=alice -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if b, err := cs.RbacV1().RoleBindings("default").Get(t.Context(), "viewers", metav1.GetOptions{}); err != nil || b.Subjects[0].Name != "alice" {
		t.Errorf("binding = %+v, err = %v", b, err)
	}
}
