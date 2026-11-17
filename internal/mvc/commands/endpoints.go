package commands

import k8s "k8s-tui/internal/kubernetes"

// endpointsView is the Endpoints objects in More: the addresses a service
// sends traffic to (replaced by EndpointSlices).
func endpointsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "", Version: "v1", Resource: "endpoints", Kind: "Endpoints", Namespaced: true},
		Title:       "ep",
		DeleteAbout: "remove them; the endpoints controller makes them again for a service with a selector",
	}
}
