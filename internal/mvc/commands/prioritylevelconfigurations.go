package commands

import k8s "k8s-tui/internal/kubernetes"

// priorityLevelConfigurationsView is the PriorityLevelConfiguration objects
// in More: how much of the API server's capacity each priority level of
// requests gets.
func priorityLevelConfigurationsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "flowcontrol.apiserver.k8s.io", Version: "v1", Resource: "prioritylevelconfigurations", Kind: "PriorityLevelConfiguration", Namespaced: false},
		Title:       "prioritylevel",
		DeleteAbout: "remove it; schemas naming it fall to the next (a built-in one comes back)",
	}
}
