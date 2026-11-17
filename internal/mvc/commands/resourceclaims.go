package commands

import k8s "k8s-tui/internal/kubernetes"

// resourceClaimsView is the ResourceClaim objects in More: requests for
// devices (a GPU) for pods, allocated by the scheduler from what drivers
// publish.
func resourceClaimsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "resource.k8s.io", Version: "v1", Resource: "resourceclaims", Kind: "ResourceClaim", Namespaced: true},
		Title:       "resourceclaim",
		DeleteAbout: "remove it; it waits until no pod uses it",
	}
}
