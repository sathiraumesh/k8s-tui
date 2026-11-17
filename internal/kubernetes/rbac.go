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
	groupOf, err := c.resourceGroups(ctx)
	if err != nil {
		return nil, err
	}
	byGroup := map[string][]string{}
	for _, res := range resources {
		name, group := "*", "*"
		if res != "*" {
			if name, group, err = groupOf(res); err != nil {
				return nil, err
			}
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

// resourceGroups returns what finds the API group of a resource as typed:
// "deployments" is in apps, found from what the cluster serves;
// "deployments.apps" names it; "pods" is in the core group, "".
func (c *Client) resourceGroups(ctx context.Context) (func(resource string) (name, group string, err error), error) {
	served, err := c.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	groupOf := map[string]string{}
	for _, r := range served {
		group, _, grouped := strings.Cut(r.APIVersion, "/")
		if !grouped {
			group = "" // core: "v1"
		}
		if _, ok := groupOf[r.Name]; !ok {
			groupOf[r.Name] = group
		}
	}
	return func(resource string) (string, string, error) {
		if name, group, explicit := strings.Cut(resource, "."); explicit {
			return name, group, nil
		}
		group, ok := groupOf[resource]
		if !ok {
			return "", "", fmt.Errorf("the cluster serves no resource %q", resource)
		}
		return resource, group, nil
	}, nil
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

// parseRoleRef reads the role a binding gives: "role:NAME" or
// "clusterrole:NAME". A ClusterRoleBinding can only give a ClusterRole.
func parseRoleRef(role string, cluster bool) (rbacv1.RoleRef, error) {
	kind, name, ok := strings.Cut(role, ":")
	switch {
	case ok && name != "" && kind == "clusterrole":
		return rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: name}, nil
	case ok && name != "" && kind == "role" && !cluster:
		return rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}, nil
	case cluster:
		return rbacv1.RoleRef{}, fmt.Errorf("role %q: type clusterrole:NAME, e.g. clusterrole:view", role)
	}
	return rbacv1.RoleRef{}, fmt.Errorf("role %q: type role:NAME or clusterrole:NAME", role)
}

// parseSubject reads who a binding is for: "user:NAME", "group:NAME" or
// "serviceaccount:NAMESPACE:NAME".
func parseSubject(s string) (rbacv1.Subject, error) {
	kind, rest, _ := strings.Cut(s, ":")
	switch kind {
	case "user":
		if rest != "" {
			return rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: rest}, nil
		}
	case "group":
		if rest != "" {
			return rbacv1.Subject{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: rest}, nil
		}
	case "serviceaccount":
		if ns, name, ok := strings.Cut(rest, ":"); ok && ns != "" && name != "" {
			return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: ns, Name: name}, nil
		}
	}
	return rbacv1.Subject{}, fmt.Errorf("subject %q: type user:NAME, group:NAME or serviceaccount:NAMESPACE:NAME", s)
}

// CreateBinding gives role (see parseRoleRef) to subjects (see
// parseSubject) in namespace, or everywhere with a ClusterRoleBinding if
// namespace is "", like `kubectl create rolebinding NAME --clusterrole=view
// --user=alice -n NAMESPACE` (or `kubectl create clusterrolebinding`).
func (c *Client) CreateBinding(ctx context.Context, namespace, name, role string, subjects []string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("binding name %q: %s", name, strings.Join(errs, "; "))
	}
	ref, err := parseRoleRef(role, namespace == "")
	if err != nil {
		return err
	}
	if len(subjects) == 0 {
		return fmt.Errorf("no subject given: type user:NAME, group:NAME or serviceaccount:NAMESPACE:NAME")
	}
	var subs []rbacv1.Subject
	for _, s := range subjects {
		sub, err := parseSubject(s)
		if err != nil {
			return err
		}
		subs = append(subs, sub)
	}
	opts := metav1.CreateOptions{DryRun: dryRunOption(dryRun)}
	meta := metav1.ObjectMeta{Name: name, Namespace: namespace}
	if namespace == "" {
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: meta, RoleRef: ref, Subjects: subs}, opts)
	} else {
		_, err = c.clientset.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: meta, RoleRef: ref, Subjects: subs}, opts)
	}
	return err
}
