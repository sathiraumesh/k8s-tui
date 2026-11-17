package commands

import k8s "k8s-tui/internal/kubernetes"

// clusterTrustBundlesView is the ClusterTrustBundle objects in More: CA
// certificates published for pods to trust, mounted with a
// clusterTrustBundle volume.
func clusterTrustBundlesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "certificates.k8s.io", Version: "v1", Resource: "clustertrustbundles", Kind: "ClusterTrustBundle", Namespaced: false},
		Title:       "trustbundle",
		DeleteAbout: "remove it; pods mounting it no longer get its certificates",
	}
}
