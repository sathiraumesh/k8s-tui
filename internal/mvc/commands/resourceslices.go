package commands

import k8s "k8s-tui/internal/kubernetes"

// resourceSlicesView is the ResourceSlice objects in More: the devices each
// driver publishes, per node or pool; drivers write them.
func resourceSlicesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "resource.k8s.io", Version: "v1", Resource: "resourceslices", Kind: "ResourceSlice", Namespaced: false},
		Title:       "resourceslice",
		DeleteAbout: "remove it; a running driver publishes it again",
	}
}
