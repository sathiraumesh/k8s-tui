package commands

import k8s "k8s-tui/internal/kubernetes"

// podCertificateRequestsView is the PodCertificateRequest objects in More:
// certificates the kubelet asks for on behalf of a pod (podCertificate
// volumes).
func podCertificateRequestsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "certificates.k8s.io", Version: "v1", Resource: "podcertificaterequests", Kind: "PodCertificateRequest", Namespaced: true},
		Title:       "podcertrequest",
		DeleteAbout: "remove it; the kubelet asks again when it refreshes",
	}
}
