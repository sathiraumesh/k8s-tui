package commands

import k8s "k8s-tui/internal/kubernetes"

// validatingAdmissionPoliciesView is the ValidatingAdmissionPolicy objects
// in More: checks of requests written in CEL, run in the API server itself;
// a binding puts one to work.
func validatingAdmissionPoliciesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicies", Kind: "ValidatingAdmissionPolicy", Namespaced: false},
		Title:       "validatingpolicy",
		DeleteAbout: "remove it; its bindings then check nothing",
	}
}
