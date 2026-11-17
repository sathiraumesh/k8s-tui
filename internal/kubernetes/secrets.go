package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What secrets have of their own; the rest is the generic calls
// (generic.go). A secret's values are only base64 encoded, not encrypted:
// anyone who can read the secret can read them.

// SecretData returns the secret's entries, decoded, sorted by key. A value
// that isn't text is "(binary, N bytes)".
func (c *Client) SecretData(ctx context.Context, namespace, name string) ([]KeyValue, error) {
	s, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data := make([]KeyValue, 0, len(s.Data))
	for k, v := range s.Data {
		value := string(v)
		if !utf8.Valid(v) {
			value = fmt.Sprintf("(binary, %d bytes)", len(v))
		}
		data = append(data, KeyValue{k, value})
	}
	sort.Slice(data, func(i, j int) bool { return data[i].Key < data[j].Key })
	return data, nil
}

// CreateSecret makes an Opaque secret from key=value entries, like
// `kubectl create secret generic NAME --from-literal=key=value ... -n
// NAMESPACE`.
func (c *Client) CreateSecret(ctx context.Context, namespace, name string, literals []string, dryRun bool) error {
	data, err := parseLiterals(literals)
	if err != nil {
		return err
	}
	s := &corev1.Secret{Type: corev1.SecretTypeOpaque, StringData: data}
	return c.createSecret(ctx, namespace, name, s, dryRun)
}

// CreateRegistrySecret makes a secret pods pull images from a private
// registry with (imagePullSecrets), like `kubectl create secret
// docker-registry NAME --docker-server=SERVER --docker-username=USER
// --docker-password=PASSWORD -n NAMESPACE`.
func (c *Client) CreateRegistrySecret(ctx context.Context, namespace, name, server, user, password string, dryRun bool) error {
	if server == "" || user == "" || password == "" {
		return fmt.Errorf("type a name, the registry, a user and a password")
	}
	config, err := json.Marshal(map[string]any{"auths": map[string]any{server: map[string]string{
		"username": user, "password": password,
	}}})
	if err != nil {
		return err
	}
	s := &corev1.Secret{Type: corev1.SecretTypeDockerConfigJson, Data: map[string][]byte{corev1.DockerConfigJsonKey: config}}
	return c.createSecret(ctx, namespace, name, s, dryRun)
}

// createSecret makes s, called name, in namespace.
func (c *Client) createSecret(ctx context.Context, namespace, name string, s *corev1.Secret, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("secret name %q: %s", name, strings.Join(errs, "; "))
	}
	s.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: namespace}
	_, err := c.clientset.CoreV1().Secrets(namespace).Create(ctx, s, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
