package commands

import k8s "k8s-tui/internal/kubernetes"

// replicationControllersView is ReplicationControllers in More: the old way
// to keep pods running, before deployments. Its own: Scale.
func replicationControllersView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "replicationcontrollers", Kind: "ReplicationController", Namespaced: true},
		Title: "rc",
		Actions: []Action{
			scaleAction("replicationcontrollers", "use a deployment for new apps", (*k8s.Client).ScaleReplicationController),
		},
		DeleteAbout: "remove it and its pods",
	}
}
