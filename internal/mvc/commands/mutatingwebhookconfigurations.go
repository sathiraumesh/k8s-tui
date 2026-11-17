package commands

import k8s "k8s-tui/internal/kubernetes"

// mutatingWebhookConfigurationsView is the MutatingWebhookConfiguration
// objects in More: the webhooks the API server lets change a request before
// it is saved, e.g. to add a sidecar.
func mutatingWebhookConfigurationsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations", Kind: "MutatingWebhookConfiguration", Namespaced: false},
		Title:       "mutatingwebhook",
		DeleteAbout: "remove it; requests are no longer changed by its webhooks",
	}
}
