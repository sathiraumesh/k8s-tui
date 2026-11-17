package commands

import k8s "k8s-tui/internal/kubernetes"

// componentStatusesView is ComponentStatuses in More: whether the
// scheduler, the controller manager and etcd answer (deprecated since
// v1.19; read-only). Its own: STATUS colored.
func componentStatusesView() View {
	return View{
		Type:   k8s.ResourceType{Version: "v1", Resource: "componentstatuses", Kind: "ComponentStatus"},
		Title:  "cs",
		Status: &ComponentStatus,
	}
}

// ComponentStatus is the STATUS column of the component statuses: NAME
// STATUS. Healthy is green, Unhealthy red.
var ComponentStatus = StatusColumn{Column: 1, Level: func(status string) Level {
	switch status {
	case "Healthy":
		return LevelOK
	case "Unhealthy":
		return LevelBad
	}
	return LevelNone
}}
