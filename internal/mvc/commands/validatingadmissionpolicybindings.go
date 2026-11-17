package commands

import k8s "k8s-tui/internal/kubernetes"

// validatingAdmissionPolicyBindingsView is the
// ValidatingAdmissionPolicyBinding objects in More: where a validating
// admission policy applies, and what to do when it fails (Deny, Warn,
// Audit).
func validatingAdmissionPolicyBindingsView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicybindings", Kind: "ValidatingAdmissionPolicyBinding", Namespaced: false},
		Title:       "validatingpolicybinding",
		DeleteAbout: "remove it; its policy no longer applies there",
	}
}
