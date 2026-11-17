package commands

import k8s "k8s-tui/internal/kubernetes"

// storageVersionMigrationsView is the StorageVersionMigration objects in
// More: migrations of a resource's stored objects to the version now stored.
func storageVersionMigrationsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "storagemigration.k8s.io", Version: "v1", Resource: "storageversionmigrations", Kind: "StorageVersionMigration", Namespaced: false},
		Title:       "storagemigration",
		DeleteAbout: "remove it; a migration that ran stays done",
	}
}
