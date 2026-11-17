package commands

import k8s "k8s-tui/internal/kubernetes"

// deviceTaintRulesView is the DeviceTaintRule objects in More: taints on
// devices, so pods that don't tolerate them stay off (or are evicted).
func deviceTaintRulesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "resource.k8s.io", Version: "v1", Resource: "devicetaintrules", Kind: "DeviceTaintRule", Namespaced: false},
		Title:       "devicetaint",
		DeleteAbout: "remove it; its devices are no longer tainted by it",
	}
}
