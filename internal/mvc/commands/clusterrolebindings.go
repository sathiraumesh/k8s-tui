package commands

import k8s "k8s-tui/internal/kubernetes"

// clusterRoleBindingsView is ClusterRoleBindings in More: a cluster role
// given to users, groups or service accounts everywhere. Its own: making
// one.
func clusterRoleBindingsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings", Kind: "ClusterRoleBinding"},
		Title:       "clusterrolebinding",
		Own:         []Action{bindingAction("clusterrolebinding", "", false)},
		OwnButton:   "+ create",
		OwnTitle:    "Create a cluster role binding",
		DeleteAbout: "remove it; its subjects lose what it gave them everywhere (a built-in one, system:*, comes back on restart)",
	}
}
