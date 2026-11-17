package commands

import (
	"strings"

	k8s "k8s-tui/internal/kubernetes"
)

// apiServicesView is APIServices in More: each API group and version the
// API server serves, its own (Local) or by an extension server, such as
// metrics-server for v1beta1.metrics.k8s.io. Its own: AVAILABLE colored.
func apiServicesView() View {
	return View{
		Type:        k8s.ResourceType{Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices", Kind: "APIService"},
		Title:       "apiservice",
		Status:      &APIServiceAvailable,
		DeleteAbout: "remove it; its API is no longer served (a Local one, built in, comes back)",
	}
}

// APIServiceAvailable is the AVAILABLE column of the API services: NAME
// SERVICE AVAILABLE. True is green; False (MissingEndpoints and the like:
// its server doesn't answer) is red, and makes kubectl's discovery slow.
var APIServiceAvailable = StatusColumn{Column: 2, Level: func(available string) Level {
	switch {
	case available == "True":
		return LevelOK
	case strings.HasPrefix(available, "False"):
		return LevelBad
	}
	return LevelNone
}}
