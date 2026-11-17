package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "k8s-tui/internal/kubernetes"

	tea "github.com/charmbracelet/bubbletea"
)

// serviceAccountsView is ServiceAccounts in More: the identities pods run
// as. Its own: Token (a short-lived token to act as it), and making one.
func serviceAccountsView() View {
	return View{
		Type:  k8s.ResourceType{Version: "v1", Resource: "serviceaccounts", Kind: "ServiceAccount", Namespaced: true},
		Title: "sa",
		Actions: []Action{{
			Key: "t", Name: "Token", About: "a token to act as it, lasting the time typed, e.g. 1h; copy it from the page",
			Input:        "duration",
			InputDefault: "1h",
			InputArgs:    func(d string) string { return "--duration=" + strings.TrimSpace(d) },
			Command:      "kubectl create token %s",
			Run: func(c *k8s.Client, target, duration string, _ []Flag) tea.Cmd {
				namespace, name := SplitTarget(target)
				return Read("Token", target, func(ctx context.Context) (string, error) {
					token, expires, err := c.ServiceAccountToken(ctx, namespace, name, duration)
					return tokenText(token, expires), err
				})
			},
		}},
		Own: []Action{{
			Key: "s", Name: "service account", About: "a new identity for pods; give it permissions with a role binding",
			Changes: true,
			Input:   "name",
			Flags:   []Flag{{Name: FlagDryRun}},
			Command: "kubectl create serviceaccount " + InputHere + " -n %s",
			Run: func(c *k8s.Client, namespace, name string, flags []Flag) tea.Cmd {
				name = strings.TrimSpace(name)
				return Change("Create", namespace+"/"+name, flags, func(ctx context.Context) (string, error) {
					return "", c.CreateServiceAccount(ctx, namespace, name, Ticked(flags, FlagDryRun))
				})
			},
		}},
		OwnButton: "+ create",
		OwnTitle:  "Create a service account",
	}
}

// tokenText is the page of a token: when it expires, then the token alone
// on its line, to copy (right click › Copy row).
func tokenText(token string, expires time.Time) string {
	if token == "" {
		return ""
	}
	return fmt.Sprintf("Expires %s. Use it with kubectl --token=..., or as Authorization: Bearer ...\n\n%s",
		expires.Local().Format("2006-01-02 15:04:05"), token)
}
