package commands

import k8s "k8s-tui/internal/kubernetes"

// resourcePoolStatusRequestsView is the ResourcePoolStatusRequest objects in
// More: asks for the state of a pool of devices (alpha).
func resourcePoolStatusRequestsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "resource.k8s.io", Version: "v1alpha3", Resource: "resourcepoolstatusrequests", Kind: "ResourcePoolStatusRequest", Namespaced: false},
		Title: "poolstatus",
	}
}
