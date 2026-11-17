package commands

import k8s "k8s-tui/internal/kubernetes"

// csiDriversView is the CSIDriver objects in More: the storage drivers (CSI)
// of the cluster and how Kubernetes should treat them.
func csiDriversView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "csidrivers", Kind: "CSIDriver", Namespaced: false},
		Title:       "csidriver",
		DeleteAbout: "remove it; Kubernetes treats its driver with the defaults (attach needed, no pod info)",
	}
}
