package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// resourceQuotasView is ResourceQuotas in More: caps on what a namespace
// may use in all. Its own: making one.
func resourceQuotasView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "resourcequotas", Kind: "ResourceQuota", Namespaced: true},
		Title: "quota",
		Own: []Action{{
			Key: "c", Name: "quota", About: "name resource=amount ...: team pods=10 requests.cpu=4 limits.memory=8Gi",
			Changes: true,
			Input:   "name resource=amount ...",
			InputArgs: func(input string) string {
				name, limits := nameAndRest(input)
				if len(limits) == 0 {
					return input
				}
				return name + " --hard=" + strings.Join(limits, ",")
			},
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl create quota " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				name, limits := nameAndRest(input)
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateResourceQuota(ctx, namespace, name, limits, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create a quota",
	}
}
