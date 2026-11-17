package commands

import (
	"context"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// customResourceDefinitionsView is CustomResourceDefinitions in More: the
// kinds added to the API, e.g. by cert-manager or an operator. Its own:
// Objects, a view of the objects of the kind one adds.
func customResourceDefinitionsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions", Kind: "CustomResourceDefinition"},
		Title: "crd",
		Actions: []Action{{
			Key: "o", Name: "Objects", About: "the objects of the kind it adds, in a view like any kind's",
			Command: "kubectl get %s",
			Run: func(c *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				return func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
					defer cancel()
					t, err := c.CustomResourceType(ctx, target)
					if err != nil {
						return ActionDoneMsg{Action: "Objects", Target: target, Err: err}
					}
					return OpenViewMsg{View: View{Type: t, Title: t.Resource}}
				}
			},
		}},
		DeleteAbout: "remove it and EVERY object of the kind it adds",
	}
}

// OpenViewMsg asks the model to open View in More, e.g. a CRD's objects.
type OpenViewMsg struct {
	View View
}
