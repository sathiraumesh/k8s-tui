package commands

import k8s "k8s-tui/internal/kubernetes"

// validatingWebhookConfigurationsView is the ValidatingWebhookConfiguration
// objects in More: the webhooks the API server asks whether to allow a
// request, e.g. a policy engine.
func validatingWebhookConfigurationsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations", Kind: "ValidatingWebhookConfiguration", Namespaced: false},
		Title:       "validatingwebhook",
		DeleteAbout: "remove it; requests are no longer checked by its webhooks",
	}
}
