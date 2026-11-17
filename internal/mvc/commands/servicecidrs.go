package commands

import k8s "k8s-tui/internal/kubernetes"

// serviceCIDRsView is the ServiceCIDR objects in More: the ranges services
// get their cluster IPs from; more can be added.
func serviceCIDRsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "networking.k8s.io", Version: "v1", Resource: "servicecidrs", Kind: "ServiceCIDR", Namespaced: false},
		Title:       "servicecidr",
		DeleteAbout: "remove it; IPs still in use keep it until their services go",
	}
}
