package commands

import k8s "k8s-tui/internal/kubernetes"

// storageVersionsView is the StorageVersion objects in More: which version
// each API server stores each resource in (alpha).
func storageVersionsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "internal.apiserver.k8s.io", Version: "v1alpha1", Resource: "storageversions", Kind: "StorageVersion", Namespaced: false},
		Title: "storageversion",
	}
}
