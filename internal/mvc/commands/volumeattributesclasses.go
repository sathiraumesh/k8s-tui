package commands

import k8s "k8s-tui/internal/kubernetes"

// volumeAttributesClassesView is the VolumeAttributesClass objects in More:
// settings a volume can change while in use, e.g. its IOPS, if its CSI
// driver allows.
func volumeAttributesClassesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattributesclasses", Kind: "VolumeAttributesClass", Namespaced: false},
		Title:       "vac",
		DeleteAbout: "remove it; check no claim names it first",
	}
}
