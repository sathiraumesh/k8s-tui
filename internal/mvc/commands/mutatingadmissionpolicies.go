package commands

import k8s "k8s-tui/internal/kubernetes"

// mutatingAdmissionPoliciesView is the MutatingAdmissionPolicy objects in
// More: changes to requests written in CEL, run in the API server itself; a
// binding puts one to work.
func mutatingAdmissionPoliciesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingadmissionpolicies", Kind: "MutatingAdmissionPolicy", Namespaced: false},
		Title:       "mutatingpolicy",
		DeleteAbout: "remove it; its bindings then change nothing",
	}
}
