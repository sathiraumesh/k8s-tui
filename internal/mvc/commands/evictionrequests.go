package commands

import k8s "k8s-tui/internal/kubernetes"

// evictionRequestsView is the EvictionRequest objects in More: requests that
// a pod be evicted, handled by the pod's interceptors (alpha).
func evictionRequestsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "lifecycle.k8s.io", Version: "v1alpha1", Resource: "evictionrequests", Kind: "EvictionRequest", Namespaced: true},
		Title: "evictionrequest",
	}
}
