package commands

import k8s "k8s-tui/internal/kubernetes"

// compositePodGroupsView is the CompositePodGroup objects in More: groups of
// pod groups, scheduled together (alpha).
func compositePodGroupsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "scheduling.k8s.io", Version: "v1alpha3", Resource: "compositepodgroups", Kind: "CompositePodGroup", Namespaced: true},
		Title: "compositepodgroup",
	}
}
