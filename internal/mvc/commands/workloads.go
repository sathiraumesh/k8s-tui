package commands

import k8s "k8s-tui/internal/kubernetes"

// workloadsView is the Workload objects in More: a workload's pod groups,
// for gang scheduling (beta).
func workloadsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "scheduling.k8s.io", Version: "v1beta1", Resource: "workloads", Kind: "Workload", Namespaced: true},
		Title: "workload",
	}
}
