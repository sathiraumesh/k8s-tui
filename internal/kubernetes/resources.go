package kubernetes

import (
	"context"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// Resource is one kind of thing the cluster serves, as
// `kubectl api-resources` lists it, e.g. pods, deployments.apps, or a
// custom resource.
type Resource struct {
	Name       string   // plural name, e.g. "deployments"
	ShortNames []string // e.g. ["deploy"]; empty if there are none
	APIVersion string   // group/version, e.g. "apps/v1", or "v1" for the core group
	Namespaced bool     // true if each one lives in a namespace
	Kind       string   // e.g. "Deployment"
	Verbs      []string // what can be done to it, e.g. ["get", "list", "watch"]
	Categories []string // e.g. ["all"]; empty if there are none
}

// ListResources returns every kind of thing the cluster serves, sorted by
// name, like `kubectl api-resources`. It asks the cluster's discovery API.
//
// A group can be served in several versions (autoscaling/v1 and
// autoscaling/v2); like kubectl, only the version the cluster prefers is
// listed. Subresources such as pods/log are left out.
//
// If some API groups can't be read (an extension server that is down), the
// ones that could be read are still returned, with no error.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	groups, lists, err := c.clientset.Discovery().ServerGroupsAndResourcesWithContext(ctx)
	if err != nil && !(discovery.IsGroupDiscoveryFailedError(err) && len(lists) > 0) {
		return nil, err
	}

	// The version each group prefers, e.g. "autoscaling" → "autoscaling/v2".
	preferred := make(map[string]string, len(groups))
	for _, g := range groups {
		preferred[g.Name] = g.PreferredVersion.GroupVersion
	}

	var resources []Resource
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		if p, ok := preferred[gv.Group]; ok && p != list.GroupVersion {
			continue // another version of the group is preferred
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") {
				continue // a subresource, like pods/log
			}
			resources = append(resources, Resource{
				Name:       r.Name,
				ShortNames: r.ShortNames,
				APIVersion: list.GroupVersion,
				Namespaced: r.Namespaced,
				Kind:       r.Kind,
				Verbs:      r.Verbs,
				Categories: r.Categories,
			})
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Name != resources[j].Name {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].APIVersion < resources[j].APIVersion
	})
	return resources, nil
}
