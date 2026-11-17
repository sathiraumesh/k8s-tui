package commands

import k8s "k8s-tui/internal/kubernetes"

// leasesView is the Lease objects in More: who holds a lock: each node's
// heartbeat (kube-node-lease), and the leader of a controller.
func leasesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "coordination.k8s.io", Version: "v1", Resource: "leases", Kind: "Lease", Namespaced: true},
		Title:       "lease",
		DeleteAbout: "remove it; its holder takes it again, a node at its next heartbeat",
	}
}
