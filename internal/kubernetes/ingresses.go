package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What ingresses have of their own; the rest is the generic calls
// (generic.go). An ingress routes HTTP from outside the cluster to
// services, by host and path; an ingress controller (nginx, traefik, …)
// does the routing.

// parseRule reads a rule as `kubectl create ingress --rule` takes it:
// HOST/PATH=SERVICE:PORT, e.g. "shop.example.com/api=api:80". The host may
// be left out ("/=web:80" for every host); a path ending in * matches what
// starts with it (Prefix), any other only itself (Exact).
func parseRule(rule string) (networkingv1.IngressRule, error) {
	where, backend, ok := strings.Cut(rule, "=")
	service, portText, hasPort := strings.Cut(backend, ":")
	host, path, hasPath := strings.Cut(where, "/")
	port, err := strconv.Atoi(portText)
	if !ok || !hasPort || !hasPath || service == "" || err != nil || port < 1 || port > 65535 {
		return networkingv1.IngressRule{}, fmt.Errorf("rule %q: type HOST/PATH=SERVICE:PORT, e.g. shop.example.com/=web:80", rule)
	}
	pathType := networkingv1.PathTypeExact
	path = "/" + path
	if strings.HasSuffix(path, "*") {
		path, pathType = strings.TrimSuffix(path, "*"), networkingv1.PathTypePrefix
	}
	return networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
			Paths: []networkingv1.HTTPIngressPath{{
				Path:     path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
					Name: service, Port: networkingv1.ServiceBackendPort{Number: int32(port)},
				}},
			}},
		}},
	}, nil
}

// CreateIngress makes an ingress from rules (see parseRule), like `kubectl
// create ingress NAME --rule=HOST/PATH=SERVICE:PORT ... -n NAMESPACE`.
func (c *Client) CreateIngress(ctx context.Context, namespace, name string, rules []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("ingress name %q: %s", name, strings.Join(errs, "; "))
	}
	if len(rules) == 0 {
		return fmt.Errorf("no rule given: type HOST/PATH=SERVICE:PORT, e.g. shop.example.com/=web:80")
	}
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	for _, r := range rules {
		rule, err := parseRule(r)
		if err != nil {
			return err
		}
		ing.Spec.Rules = append(ing.Spec.Rules, rule)
	}
	_, err := c.clientset.NetworkingV1().Ingresses(namespace).Create(ctx, ing, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
