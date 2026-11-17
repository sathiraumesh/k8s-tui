package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// namespacesView is Namespaces in More: the groups the cluster's things
// are in. Its own: the STATUS colors, Use (make it the namespace picked),
// and making one.
func namespacesView() View {
	return View{
		Type:        k8s.ResourceType{Version: "v1", Resource: "namespaces", Kind: "Namespace"},
		Title:       "ns",
		Status:      &NamespaceStatus,
		DeleteAbout: "remove it and EVERYTHING in it: pods, services, secrets, …",
		Actions: []Action{{
			Key: "u", Name: "Use", About: "make it the namespace of the tabs, as the ns button picks one",
			Command: "kubectl config set-context --current --namespace=%s",
			Run: func(_ *k8s.Client, target, _ string, _ []Flag) tea.Cmd {
				return func() tea.Msg { return UseNamespaceMsg{Namespace: target} }
			},
		}},
		Own: []Action{{
			Key: "n", Name: "namespace", About: "a new, empty namespace",
			Changes: true,
			Input:   "name",
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl create namespace " + InputHere,
			Run: func(c *k8s.Client, _, name string, flags []Flag) tea.Cmd {
				name = strings.TrimSpace(name)
				return Change("Create", name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateNamespace(ctx, name, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create a namespace",
	}
}

// UseNamespaceMsg asks the model to make Namespace the one picked.
type UseNamespaceMsg struct {
	Namespace string
}

// NamespaceStatus is the STATUS column of the namespaces: NAME STATUS.
// Active is fine; Terminating (being deleted, with all in it) is on its way.
var NamespaceStatus = StatusColumn{Column: 1, Level: func(status string) Level {
	switch status {
	case "Active":
		return LevelOK
	case "Terminating":
		return LevelWarn
	}
	return LevelNone
}}
