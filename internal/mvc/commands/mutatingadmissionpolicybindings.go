package commands

import k8s "k8s-tui/internal/kubernetes"

// mutatingAdmissionPolicyBindingsView is the MutatingAdmissionPolicyBinding
// objects in More: where a mutating admission policy applies.
func mutatingAdmissionPolicyBindingsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingadmissionpolicybindings", Kind: "MutatingAdmissionPolicyBinding", Namespaced: false},
		Title:       "mutatingpolicybinding",
		DeleteAbout: "remove it; its policy no longer applies there",
	}
}
