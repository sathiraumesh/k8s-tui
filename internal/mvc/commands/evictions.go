package commands

import k8s "k8s-tui/internal/kubernetes"

// evictionsView is the Eviction objects in More: requests to evict pods,
// kept as objects (alpha; kubectl drain uses the pods/eviction subresource).
func evictionsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "lifecycle.k8s.io", Version: "v1alpha1", Resource: "evictions", Kind: "Eviction", Namespaced: true},
		Title: "eviction",
	}
}
