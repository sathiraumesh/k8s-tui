package commands

import k8s "k8s-tui/internal/kubernetes"

// resourceClaimTemplatesView is the ResourceClaimTemplate objects in More:
// templates from which each pod gets its own resource claim.
func resourceClaimTemplatesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "resource.k8s.io", Version: "v1", Resource: "resourceclaimtemplates", Kind: "ResourceClaimTemplate", Namespaced: true},
		Title:       "resourceclaimtemplate",
		DeleteAbout: "remove it; claims made from it stay with their pods",
	}
}
