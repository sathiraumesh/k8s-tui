package commands

import k8s "k8s-tui/internal/kubernetes"

// csiNodesView is the CSINode objects in More: which CSI drivers run on each
// node, and how many volumes it can take; the kubelet writes them.
func csiNodesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "csinodes", Kind: "CSINode", Namespaced: false},
		Title:       "csinode",
		DeleteAbout: "remove it; the kubelet writes it again",
	}
}
