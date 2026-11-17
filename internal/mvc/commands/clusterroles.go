package commands

import k8s "k8s-tui/internal/kubernetes"

// clusterRolesView is ClusterRoles in More: what may be done to which
// resources, in every namespace (or to things not in one, like nodes).
// Built in: cluster-admin, admin, edit, view. Its own: making one.
func clusterRolesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles", Kind: "ClusterRole"},
		Title:       "clusterrole",
		Own:         []Action{roleAction("clusterrole", "", false)},
		OwnButton:   "+ create",
		OwnTitle:    "Create a cluster role",
		DeleteAbout: "remove it; the bindings to it then give nothing (a built-in one, system:*, comes back on restart)",
	}
}
