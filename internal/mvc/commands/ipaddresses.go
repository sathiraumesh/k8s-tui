package commands

import k8s "k8s-tui/internal/kubernetes"

// ipAddressesView is the IPAddress objects in More: the cluster IPs given
// out to services, one object each.
func ipAddressesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "networking.k8s.io", Version: "v1", Resource: "ipaddresses", Kind: "IPAddress", Namespaced: false},
		Title:       "ip",
		DeleteAbout: "remove it; the API server makes it again for a service that has the IP",
	}
}
