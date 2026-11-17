package commands

import k8s "k8s-tui/internal/kubernetes"

// replicaSetsView is ReplicaSets in More: a number of identical pods, most
// made by a deployment, one per version of its template. Its own: Scale.
func replicaSetsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet", Namespaced: true},
		Title: "rs",
		Actions: []Action{
			scaleAction("replicasets.apps", "a deployment that owns it sets it back: scale the deployment",
				(*k8s.Client).ScaleReplicaSet),
		},
		DeleteAbout: "remove it and its pods; a deployment that owns it makes it again",
	}
}
