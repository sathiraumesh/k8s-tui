package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// ingressesView is Ingresses in More: HTTP routes from outside the cluster
// to services, by host and path. Its own: making one from rules.
func ingressesView() View {
	return View{
		Type:  k8s.ResourceType{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress", Namespaced: true},
		Title: "ing",
		Own: []Action{{
			Key: "c", Name: "ingress", About: "name and rules HOST/PATH=SERVICE:PORT: shop shop.example.com/*=web:80; needs an ingress controller",
			Changes:   true,
			Input:     "name rule ...",
			InputArgs: ruleArgs,
			Flags:     []Flag{{Name: FlagDryRun}},
			Command:   "kubectl create ingress " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
				name, rules := nameAndRest(input)
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateIngress(ctx, namespace, name, rules, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create an ingress",
	}
}

// ruleArgs writes "shop a.com/=web:80" as kubectl takes it:
// "shop --rule=a.com/=web:80".
func ruleArgs(input string) string {
	name, rules := nameAndRest(input)
	parts := []string{name}
	for _, r := range rules {
		parts = append(parts, "--rule="+r)
	}
	return strings.Join(parts, " ")
}
