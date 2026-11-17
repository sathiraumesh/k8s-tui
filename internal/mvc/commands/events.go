package commands

import k8s "k8s-tui/internal/kubernetes"

// The events: what happened to things, e.g. a pod pulled its image or
// failed to start, kept about an hour. The same events are served by two
// APIs, v1 (kubectl get events) and events.k8s.io/v1; each has a view.
// Their own: TYPE colored, Warning yellow.

// eventsView is the v1 events.
func eventsView() View {
	return View{
		Type:   k8s.ResourceType{Version: "v1", Resource: "events", Kind: "Event", Namespaced: true},
		Title:  "events",
		Status: &EventType,
	}
}

// eventsK8sIOView is the events.k8s.io/v1 events.
func eventsK8sIOView() View {
	return View{
		Type:   k8s.ResourceType{Group: "events.k8s.io", Version: "v1", Resource: "events", Kind: "Event", Namespaced: true},
		Title:  "events.k8s.io",
		Status: &EventType,
	}
}

// EventType is the TYPE column of the events: NAME LAST-SEEN TYPE (the
// name is put first, see View.Load). Warning is yellow; Normal isn't
// colored.
var EventType = StatusColumn{Column: 2, Level: func(t string) Level {
	if t == "Warning" {
		return LevelWarn
	}
	return LevelNone
}}
