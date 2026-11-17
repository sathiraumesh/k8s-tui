package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What service accounts have of their own; the rest is the generic calls
// (generic.go).

// ServiceAccountToken asks the API server for a token of the service
// account that lasts for duration (e.g. "1h"), like `kubectl create token
// NAME --duration=1h -n NAMESPACE`. The token isn't stored anywhere: it is
// only in the answer. The server may give a shorter one than asked for.
func (c *Client) ServiceAccountToken(ctx context.Context, namespace, name, duration string) (token string, expires time.Time, err error) {
	d, err := time.ParseDuration(strings.TrimSpace(duration))
	if err != nil || d < 10*time.Minute {
		return "", time.Time{}, fmt.Errorf("duration %q: type at least 10m, e.g. 1h", duration)
	}
	seconds := int64(d.Seconds())
	req := &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &seconds}}
	resp, err := c.clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, name, req, metav1.CreateOptions{})
	if err != nil {
		return "", time.Time{}, err
	}
	return resp.Status.Token, resp.Status.ExpirationTimestamp.Time, nil
}

// CreateServiceAccount makes a service account, like `kubectl create
// serviceaccount NAME -n NAMESPACE`: an identity for pods, given
// permissions with role bindings.
func (c *Client) CreateServiceAccount(ctx context.Context, namespace, name string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("service account name %q: %s", name, strings.Join(errs, "; "))
	}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	_, err := c.clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
