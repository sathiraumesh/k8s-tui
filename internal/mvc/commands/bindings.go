package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// bindingsView is Bindings in More: how the scheduler puts a pod on a
// node; nothing to list. Its own: Bind, putting a Pending pod on a node by
// hand.
func bindingsView() View {
	return View{
		Type:       k8s.ResourceType{Version: "v1", Resource: "bindings", Kind: "Binding", Namespaced: true},
		Title:      "binding",
		CreateOnly: "Nothing to list: a binding puts a pod on a node, as the scheduler does. Press + to bind a Pending pod.",
		Own: []Action{{
			Key: "b", Name: "Bind", About: "pod node: put a Pending pod of this namespace on a node, skipping the scheduler's checks",
			Changes: true,
			Input:   "pod node",
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl create -n %s -f - # Binding of the pod to the node: " + InputHere,
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "")
				return Change("Bind", namespace+"/"+f[0], flags, func(ctx context.Context) (string, error) {
					return "", c.BindPod(ctx, namespace, f[0], f[1], Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ bind",
		OwnTitle:  "Bind a pod to a node",
	}
}
