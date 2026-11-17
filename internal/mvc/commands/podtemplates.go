package commands

import k8s "k8s-tui/internal/kubernetes"

// podTemplatesView is the PodTemplate objects in More: pod templates kept on
// their own, for tools to make pods from; the built-in controllers keep
// theirs inline.
func podTemplatesView() View {
	return View{
		Type:  k8s.ResourceType{Group: "", Version: "v1", Resource: "podtemplates", Kind: "PodTemplate", Namespaced: true},
		Title: "podtemplate",
	}
}
