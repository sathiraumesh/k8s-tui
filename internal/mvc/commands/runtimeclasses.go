package commands

import k8s "k8s-tui/internal/kubernetes"

// runtimeClassesView is the RuntimeClass objects in More: the container
// runtimes a pod can ask for (runtimeClassName), e.g. gVisor or Kata for
// more isolation.
func runtimeClassesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses", Kind: "RuntimeClass", Namespaced: false},
		Title:       "runtimeclass",
		DeleteAbout: "remove it; pods naming it are refused, those running keep going",
	}
}
