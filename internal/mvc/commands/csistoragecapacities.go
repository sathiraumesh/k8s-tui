package commands

import k8s "k8s-tui/internal/kubernetes"

// csiStorageCapacitiesView is the CSIStorageCapacity objects in More: how
// much storage a CSI driver has free where, for the scheduler to place pods
// with new volumes.
func csiStorageCapacitiesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storage.k8s.io", Version: "v1", Resource: "csistoragecapacities", Kind: "CSIStorageCapacity", Namespaced: true},
		Title:       "csistoragecapacity",
		DeleteAbout: "remove it; its driver writes it again",
	}
}
