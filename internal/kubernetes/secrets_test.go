package kubernetes

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretData(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "login"},
		Data:       map[string][]byte{"user": []byte("admin"), "key": {0xff, 0xfe}},
	}
	data, err := NewClientFromClientset(fake.NewClientset(s)).SecretData(context.Background(), "apps", "login")
	if err != nil || len(data) != 2 || data[0] != (KeyValue{"key", "(binary, 2 bytes)"}) || data[1] != (KeyValue{"user", "admin"}) {
		t.Errorf("data = %q, err = %v", data, err)
	}
}

func TestCreateSecret(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateSecret(context.Background(), "apps", "login", []string{"user=admin", "password=s3cret"}, false); err != nil {
		t.Fatal(err)
	}
	s, err := cs.CoreV1().Secrets("apps").Get(context.Background(), "login", metav1.GetOptions{})
	if err != nil || s.Type != corev1.SecretTypeOpaque || s.StringData["password"] != "s3cret" {
		t.Errorf("secret = %+v, err = %v", s, err)
	}
	if err := c.CreateSecret(context.Background(), "apps", "x", []string{"novalue"}, false); err == nil {
		t.Error("bad entry: err = nil")
	}
}

func TestCreateRegistrySecret(t *testing.T) {
	cs := fake.NewClientset()
	c := NewClientFromClientset(cs)
	if err := c.CreateRegistrySecret(context.Background(), "apps", "regcred", "ghcr.io", "me", "pw", false); err != nil {
		t.Fatal(err)
	}
	s, _ := cs.CoreV1().Secrets("apps").Get(context.Background(), "regcred", metav1.GetOptions{})
	if s.Type != corev1.SecretTypeDockerConfigJson || !strings.Contains(string(s.Data[corev1.DockerConfigJsonKey]), `"ghcr.io":{"password":"pw","username":"me"}`) {
		t.Errorf("secret = %+v", s)
	}
	if err := c.CreateRegistrySecret(context.Background(), "apps", "x", "ghcr.io", "", "", false); err == nil {
		t.Error("missing user: err = nil")
	}
}
