package mvc

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretsDecode(t *testing.T) {
	cs := fake.NewClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "login"}, Data: map[string][]byte{"user": []byte("admin")}})
	m := openKindFromMore(t, cs, "secrets   v1   true   Secret", "apps   login   Opaque   1   3h")
	if !strings.Contains(m.View(), "More › secret") {
		t.Fatal("not in More › secret")
	}
	next, cmd := press(t, m, enterKey).Update(key("a"))
	if m = run(t, next.(Model), cmd); strings.Join(m.text, "\n") != "user:\n  admin" {
		t.Errorf("text = %q", m.text)
	}
}

func TestCreateGenericSecret(t *testing.T) {
	cs := fake.NewClientset()
	m := openKindFromMore(t, cs, "secrets   v1   true   Secret")
	m = press(t, m, key(KeyOwn), key("g"))
	m = typeText(t, m, "login user=admin")
	if m.kubectlCommand() != "kubectl create secret generic login --from-literal=user=admin -n default" {
		t.Errorf("command = %q", m.kubectlCommand())
	}
	_, cmd := press(t, m, enterKey).Update(key(KeyYes))
	run(t, m, cmd)
	if _, err := cs.CoreV1().Secrets("default").Get(t.Context(), "login", metav1.GetOptions{}); err != nil {
		t.Errorf("no secret: %v", err)
	}
}
