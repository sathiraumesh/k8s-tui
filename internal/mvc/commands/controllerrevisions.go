package commands

import k8s "k8s-tui/internal/kubernetes"

// controllerRevisionsView is the ControllerRevision objects in More: the
// past templates of statefulsets and daemonsets, for kubectl rollout history
// and undo.
func controllerRevisionsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "apps", Version: "v1", Resource: "controllerrevisions", Kind: "ControllerRevision", Namespaced: true},
		Title:       "controllerrevision",
		DeleteAbout: "remove it; that revision can no longer be rolled back to",
	}
}
