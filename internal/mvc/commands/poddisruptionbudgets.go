package commands

import (
	"context"
	"fmt"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// podDisruptionBudgetsView is PodDisruptionBudgets in More: how many of an
// app's pods must stay up while nodes are drained. Its own: making one.
func podDisruptionBudgetsView() View {
	return View{
		Type:  k8s.ResourceType{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets", Kind: "PodDisruptionBudget", Namespaced: true},
		Title: "pdb",
		Own: []Action{{
			Key: "c", Name: "budget", About: "name selector min-available: web app=web 1 (or 50%)",
			Changes:   true,
			Input:     "name selector min-available",
			InputArgs: pdbArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create poddisruptionbudget " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				f := append(strings.Fields(input), "", "", "")
				return Change("Create", namespace+"/"+f[0], flags, func(ctx context.Context) (string, error) {
					return "", c.CreatePodDisruptionBudget(ctx, namespace, f[0], f[1], f[2], Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create a disruption budget",
	}
}

// pdbArgs writes "web app=web 1" as kubectl takes it:
// "web --selector=app=web --min-available=1".
func pdbArgs(input string) string {
	f := strings.Fields(input)
	if len(f) != 3 {
		return input
	}
	return fmt.Sprintf("%s --selector=%s --min-available=%s", f[0], f[1], f[2])
}
