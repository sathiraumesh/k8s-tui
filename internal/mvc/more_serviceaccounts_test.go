package mvc

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServiceAccountTokenCommand(t *testing.T) {
	m := openKindFromMore(t, fake.NewClientset(), "serviceaccounts   sa   v1   true   ServiceAccount", "apps   deployer   0   3h")
	m = press(t, m, enterKey, key("t"))
	if m.input != "1h" || m.kubectlCommand() != "kubectl create token deployer -n apps --duration=1h" {
		t.Errorf("input %q, command %q", m.input, m.kubectlCommand())
	}
}

func TestCreateServiceAccountFromView(t *testing.T) {
	cs := fake.NewClientset()
	m := openKindFromMore(t, cs, "serviceaccounts   sa   v1   true   ServiceAccount")
	m = press(t, m, key(KeyOwn), key("s"))
	m = typeText(t, m, "deployer")
	if !strings.HasPrefix(m.kubectlCommand(), "kubectl create serviceaccount deployer -n default") {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if _, err := cs.CoreV1().ServiceAccounts("default").Get(t.Context(), "deployer", metav1.GetOptions{}); err != nil {
		t.Errorf("no service account: %v", err)
	}
}
