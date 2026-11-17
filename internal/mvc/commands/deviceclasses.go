package commands

import k8s "k8s-tui/internal/kubernetes"

// deviceClassesView is the DeviceClass objects in More: kinds of devices
// (GPUs, NICs) a claim can ask for, by a selector over what drivers publish
// (Dynamic Resource Allocation).
func deviceClassesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "resource.k8s.io", Version: "v1", Resource: "deviceclasses", Kind: "DeviceClass", Namespaced: false},
		Title:       "deviceclass",
		DeleteAbout: "remove it; new claims naming it can't be allocated",
	}
}
