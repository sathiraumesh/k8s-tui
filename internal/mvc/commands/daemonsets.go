package commands

import k8s "k8s-tui/internal/kubernetes"

// daemonSetsView is DaemonSets in More: one pod on every node, e.g. a log
// collector or a network plugin. Its own: Restart and Set image (no
// Scale: the nodes decide how many pods it runs).
func daemonSetsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "apps", Version: "v1", Resource: "daemonsets", Kind: "DaemonSet", Namespaced: true},
		Title: "ds",
		Actions: []Action{
			restartAction("daemonsets.apps", "replace its pod on each node, a node at a time",
				(*k8s.Client).RestartDaemonSet),
			setImageAction("daemonset", "its pods are then replaced node by node",
				(*k8s.Client).DaemonSetContainers, (*k8s.Client).SetDaemonSetImage),
		},
		DeleteAbout: "remove it and its pod on every node",
	}
}
