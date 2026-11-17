package commands

import (
	"context"
	"strings"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// networkPoliciesView is NetworkPolicies in More: which traffic may reach
// which pods (if the network plugin enforces them). Its own: two ready
// policies for every pod of the namespace.
func networkPoliciesView() View {
	return View{
		Type:  k8s.ResourceType{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies", Kind: "NetworkPolicy", Namespaced: true},
		Title: "netpol",
		Own: []Action{
			policyAction("d", "deny all ingress", "refuse all traffic to every pod of the namespace; then allow what is needed",
				"deny-all-ingress", k8s.DenyAllIngress),
			policyAction("s", "allow same namespace", "let in only traffic from pods of the same namespace",
				"allow-same-namespace", k8s.AllowSameNamespace),
		},
		OwnButton:   "+ create",
		OwnTitle:    "Create a network policy",
		DeleteAbout: "remove it; the traffic it allowed or refused is no longer checked by it",
	}
}

// policyAction makes the ready-made policy kind, its name typed.
func policyAction(key, name, about, defaultName string, kind k8s.PolicyKind) Action {
	return Action{
		Key: key, Name: name, About: about + "; needs a network plugin that enforces policies",
		Changes:      true,
		Input:        "name",
		InputDefault: defaultName,
		Flags:        []Flag{{Name: FlagDryRun}},
		Command:      "kubectl apply -n %s -f - # NetworkPolicy " + InputHere + ": " + name,
		Run: func(c *k8s.Client, namespace, input string, flags []Flag) tea.Cmd {
			input = strings.TrimSpace(input)
			return Change("Create", namespace+"/"+input, flags, func(ctx context.Context) (string, error) {
				return "", c.CreateNetworkPolicy(ctx, namespace, input, kind, Ticked(flags, FlagDryRun))
			})
		},
	}
}
