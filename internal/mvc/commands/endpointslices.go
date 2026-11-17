package commands

import k8s "k8s-tui/internal/kubernetes"

// endpointSlicesView is the EndpointSlice objects in More: the addresses of
// a service's pods, in slices of up to 100, ready or not.
func endpointSlicesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices", Kind: "EndpointSlice", Namespaced: true},
		Title:       "endpointslice",
		DeleteAbout: "remove it; the endpointslice controller makes it again for a service with a selector",
	}
}
