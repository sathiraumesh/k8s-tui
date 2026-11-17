package kubernetes

import (
	"context"
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What network policies have of their own; the rest is the generic calls
// (generic.go). A policy says which traffic may reach the pods it picks;
// once one picks a pod, all other traffic to it is refused. Only a network
// plugin that enforces policies (Calico, Cilium, …) makes them work.

// PolicyKind is a ready-made network policy for every pod of a namespace.
type PolicyKind int

const (
	// DenyAllIngress refuses all traffic to the pods: the usual start, to
	// then allow what is needed.
	DenyAllIngress PolicyKind = iota
	// AllowSameNamespace lets in only traffic from pods of the same
	// namespace.
	AllowSameNamespace
)

// CreateNetworkPolicy makes the policy kind, called name, for every pod
// in namespace. kubectl has no command for it; it is the YAML `kubectl
// apply -f` would take for
//
//	kind: NetworkPolicy
//	spec:
//	  podSelector: {}            # every pod of the namespace
//	  policyTypes: [Ingress]
//	  ingress: [{from: [{podSelector: {}}]}]   # AllowSameNamespace only
func (c *Client) CreateNetworkPolicy(ctx context.Context, namespace, name string, kind PolicyKind, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("policy name %q: %s", name, strings.Join(errs, "; "))
	}
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}
	if kind == AllowSameNamespace {
		np.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
		}}
	}
	_, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, np, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
