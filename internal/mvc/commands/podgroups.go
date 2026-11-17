package commands

import k8s "k8s-tui/internal/kubernetes"

// podGroupsView is the PodGroup objects in More: pods to be scheduled
// together, all or none (gang scheduling, beta).
func podGroupsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "scheduling.k8s.io", Version: "v1beta1", Resource: "podgroups", Kind: "PodGroup", Namespaced: true},
		Title: "podgroup",
	}
}
