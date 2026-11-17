package commands

import k8s "k8s-tui/internal/kubernetes"

// volumeAttachmentsView is the VolumeAttachment objects in More: which
// volume is attached to which node, by which CSI driver.
func volumeAttachmentsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments", Kind: "VolumeAttachment", Namespaced: false},
		Title:       "volumeattachment",
		DeleteAbout: "remove it: asks the driver to detach the volume (careful while a pod uses it)",
	}
}
