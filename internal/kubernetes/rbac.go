package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// What the RBAC kinds have of their own; the rest is the generic calls
// (generic.go). A Role (in a namespace) or ClusterRole (everywhere) says
// what may be done to which resources; a RoleBinding or ClusterRoleBinding
// gives a role to users, groups or service accounts.

// resourceRules turns verbs and resources, as `kubectl create role
// --verb=... --resource=...` takes them, into rules: one per API group,
// the group of each resource found from what the cluster serves
// ("deployments" is apps), or as given ("deployments.apps"). "*" is every
// resource of every group.
func (c *Client) resourceRules(ctx context.Context, verbs, resources []string) ([]rbacv1.PolicyRule, error) {
	if len(verbs) == 0 || len(resources) == 0 {
		return nil, fmt.Errorf("type verbs and resources, e.g. get,list,watch pods,deployments")
	}
	served, err := c.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	groupOf := map[string]string{}
	for _, r := range served {
		group, _, _ := strings.Cut(r.APIVersion, "/")
		if !strings.Contains(r.APIVersion, "/") {
			group = "" // core: "v1"
		}
		if _, ok := groupOf[r.Name]; !ok {
			groupOf[r.Name] = group
		}
	}
	byGroup := map[string][]string{}
	for _, res := range resources {
		name, group, explicit := strings.Cut(res, ".")
		switch {
		case res == "*":
			name, group = "*", "*"
		case !explicit:
			g, ok := groupOf[name]
			if !ok {
				return nil, fmt.Errorf("the cluster serves no resource %q", name)
			}
			group = g
		}
		byGroup[group] = append(byGroup[group], name)
	}
	groups := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	rules := make([]rbacv1.PolicyRule, 0, len(groups))
	for _, g := range groups {
		rules = append(rules, rbacv1.PolicyRule{APIGroups: []string{g}, Resources: byGroup[g], Verbs: verbs})
	}
	return rules, nil
}

// CreateRole makes a role allowing verbs on resources (see resourceRules),
// in namespace, or a ClusterRole if namespace is "", like `kubectl create
// role NAME --verb=get,list --resource=pods -n NAMESPACE` (or `kubectl
// create clusterrole`).
func (c *Client) CreateRole(ctx context.Context, namespace, name string, verbs, resources []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("role name %q: %s", name, strings.Join(errs, "; "))
	}
	rules, err := c.resourceRules(ctx, verbs, resources)
	if err != nil {
		return err
	}
	opts := metav1.CreateOptions{DryRun: dryRunOption(dryRun)}
	meta := metav1.ObjectMeta{Name: name, Namespace: namespace}
	if namespace == "" {
		_, err = c.clientset.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{ObjectMeta: meta, Rules: rules}, opts)
	} else {
		_, err = c.clientset.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: meta, Rules: rules}, opts)
	}
	return err
}
