package commands

import k8s "k8s-tui/internal/kubernetes"

// flowSchemasView is the FlowSchema objects in More: which requests to the
// API server go to which priority level (API Priority and Fairness).
func flowSchemasView() View {
	return View{
		Type:        k8s.ResourceType{Group: "flowcontrol.apiserver.k8s.io", Version: "v1", Resource: "flowschemas", Kind: "FlowSchema", Namespaced: false},
		Title:       "flowschema",
		DeleteAbout: "remove it; its requests go to the next matching schema (a built-in one comes back)",
	}
}
