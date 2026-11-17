package commands

import k8s "k8s-tui/internal/kubernetes"

// leaseCandidatesView is the LeaseCandidate objects in More: the components
// standing to lead, for coordinated leader election (beta).
func leaseCandidatesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "coordination.k8s.io", Version: "v1beta1", Resource: "leasecandidates", Kind: "LeaseCandidate", Namespaced: true},
		Title:       "leasecandidate",
		DeleteAbout: "remove it; its component stands again",
	}
}
